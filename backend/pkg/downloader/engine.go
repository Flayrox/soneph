package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"soneph-backend/pkg/config"
	"soneph-backend/pkg/fastfilter"
	"soneph-backend/pkg/jobs"
	"soneph-backend/pkg/store"
	"soneph-backend/pkg/tags"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusQueued      TaskStatus = "queued"
	StatusDownloading TaskStatus = "downloading"
	StatusCompleted   TaskStatus = "completed"
	StatusFailed      TaskStatus = "failed"
)

// engineBin is the download engine CLI (a pip-installed binary). The default
// name is the package's real binary; SONEPH_ENGINE lets you point to a
// differently-named executable without touching the code.
var engineBin = func() string {
	if v := os.Getenv("SONEPH_ENGINE"); v != "" {
		return v
	}
	return "spotdl"
}()

// engineCandidateDirsFor lists the directories where the download engine
// (and ffmpeg / python helpers) commonly live, but which are missing from
// PATH when the app is launched from Finder/Dock (macOS) or a restricted
// environment: pipx, pip --user (macOS Python), Homebrew, pyenv, conda.
// home is injectable for tests.
func engineCandidateDirsFor(home string) []string {
	dirs := []string{
		filepath.Join(home, ".local", "bin"),     // pipx
		filepath.Join(home, ".pyenv", "shims"),   // pyenv
		filepath.Join(home, "miniconda3", "bin"), // conda
		filepath.Join(home, "anaconda3", "bin"),  // conda
		"/opt/homebrew/bin",                      // Homebrew (Apple Silicon)
		"/usr/local/bin",                         // Homebrew (Intel) / python.org
	}
	// pip --user sur macOS : ~/Library/Python/<3.x>/bin
	if home != "" {
		if entries, err := os.ReadDir(filepath.Join(home, "Library", "Python")); err == nil {
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(e.Name(), "3.") {
					dirs = append(dirs, filepath.Join(home, "Library", "Python", e.Name(), "bin"))
				}
			}
		}
	}
	return dirs
}

func engineCandidateDirs() []string {
	home, _ := os.UserHomeDir()
	return engineCandidateDirsFor(home)
}

// engineCandidatePaths returns the absolute candidate paths for a given
// engine binary name, in search order.
func engineCandidatePaths(name string) []string {
	var out []string
	for _, dir := range engineCandidateDirs() {
		out = append(out, filepath.Join(dir, name))
	}
	return out
}

var (
	engineOnce sync.Once
	enginePath string
)

// resolveEnginePath finds the download engine executable. It honors, in
// order: SONEPH_ENGINE (a name or a full path), the process PATH, then the
// common install directories a GUI-launched process doesn't see. Returns ""
// when the engine is not installed anywhere.
func resolveEnginePath() string {
	name := engineBin
	if name == "" {
		return ""
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, p := range engineCandidatePaths(name) {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// EngineBin returns the configured engine binary name ("spotdl" unless
// SONEPH_ENGINE overrides it).
func EngineBin() string {
	return engineBin
}

// EnginePath returns the absolute path of the download engine binary, or ""
// if it is not installed. The result is cached: env vars are read at
// startup, which is when they're set by the launcher anyway.
func EnginePath() string {
	engineOnce.Do(func() {
		enginePath = resolveEnginePath()
	})
	return enginePath
}

// engineMissingMessage explains how to install the download engine. It
// surfaces in the task list and toasts, so it's written for the end user
// (the app UI is French).
func engineMissingMessage() string {
	name := engineBin
	if name == "" {
		name = "spotdl"
	}
	return fmt.Sprintf(
		"Moteur de téléchargement « %s » introuvable. Installe-le puis relance l'app :\n\n    pipx install %s\n    (ou : pip install %s)\n\nCherché dans : %s",
		name, name, name,
		strings.Join(append([]string{"le PATH"}, engineCandidatePaths(name)...), ", "),
	)
}

type DownloadTask struct {
	ID             string     `json:"id"`
	URL            string     `json:"url"`
	Bitrate        string     `json:"bitrate"`
	Order          string     `json:"order"`
	Status         TaskStatus `json:"status"`
	Progress       string     `json:"progress"`
	CurrentTrack   string     `json:"current_track"`
	TotalTracks    int        `json:"total_tracks"`
	CompletedCount int        `json:"completed_count"`
	RecentTracks   []string   `json:"recent_tracks"`
	Logs           []string   `json:"logs"`
	CreatedAt      time.Time  `json:"created_at"`
	Error          string     `json:"error,omitempty"`
}

// FileMove décrit un fichier déplacé par le moteur (ex. single → album) :
// l'identité (URL Spotify) est la même, seul le rel_path a changé. Le backend
// s'en sert pour migrer les stats (historique, likes, playlists) sans les
// perdre quand un morceau bouge.
type FileMove struct {
	OldRel string `json:"old_rel"`
	NewRel string `json:"new_rel"`
}

type Manager struct {
	mu           sync.RWMutex
	tasks        map[string]*DownloadTask
	queue        chan *DownloadTask
	downloadDir  string
	broadcastFn  func(event string, data interface{})
	persistPath  string
	onFilesMoved func(moves []FileMove)
	onTaskDone   func(task *DownloadTask)
	// jobs (M4) : quand non nil, la file est la table jobs — dequeue
	// atomique, backoff exponentiel et circuit breaker par source.
	jobs *jobs.Queue

	// filterResults (M5) : résultat du job fast_filter par tâche, appliqué
	// par ApplyFastFilter puis consommé par runTask (pré-création des
	// dossiers d'album) et nettoyé. En mode hérité (sans file), le filtre
	// s'exécute directement dans runTask.
	filterResults map[string]fastfilter.Result

	// Cache de la carte d'identité (URL Spotify → chemins) : la lire relit
	// les tags ID3 de TOUTE la bibliothèque (~1 s pour 136 fichiers, bien
	// plus avec des milliers). Sans cache, chaque tâche la calculait deux
	// fois (avant + après), ce qui ralentissait tout sur une grosse
	// bibliothèque. Le « avant » peut être légèrement périmé (TTL) — il
	// représente l'état d'avant la tâche ; le « après » est toujours frais.
	identityMu   sync.Mutex
	identityMap  map[string][]string
	identityTime time.Time
}

// identityCacheTTL borne la fraîcheur du cache « avant ». 30 s : assez court
// pour ne pas rater un déplacement, assez long pour amortir le scan complet.
const identityCacheTTL = 30 * time.Second

// queuePath returns where in-flight tasks are persisted so a backend restart
// can re-queue them (same config dir as the app settings).
func queuePath() string {
	if p := os.Getenv("SONEPH_QUEUE_FILE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "soneph", "queue.json")
}

// GetPythonExec returns the Python interpreter used to run our helper
// scripts. It prefers the interpreter that the download engine uses, because
// that environment ships with the deps the scripts need (mutagen,
// syncedlyrics). This keeps local dev (homebrew python) and Docker working
// identically.
func GetPythonExec() string {
	if enginePath := EnginePath(); enginePath != "" {
		if data, err := os.ReadFile(enginePath); err == nil {
			shebang := strings.TrimPrefix(strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0]), "#!")
			if fields := strings.Fields(shebang); len(fields) > 0 {
				prog := fields[0]
				if prog == "/usr/bin/env" && len(fields) > 1 {
					prog = fields[1]
				}
				if strings.HasPrefix(prog, "/") {
					if _, err := os.Stat(prog); err == nil {
						return prog
					}
				} else if _, err := exec.LookPath(prog); err == nil {
					return prog
				}
			}
		}
	}
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3"
	}
	if _, err := exec.LookPath("python"); err == nil {
		return "python"
	}
	return "python3"
}

func GetScriptPath(scriptName string) string {
	// 1. Docker / container path
	dockerPath := filepath.Join("/app", scriptName)
	if _, err := os.Stat(dockerPath); err == nil {
		return dockerPath
	}

	// 2. Relative to current working directory
	if _, err := os.Stat(scriptName); err == nil {
		return scriptName
	}

	// 3. Parent directory / root workspace
	parentPath := filepath.Join("..", scriptName)
	if _, err := os.Stat(parentPath); err == nil {
		return parentPath
	}

	// 4. Executable binary directory
	if execPath, err := os.Executable(); err == nil {
		binDir := filepath.Dir(execPath)
		p := filepath.Join(binDir, scriptName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return scriptName
}

// Regexes used to parse the download engine's console output. The engine
// changes its output format between releases — these are tested
// (engine_test.go) so a format change is caught by CI instead of silently
// breaking progress tracking.
var (
	reTotal       = regexp.MustCompile(`Found\s+(\d+)\s+songs`)
	reDownloaded  = regexp.MustCompile(`Downloaded\s+"([^"]+)"`)
	reSkipping    = regexp.MustCompile(`Skipping\s+([^(]+)`)
	reDownloading = regexp.MustCompile(`Downloading\s+"([^"]+)"`)
	// Emis par le moteur quand un morceau déjà sur disque (ex. single) est
	// déplacé vers son album et que ses tags sont réécrits sans re-télécharger.
	reMetadataUpgraded = regexp.MustCompile(`Updated\s+metadata\s+for\s+(.+?),\s+moved\s+to\s+new\s+location`)
)

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func NewManager(downloadDir string, broadcastFn func(event string, data interface{}), jobQueues ...*jobs.Queue) *Manager {
	if downloadDir == "" {
		downloadDir = "./downloads"
	}
	_ = os.MkdirAll(downloadDir, 0755)

	m := &Manager{
		tasks:         make(map[string]*DownloadTask),
		queue:         make(chan *DownloadTask, 100),
		downloadDir:   downloadDir,
		broadcastFn:   broadcastFn,
		persistPath:   queuePath(),
		filterResults: make(map[string]fastfilter.Result),
	}
	if len(jobQueues) > 0 {
		m.jobs = jobQueues[0]
		// La file est la table jobs : les tâches 'running' d'un processus
		// mort (kill -9) sont relancées au démarrage.
		if err := m.jobs.RequeueOrphaned(); err != nil {
			slog.Error("relance des jobs orphelins impossible", "err", err)
		}
	} else {
		m.recoverQueue()
	}

	// Parallel engine processes (one per queued URL). Keep this modest:
	// each process already downloads several tracks concurrently, and going
	// too aggressive triggers platform rate limiting which makes
	// everything slower. Réglable via l'UI (config) ou SONEPH_WORKERS.
	cfg := config.Load()
	workers := envInt("SONEPH_WORKERS", cfg.Workers)
	for i := 0; i < workers; i++ {
		go m.worker()
	}
	return m
}

func (m *Manager) AddTask(url string, bitrate string, order string) *DownloadTask {
	if bitrate == "" {
		bitrate = "320k"
	}
	if order == "" {
		order = "normal"
	}
	m.mu.Lock()
	id := fmt.Sprintf("task_%d", time.Now().UnixNano())
	task := &DownloadTask{
		ID:           id,
		URL:          url,
		Bitrate:      bitrate,
		Order:        order,
		Status:       StatusQueued,
		Progress:     "In queue...",
		RecentTracks: []string{},
		Logs:         []string{fmt.Sprintf("[%s] Task queued for: %s (Quality: %s, Order: %s)", time.Now().Format("15:04:05"), url, bitrate, order)},
		CreatedAt:    time.Now(),
	}
	m.tasks[id] = task
	m.mu.Unlock()

	m.notifyUpdate(task)
	if m.jobs != nil {
		// M5 : le fast filter (Go) tourne d'abord en tant que job asynchrone
		// dans la file M4. À son terme, le worker fast_filter applique le
		// résultat à la tâche et ré-enfile le téléchargement (même task_id).
		payload, _ := json.Marshal(fastfilter.Payload{TaskID: id, URL: url, Bitrate: bitrate, Order: order})
		if err := m.jobs.Enqueue(store.Job{ID: "ff_" + id, Type: "fast_filter", Payload: string(payload)}); err != nil {
			slog.Error("job fast_filter non enfilé", "task", id, "err", err)
		}
	} else {
		m.queue <- task
		m.persist()
	}
	return task
}

// ApplyFastFilter (M5) applique le résultat du job fast_filter à la tâche :
// totaux, morceaux déjà sur disque, logs — et le mémorise pour runTask
// (pré-création des dossiers d'album). Appelé par le worker fast_filter
// AVANT de ré-enfiler le téléchargement.
func (m *Manager) ApplyFastFilter(p fastfilter.Payload, res fastfilter.Result) {
	m.mu.Lock()
	task, ok := m.tasks[p.TaskID]
	if !ok {
		// Backend redémarré entre le filtre et le téléchargement : on
		// reconstruit une tâche minimale depuis le payload (taskFromJob fait
		// pareil côté worker download).
		task = &DownloadTask{
			ID:           p.TaskID,
			URL:          p.URL,
			Bitrate:      p.Bitrate,
			Order:        p.Order,
			Status:       StatusQueued,
			Progress:     "Instant scanning disk for existing songs...",
			Logs:         []string{},
			RecentTracks: []string{},
			CreatedAt:    time.Now(),
		}
		m.tasks[p.TaskID] = task
	}
	m.filterResults[p.TaskID] = res
	m.mu.Unlock()

	m.applyFilterResult(task, res)
	m.notifyUpdate(task)
}

// EnqueueDownload (M5) ré-enfile le téléchargement après un fast filter
// réussi : la tâche existante (même task_id) est reprise par le worker
// download. Appelé par le worker fast_filter dans main.go.
func (m *Manager) EnqueueDownload(p fastfilter.Payload) {
	if m.jobs == nil {
		return
	}
	payload, _ := json.Marshal(jobs.PayloadDownload{URL: p.URL, Bitrate: p.Bitrate, Order: p.Order})
	if err := m.jobs.Enqueue(store.Job{ID: p.TaskID, Type: "download", Payload: string(payload)}); err != nil {
		slog.Error("job download non ré-enfilé après filtre", "task", p.TaskID, "err", err)
	}
}

// takeFilterResult lit (et efface) le résultat du fast filter mémorisé pour
// une tâche. En mode file jobs, le filtre a déjà tourné : runTask le
// consomme pour l'étape de pré-création des dossiers. Sans résultat, c'est
// le mode hérité (le filtre s'exécute dans runTask).
func (m *Manager) takeFilterResult(taskID string) (fastfilter.Result, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res, ok := m.filterResults[taskID]
	if ok {
		delete(m.filterResults, taskID)
	}
	return res, ok
}

// applyFilterResult met à jour la tâche depuis le résultat du fast filter
// (mêmes effets que l'ancien pipeline Python) : totaux, liste « déjà sur
// disque », logs, et passage « all tracks present » quand tout est là.
func (m *Manager) applyFilterResult(task *DownloadTask, res fastfilter.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if res.Applied {
		task.TotalTracks = res.TotalTracks
		task.CompletedCount = res.AlreadyDownloaded
		for _, s := range res.SkippedTracks {
			task.RecentTracks = append([]string{s + " (déjà sur disque)"}, task.RecentTracks...)
		}
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] Fast filter complete: %d songs already on disk, %d missing.", time.Now().Format("15:04:05"), res.AlreadyDownloaded, res.MissingCount))
		if res.MissingCount == 0 {
			// Tout est déjà sur disque : on lance quand même le moteur en
			// mode métadonnées. Il détecte les singles téléchargés avant leur
			// album et met à jour leurs tags (album, pochette…) sans
			// re-télécharger l'audio.
			task.Progress = "All tracks present — metadata upgrade pass (singles → albums)..."
			task.Logs = append(task.Logs, fmt.Sprintf("[%s] All %d tracks present on disk — mise à jour des métadonnées (singles → albums).", time.Now().Format("15:04:05"), res.TotalTracks))
		}
	} else if res.Reason != "" {
		// Filtre désactivé (ex. playlist > 100 titres, API embed plafonnée) :
		// on l'explique dans les logs au lieu de montrer des chiffres faux.
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] ⚠️ %s", time.Now().Format("15:04:05"), res.Reason))
	}
}

// SetOnFilesMoved enregistre le callback appelé quand le moteur a déplacé
// des fichiers (single → album) : le backend migre alors les stats vers les
// nouveaux chemins.
func (m *Manager) SetOnFilesMoved(fn func(moves []FileMove)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onFilesMoved = fn
}

// SetOnTaskDone enregistre le callback appelé quand un téléchargement se
// termine (succès comme échec partiel). Le backend s'en sert pour finaliser
// une playlist créée en même temps que le téléchargement : les morceaux
// fraîchement arrivés sur disque y sont ajoutés.
func (m *Manager) SetOnTaskDone(fn func(task *DownloadTask)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTaskDone = fn
}

// Broadcast émet un événement WebSocket (même canal que les task_update),
// pour que le handler API puisse prévenir le frontend (ex. playlist
// complétée) sans tenir le hub lui-même.
func (m *Manager) Broadcast(event string, data interface{}) {
	m.mu.RLock()
	fn := m.broadcastFn
	m.mu.RUnlock()
	if fn != nil {
		fn(event, data)
	}
}

// runScanIdentity lance le scan complet (lecture des tags WOAS de tous les
// MP3) et renvoie la carte {identité → chemins}. Plusieurs fichiers peuvent
// partager la même identité (même morceau sur plusieurs albums) : on garde
// la liste complète. M6 : port Go de scan_identity.py (pkg/tags.IdentityMap)
// — plus de sous-processus Python.
func (m *Manager) runScanIdentity() map[string][]string {
	out, err := tags.IdentityMap(m.downloadDir)
	if err != nil {
		return map[string][]string{}
	}
	return out
}

// scanIdentity renvoie la carte d'identité, en réutilisant le cache s'il est
// encore frais (TTL 30 s). Utilisé pour l'état « avant » d'une tâche : un
// léger décalage est sans conséquence pour la détection des déplacements.
func (m *Manager) scanIdentity() map[string][]string {
	m.identityMu.Lock()
	if m.identityMap != nil && time.Since(m.identityTime) < identityCacheTTL {
		cached := m.identityMap
		m.identityMu.Unlock()
		return cached
	}
	m.identityMu.Unlock()

	fresh := m.runScanIdentity()

	m.identityMu.Lock()
	m.identityMap = fresh
	m.identityTime = time.Now()
	m.identityMu.Unlock()
	return fresh
}

// scanIdentityFresh force un scan complet, pour l'état « après » d'une tâche
// (les fichiers viennent de changer sur disque, le cache serait périmé).
func (m *Manager) scanIdentityFresh() map[string][]string {
	fresh := m.runScanIdentity()

	m.identityMu.Lock()
	m.identityMap = fresh
	m.identityTime = time.Now()
	m.identityMu.Unlock()
	return fresh
}

// ScanIdentityMap expose la carte des identités pour le handler API (ex.
// retrouver les autres copies d'un morceau quand l'utilisateur en supprime
// une, pour migrer les stats). Appelé rarement (suppression de fichier) : on
// force un scan frais pour ne pas rater une copie récemment téléchargée.
func (m *Manager) ScanIdentityMap() map[string][]string {
	return m.scanIdentityFresh()
}

// diffMoves compare la carte avant/après un téléchargement et renvoie les
// fichiers dont le rel_path a changé (déplacés, pas supprimés). On ne traite
// que le cas propre : une seule occurrence avant, une seule après, à des
// chemins différents. Les cas multi-copies sont laissés de côté (plus sûr
// que de migrer au mauvais endroit).
func diffMoves(before, after map[string][]string) []FileMove {
	var moves []FileMove
	for id, oldPaths := range before {
		newPaths, ok := after[id]
		if !ok || len(oldPaths) != 1 || len(newPaths) != 1 {
			continue
		}
		if oldPaths[0] != newPaths[0] {
			moves = append(moves, FileMove{OldRel: oldPaths[0], NewRel: newPaths[0]})
		}
	}
	sort.Slice(moves, func(i, j int) bool {
		return moves[i].OldRel < moves[j].OldRel
	})
	return moves
}

// persist writes the in-flight tasks (queued + downloading) to disk so a
// backend restart can re-queue them. Completed/failed tasks are dropped —
// their files are on disk anyway.
func (m *Manager) persist() {
	tasks := m.GetTasks()
	active := make([]*DownloadTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Status == StatusQueued || t.Status == StatusDownloading {
			active = append(active, t)
		}
	}
	data, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.persistPath), 0o755)
	_ = os.WriteFile(m.persistPath, data, 0o644)
}

// recoverQueue re-queues any tasks left over from a previous run (crashed or
// stopped backend).
func (m *Manager) recoverQueue() {
	data, err := os.ReadFile(m.persistPath)
	if err != nil {
		return
	}
	var tasks []*DownloadTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return
	}
	for _, t := range tasks {
		if t == nil || t.ID == "" {
			continue
		}
		if t.Status != StatusQueued && t.Status != StatusDownloading {
			continue
		}
		t.Status = StatusQueued
		t.Error = ""
		t.Progress = "Re-queued after server restart"
		t.Logs = append(t.Logs, fmt.Sprintf("[%s] Server restarted — task re-queued.", time.Now().Format("15:04:05")))
		m.tasks[t.ID] = t
		m.queue <- t
	}
}

func (m *Manager) GetTasks() []*DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*DownloadTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		taskCopy := *task
		taskCopy.RecentTracks = append([]string{}, task.RecentTracks...)
		taskCopy.Logs = append([]string{}, task.Logs...)
		list = append(list, &taskCopy)
	}
	// Map iteration order is random — return tasks in queue order
	// (oldest first) so the UI shows a stable, FIFO view.
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list
}

func (m *Manager) GetTask(id string) (*DownloadTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, exists := m.tasks[id]
	if !exists {
		return nil, false
	}
	taskCopy := *t
	taskCopy.RecentTracks = append([]string{}, t.RecentTracks...)
	taskCopy.Logs = append([]string{}, t.Logs...)
	return &taskCopy, true
}

func (m *Manager) notifyUpdate(task *DownloadTask) {
	if m.broadcastFn == nil {
		return
	}
	m.mu.RLock()
	taskCopy := *task
	taskCopy.RecentTracks = append([]string{}, task.RecentTracks...)
	taskCopy.Logs = append([]string{}, task.Logs...)
	m.mu.RUnlock()

	m.broadcastFn("task_update", &taskCopy)
}

func (m *Manager) worker() {
	if m.jobs != nil {
		m.workerJobs()
		return
	}
	for task := range m.queue {
		m.runTask(task)
	}
}

// workerJobs est le worker M4 : il réclame un job 'download' (dequeue
// atomique + circuit breaker par source), l'exécute, puis clôture ou
// reprogramme (backoff exponentiel) selon le résultat.
func (m *Manager) workerJobs() {
	for {
		job, err := m.jobs.Dequeue("download")
		if err != nil {
			slog.Error("dequeue échoué", "err", err)
			time.Sleep(jobs.PollInterval)
			continue
		}
		if job == nil {
			time.Sleep(jobs.PollInterval)
			continue
		}
		task := m.taskFromJob(job)
		m.runTask(task)

		source := jobsSource(task.URL)
		m.mu.RLock()
		errMsg := task.Error
		succeeded := task.Status == StatusCompleted
		m.mu.RUnlock()

		switch {
		case succeeded:
			if err := m.jobs.Complete(job.ID, source, ""); err != nil {
				slog.Error("job non clôturé", "id", job.ID, "err", err)
			}
		case job.Attempts >= job.MaxAttempts:
			if err := m.jobs.Complete(job.ID, source, errMsg); err != nil {
				slog.Error("job non clôturé", "id", job.ID, "err", err)
			}
		default:
			// Nouvelle tentative différée (backoff 2^attempts × base) ; la
			// tâche reste visible dans l'UI comme « queued ».
			if err := m.jobs.ScheduleRetry(job.ID, job.Attempts); err != nil {
				slog.Error("retry non programmé", "id", job.ID, "err", err)
			}
			m.mu.Lock()
			task.Status = StatusQueued
			task.Error = ""
			task.Progress = "Échec — nouvelle tentative programmée automatiquement"
			task.Logs = append(task.Logs, fmt.Sprintf("[%s] Échec — nouvelle tentative programmée (tentative %d/%d).", time.Now().Format("15:04:05"), job.Attempts+1, job.MaxAttempts))
			m.mu.Unlock()
			m.notifyUpdate(task)
		}
	}
}

// taskFromJob retrouve la tâche en mémoire (AddTask) ou la reconstruit depuis
// le payload du job après un redémarrage.
func (m *Manager) taskFromJob(job *store.Job) *DownloadTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[job.ID]; ok {
		return t
	}
	var p jobs.PayloadDownload
	_ = json.Unmarshal([]byte(job.Payload), &p)
	if p.URL == "" {
		p.URL = job.ID
	}
	t := &DownloadTask{
		ID:           job.ID,
		URL:          p.URL,
		Bitrate:      p.Bitrate,
		Order:        p.Order,
		Status:       StatusQueued,
		Progress:     "Relancé après redémarrage",
		Logs:         []string{fmt.Sprintf("[%s] Task recovered from job queue after restart.", time.Now().Format("15:04:05"))},
		RecentTracks: []string{},
		CreatedAt:    time.Now(),
	}
	m.tasks[job.ID] = t
	return t
}

// jobsSource extrait l'hôte (circuit breaker) de l'URL d'une tâche.
func jobsSource(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func (m *Manager) runTask(task *DownloadTask) {
	m.mu.Lock()
	task.Status = StatusDownloading
	task.Progress = "Instant scanning disk for existing songs..."
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Fast scanning disk for duplicates...", time.Now().Format("15:04:05")))
	m.mu.Unlock()
	m.notifyUpdate(task)

	// Le moteur n'est pas installé (ou hors PATH — cas classique d'une app
	// lancée depuis le Finder/Dock sur macOS). On échoue immédiatement avec
	// un message qui explique comment l'installer, au lieu du vague
	// « executable file not found in $PATH ».
	if EnginePath() == "" {
		m.failTask(task, engineMissingMessage())
		return
	}

	// État de la bibliothèque avant le téléchargement : comparé à l'état
	// d'après, il permet de détecter les fichiers déplacés (single → album)
	// et de migrer les stats vers leurs nouveaux chemins.
	identityBefore := m.scanIdentity()

	pythonExec := GetPythonExec()

	// M5 : le fast filter est porté en Go (pkg/fastfilter) et tourne en tant
	// que job asynchrone dans la file M4 — son résultat a été appliqué à la
	// tâche par ApplyFastFilter et mémorisé dans m.filterResults (pré-création
	// des dossiers d'album). En mode hérité (sans file jobs), on l'exécute ici
	// même, en process, sans Python.
	ffResult, haveResult := m.takeFilterResult(task.ID)
	if !haveResult {
		ffResult = fastfilter.Run(m.downloadDir, task.URL, nil)
		m.applyFilterResult(task, ffResult)
		m.notifyUpdate(task)
	}

	outputTemplate := filepath.Join(m.downloadDir, "{artist}", "{album}", "{title}.mp3")

	// Pré-création des dossiers d'album pour le passage métadonnées : si des
	// morceaux existent déjà sur disque (souvent des singles), on prépare les
	// dossiers d'album cibles pour que le déplacement du moteur ne rate pas
	// (spotdl ne crée pas le parent avant son Path.replace). Calculé avec les
	// mêmes fonctions que spotdl, donc les chemins correspondent exactement.
	//
	// ⚠️ Ce script résout l'URL via l'API Spotify (client anonyme), qui est
	// lente et rate-limitée : sur une grosse playlist, il bloquait le worker
	// pendant des dizaines de minutes AVANT même de lancer spotdl (et après
	// un redémarrage, ses processus orphelins s'accumulaient). On ne le lance
	// donc que quand le fast filter prouve que des morceaux de l'URL sont
	// déjà sur disque (cas single → album) ET que la liste est petite, et on
	// le borne à 30 s pour ne jamais bloquer la file.
	runPrecreate := ffResult.Applied &&
		ffResult.AlreadyDownloaded > 0 &&
		ffResult.TotalTracks > 0 && ffResult.TotalTracks <= 200
	if runPrecreate {
		precreateScript := GetScriptPath("precreate_dirs.py")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		pcCmd := exec.CommandContext(ctx, pythonExec, precreateScript, m.downloadDir, outputTemplate, task.URL)
		pcOut, pcErr := pcCmd.CombinedOutput()
		cancel()
		if pcErr == nil {
			var pcResult struct {
				PrecreatedDirs int `json:"precreated_dirs"`
			}
			if json.Unmarshal(pcOut, &pcResult) == nil && pcResult.PrecreatedDirs > 0 {
				m.appendLog(task, fmt.Sprintf("[%s] Dossiers d'album préparés pour la mise à jour des singles → albums.", time.Now().Format("15:04:05")))
				m.notifyUpdate(task)
			}
		}
	}

	// Mode par défaut : "metadata" au lieu de "skip". Quand un morceau
	// existe déjà sur disque (souvent un single téléchargé avant son album),
	// le moteur le déplace vers son album et réécrit ses tags (album,
	// numéro de piste, pochette…) sans re-télécharger l'audio. --scan-for-songs
	// indexe la bibliothèque par URL Spotify pour retrouver ces fichiers
	// même s'ils sont dans un autre dossier.
	overwriteFlag := "metadata"
	if task.Order == "force" {
		overwriteFlag = "force"
	}

	// Audio-only download: lyrics are fetched afterwards in a background job
	// (see fetchLyricsInBackground) so a slow lyrics provider never stalls
	// the download queue. --threads controls parallel audio downloads;
	// keep it modest to avoid platform rate limiting. Réglable via
	// l'UI (config) ou SONEPH_THREADS.
	threads := envInt("SONEPH_THREADS", config.Load().Threads)

	// Le client par défaut de yt-dlp est régulièrement bloqué par YouTube
	// (« BaseClientError: Could not get session » — bot detection / rate
	// limit IP). On passe par des clients alternatifs qui passent ce
	// blocage (web_embedded, android_vr, tv_simply) ; yt-dlp essaie chacun
	// dans l'ordre. Surchargeable via SONEPH_YTDLP_ARGS.
	ytdlpArgs := os.Getenv("SONEPH_YTDLP_ARGS")
	if ytdlpArgs == "" {
		ytdlpArgs = "--extractor-args youtube:player_client=web_embedded,android_vr,tv_simply"
	}
	cmdArgs := []string{
		"download", task.URL,
		"--bitrate", task.Bitrate,
		"--threads", strconv.Itoa(threads),
		"--overwrite", overwriteFlag,
		"--scan-for-songs",
		"--max-retries", "1",
		"--yt-dlp-args", ytdlpArgs,
		"--output", outputTemplate,
	}

	cmd := exec.Command(EnginePath(), cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.failTask(task, fmt.Sprintf("Failed to open stdout pipe: %v", err))
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		m.failTask(task, fmt.Sprintf("Failed to start the download engine: %v", err))
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		m.mu.Lock()

		if len(task.Logs) > 300 {
			task.Logs = task.Logs[len(task.Logs)-250:]
		}
		task.Logs = append(task.Logs, line)
		task.Progress = line

		if match := reTotal.FindStringSubmatch(line); len(match) > 1 {
			if count, err := strconv.Atoi(match[1]); err == nil {
				task.TotalTracks = count
			}
		}

		if match := reDownloading.FindStringSubmatch(line); len(match) > 1 {
			task.CurrentTrack = match[1]
		}

		if match := reDownloaded.FindStringSubmatch(line); len(match) > 1 {
			songName := match[1]
			task.CurrentTrack = songName
			task.CompletedCount++
			task.RecentTracks = append([]string{songName}, task.RecentTracks...)
			if len(task.RecentTracks) > 50 {
				task.RecentTracks = task.RecentTracks[:50]
			}
		}

		if match := reSkipping.FindStringSubmatch(line); len(match) > 1 {
			songName := match[1]
			task.CompletedCount++
			task.RecentTracks = append([]string{songName + " (already downloaded)"}, task.RecentTracks...)
			if len(task.RecentTracks) > 50 {
				task.RecentTracks = task.RecentTracks[:50]
			}
		}

		if match := reMetadataUpgraded.FindStringSubmatch(line); len(match) > 1 {
			songName := strings.TrimSpace(match[1])
			task.CompletedCount++
			task.RecentTracks = append([]string{songName + " (metadata → album)"}, task.RecentTracks...)
			if len(task.RecentTracks) > 50 {
				task.RecentTracks = task.RecentTracks[:50]
			}
		}

		m.mu.Unlock()
		m.notifyUpdate(task)
	}

	if err := cmd.Wait(); err != nil {
		m.mu.RLock()
		hasSomeProgress := task.CompletedCount > 0 || task.TotalTracks > 0
		m.mu.RUnlock()

		if !hasSomeProgress {
			// Message d'erreur avec la cause réelle (ex. « Could not get
			// session » de yt-dlp) au lieu du vague « exit status 1 ».
			msg := fmt.Sprintf("download engine process exited with error: %v", err)
			if hint := m.engineErrorHint(task); hint != "" {
				msg += " — " + hint
			}
			m.failTask(task, msg)
			return
		}
		m.mu.Lock()
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] Warning: download engine exited with code %v (some tracks may have failed to download — this is normal)", time.Now().Format("15:04:05"), err))
		m.mu.Unlock()
	}

	// Détection des fichiers déplacés par le moteur (single → album) et
	// migration des stats (historique, likes, playlists) vers les nouveaux
	// chemins, pour ne rien perdre quand un morceau change de dossier.
	// « Après » toujours frais (les fichiers viennent de bouger).
	identityAfter := m.scanIdentityFresh()
	if moves := diffMoves(identityBefore, identityAfter); len(moves) > 0 {
		m.mu.RLock()
		onMoved := m.onFilesMoved
		m.mu.RUnlock()
		if onMoved != nil {
			onMoved(moves)
		}
		for _, mv := range moves {
			m.appendLog(task, fmt.Sprintf("[%s] 📦 %s → %s (stats migrées)", time.Now().Format("15:04:05"), mv.OldRel, mv.NewRel))
		}
		m.notifyUpdate(task)
	}

	m.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = "Download and metadata sync complete"
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] All tracks downloaded! Fetching lyrics in background...", time.Now().Format("15:04:05")))
	m.mu.Unlock()

	// Finalisation : si ce téléchargement créait une playlist (lien playlist
	// collé dans l'app), on y ajoute maintenant les morceaux manquants qui
	// viennent d'arriver sur disque. Appelé AVANT la notification de fin pour
	// que le frontend voie la playlist déjà complétée.
	m.mu.RLock()
	onDone := m.onTaskDone
	m.mu.RUnlock()
	if onDone != nil {
		onDone(task)
	}

	m.notifyUpdate(task)
	m.persist()

	// Lyrics are fetched and embedded in the background so the queue never
	// stalls on a slow lyrics provider. Progress lands in the task logs and
	// a "downloads_changed" event lets the frontend refresh file metadata.
	go m.fetchLyricsInBackground(task)
}

// engineErrorHint renvoie la cause probable d'un échec du moteur : la
// dernière ligne de log non vide (souvent l'erreur réelle de spotdl/yt-dlp,
// ex. « BaseClientError: Could not get session ») qui remplace le vague
// « exit status 1 » affiché à l'utilisateur.
func (m *Manager) engineErrorHint(task *DownloadTask) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := len(task.Logs) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(task.Logs[i]); s != "" {
			if len(s) > 300 {
				s = s[:300] + "…"
			}
			return s
		}
	}
	return ""
}

// appendLog appends a line to a task's logs under lock, keeping the list
// bounded so a long job never grows it without limit.
func (m *Manager) appendLog(task *DownloadTask, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(task.Logs) > 300 {
		task.Logs = task.Logs[len(task.Logs)-250:]
	}
	task.Logs = append(task.Logs, line)
}

// fetchLyricsInBackground scans the download folder for MP3s missing a .lrc
// sidecar, fetches synced lyrics (parallel, 6s timeout per song), then embeds
// them into the ID3v2.3 tags. It never blocks the download queue.
func (m *Manager) fetchLyricsInBackground(task *DownloadTask) {
	pythonExec := GetPythonExec()

	// 1. Marqueur soneph dans les métadonnées (idempotent) : chaque fichier
	//    porte un tag TXXX:SONEPH + sa source, pour que l'app sache d'où
	//    vient chaque morceau même si le fichier bouge (single → album).
	tagScript := GetScriptPath("tag_soneph.py")
	tagCmd := exec.Command(pythonExec, tagScript, m.downloadDir, task.URL)
	if tagOut, tagErr := tagCmd.CombinedOutput(); tagErr == nil && len(strings.TrimSpace(string(tagOut))) > 0 {
		m.appendLog(task, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), strings.TrimSpace(string(tagOut))))
		m.notifyUpdate(task)
	}

	lyricsScript := GetScriptPath("lyrics_retry.py")

	cmd := exec.Command(pythonExec, lyricsScript, m.downloadDir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return
	}

	lyricsDone := false
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		var evt map[string]interface{}
		if json.Unmarshal([]byte(line), &evt) != nil {
			continue
		}

		evtType, _ := evt["type"].(string)
		switch evtType {
		case "scan_complete":
			missing := 0.0
			unsynced := 0.0
			if v, ok := evt["missing_lrc"].(float64); ok {
				missing = v
			}
			if v, ok := evt["unsynced_lrc"].(float64); ok {
				unsynced = v
			}
			m.appendLog(task, fmt.Sprintf("[%s] Lyrics: %d sans paroles, %d en texte brut — récupération en arrière-plan...", time.Now().Format("15:04:05"), int(missing), int(unsynced)))
			m.notifyUpdate(task)
		case "success":
			if name, ok := evt["filename"].(string); ok {
				m.appendLog(task, fmt.Sprintf("[%s] ✅ Lyrics: %s", time.Now().Format("15:04:05"), name))
				m.notifyUpdate(task)
			}
		case "kept":
			if name, ok := evt["filename"].(string); ok {
				m.appendLog(task, fmt.Sprintf("[%s] ℹ️ Lyrics (texte brut conservé): %s", time.Now().Format("15:04:05"), name))
				m.notifyUpdate(task)
			}
		case "failed":
			if name, ok := evt["filename"].(string); ok {
				m.appendLog(task, fmt.Sprintf("[%s] ⚠️ Lyrics introuvables: %s", time.Now().Format("15:04:05"), name))
				m.notifyUpdate(task)
			}
		case "done":
			m.mu.Lock()
			lyricsDone = true
			m.mu.Unlock()
			if v, ok := evt["success"].(float64); ok {
				failed := 0.0
				if f, ok := evt["failed"].(float64); ok {
					failed = f
				}
				kept := 0.0
				if k, ok := evt["kept"].(float64); ok {
					kept = k
				}
				m.appendLog(task, fmt.Sprintf("[%s] Lyrics: %d OK, %d introuvables, %d déjà en texte brut (pas de version synced).", time.Now().Format("15:04:05"), int(v), int(failed), int(kept)))
			}
			m.notifyUpdate(task)
		}
	}
	_ = cmd.Wait()

	// Embed all .lrc files into the MP3 ID3v2.3 tags (USLT + SYLT).
	embedScript := GetScriptPath("embed_lyrics.py")
	embedCmd := exec.Command(pythonExec, embedScript, m.downloadDir)
	_ = embedCmd.Run()

	m.mu.Lock()
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Lyrics embeddées dans les tags ID3.", time.Now().Format("15:04:05")))
	if !lyricsDone {
		task.Logs = append(task.Logs, "[INFO] Aucun fichier .lrc manquant ou job lyrics interrompu.")
	}
	m.mu.Unlock()
	m.notifyUpdate(task)

	// Let the frontend refresh the file list (has_lyrics / lyrics_type).
	if m.broadcastFn != nil {
		m.broadcastFn("downloads_changed", nil)
	}
}

func (m *Manager) failTask(task *DownloadTask, errorMsg string) {
	m.mu.Lock()
	task.Status = StatusFailed
	task.Error = errorMsg
	task.Progress = fmt.Sprintf("Error: %s", errorMsg)
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] ERROR: %s", time.Now().Format("15:04:05"), errorMsg))
	m.mu.Unlock()
	m.notifyUpdate(task)
	m.persist()
}
