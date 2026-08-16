package downloader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"soneph-backend/pkg/config"
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
}

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
	if enginePath, err := exec.LookPath(engineBin); err == nil {
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

func NewManager(downloadDir string, broadcastFn func(event string, data interface{})) *Manager {
	if downloadDir == "" {
		downloadDir = "./downloads"
	}
	_ = os.MkdirAll(downloadDir, 0755)

	m := &Manager{
		tasks:       make(map[string]*DownloadTask),
		queue:       make(chan *DownloadTask, 100),
		downloadDir: downloadDir,
		broadcastFn: broadcastFn,
		persistPath: queuePath(),
	}
	m.recoverQueue()

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
	m.queue <- task
	m.persist()
	return task
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

// scanIdentity renvoie la carte {identité (URL Spotify) → chemins} de la
// bibliothèque, via le script Python qui lit les tags WOAS (mutagen).
// Plusieurs fichiers peuvent partager la même identité (même morceau sur
// plusieurs albums) : on garde la liste complète.
func (m *Manager) scanIdentity() map[string][]string {
	out := map[string][]string{}
	cmd := exec.Command(GetPythonExec(), GetScriptPath("scan_identity.py"), m.downloadDir)
	data, err := cmd.Output()
	if err != nil {
		return out
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out
	}
	return out
}

// ScanIdentityMap expose la carte des identités pour le handler API (ex.
// retrouver les autres copies d'un morceau quand l'utilisateur en supprime
// une, pour migrer les stats).
func (m *Manager) ScanIdentityMap() map[string][]string {
	return m.scanIdentity()
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
	for task := range m.queue {
		m.runTask(task)
	}
}

func (m *Manager) runTask(task *DownloadTask) {
	m.mu.Lock()
	task.Status = StatusDownloading
	task.Progress = "Instant scanning disk for existing songs..."
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Fast scanning disk for duplicates...", time.Now().Format("15:04:05")))
	m.mu.Unlock()
	m.notifyUpdate(task)

	// État de la bibliothèque avant le téléchargement : comparé à l'état
	// d'après, il permet de détecter les fichiers déplacés (single → album)
	// et de migrer les stats vers leurs nouveaux chemins.
	identityBefore := m.scanIdentity()

	pythonExec := GetPythonExec()
	fastFilterScript := GetScriptPath("fast_filter.py")

	// Instant Pre-Filter Execution in Python
	ffCmd := exec.Command(pythonExec, fastFilterScript, m.downloadDir, task.URL)
	ffOutput, ffErr := ffCmd.Output()
	var ffResult struct {
		FastFilterApplied bool     `json:"fast_filter_applied"`
		TotalTracks       int      `json:"total_tracks"`
		AlreadyDownloaded int      `json:"already_downloaded_count"`
		MissingCount      int      `json:"missing_count"`
		SkippedTracks     []string `json:"skipped_tracks"`
		MissingQueries    []string `json:"missing_queries"`
	}
	if ffErr == nil {
		if jsonErr := json.Unmarshal(ffOutput, &ffResult); jsonErr == nil && ffResult.FastFilterApplied {
			m.mu.Lock()
			task.TotalTracks = ffResult.TotalTracks
			task.CompletedCount = ffResult.AlreadyDownloaded
			for _, s := range ffResult.SkippedTracks {
				task.RecentTracks = append([]string{s + " (déjà sur disque)"}, task.RecentTracks...)
			}
			task.Logs = append(task.Logs, fmt.Sprintf("[%s] Fast filter complete: %d songs already on disk, %d missing.", time.Now().Format("15:04:05"), ffResult.AlreadyDownloaded, ffResult.MissingCount))
			m.mu.Unlock()
			m.notifyUpdate(task)

			if ffResult.MissingCount == 0 {
				// Tout est déjà sur disque : on lance quand même le moteur en
				// mode métadonnées. Il détecte les singles téléchargés avant
				// leur album et met à jour leurs tags (album, pochette…) sans
				// re-télécharger l'audio.
				m.mu.Lock()
				task.Progress = "All tracks present — metadata upgrade pass (singles → albums)..."
				task.Logs = append(task.Logs, fmt.Sprintf("[%s] All %d tracks present on disk — mise à jour des métadonnées (singles → albums).", time.Now().Format("15:04:05"), ffResult.TotalTracks))
				m.mu.Unlock()
				m.notifyUpdate(task)
			}
		}
	}

	outputTemplate := filepath.Join(m.downloadDir, "{artist}", "{album}", "{title}.mp3")

	// Pré-création des dossiers d'album pour le passage métadonnées : si des
	// morceaux existent déjà sur disque (souvent des singles), on prépare les
	// dossiers d'album cibles pour que le déplacement du moteur ne rate pas
	// (spotdl ne crée pas le parent avant son Path.replace). Calculé avec les
	// mêmes fonctions que spotdl, donc les chemins correspondent exactement.
	// On saute uniquement quand le fast filter a prouvé qu'aucun morceau de
	// l'URL n'est sur disque ; sinon (URL non résolue, track seul…) on lance
	// quand même, le coût est juste une récupération de métadonnées.
	skipPrecreate := ffErr == nil && ffResult.FastFilterApplied && ffResult.AlreadyDownloaded == 0
	if !skipPrecreate {
		precreateScript := GetScriptPath("precreate_dirs.py")
		pcCmd := exec.Command(pythonExec, precreateScript, m.downloadDir, outputTemplate, task.URL)
		if pcOut, pcErr := pcCmd.CombinedOutput(); pcErr == nil {
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
	cmdArgs := []string{
		"download", task.URL,
		"--bitrate", task.Bitrate,
		"--threads", strconv.Itoa(threads),
		"--overwrite", overwriteFlag,
		"--scan-for-songs",
		"--max-retries", "1",
		"--output", outputTemplate,
	}

	cmd := exec.Command(engineBin, cmdArgs...)

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
			m.failTask(task, fmt.Sprintf("download engine process exited with error: %v", err))
			return
		}
		m.mu.Lock()
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] Warning: download engine exited with code %v (some tracks may have failed to download — this is normal)", time.Now().Format("15:04:05"), err))
		m.mu.Unlock()
	}

	// Détection des fichiers déplacés par le moteur (single → album) et
	// migration des stats (historique, likes, playlists) vers les nouveaux
	// chemins, pour ne rien perdre quand un morceau change de dossier.
	identityAfter := m.scanIdentity()
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
