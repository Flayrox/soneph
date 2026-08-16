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

type Manager struct {
	mu          sync.RWMutex
	tasks       map[string]*DownloadTask
	queue       chan *DownloadTask
	downloadDir string
	broadcastFn func(event string, data interface{})
	persistPath string
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

	pythonExec := GetPythonExec()
	fastFilterScript := GetScriptPath("fast_filter.py")

	// Instant Pre-Filter Execution in Python
	ffCmd := exec.Command(pythonExec, fastFilterScript, m.downloadDir, task.URL)
	ffOutput, ffErr := ffCmd.Output()
	if ffErr == nil {
		var ffResult struct {
			FastFilterApplied bool     `json:"fast_filter_applied"`
			TotalTracks       int      `json:"total_tracks"`
			AlreadyDownloaded int      `json:"already_downloaded_count"`
			MissingCount      int      `json:"missing_count"`
			SkippedTracks     []string `json:"skipped_tracks"`
			MissingQueries    []string `json:"missing_queries"`
		}
		if jsonErr := json.Unmarshal(ffOutput, &ffResult); jsonErr == nil && ffResult.FastFilterApplied {
			m.mu.Lock()
			task.TotalTracks = ffResult.TotalTracks
			task.CompletedCount = ffResult.AlreadyDownloaded
			for _, s := range ffResult.SkippedTracks {
				task.RecentTracks = append([]string{s + " (instant skip)"}, task.RecentTracks...)
			}
			task.Logs = append(task.Logs, fmt.Sprintf("[%s] Fast filter complete: %d songs already on disk, %d missing.", time.Now().Format("15:04:05"), ffResult.AlreadyDownloaded, ffResult.MissingCount))
			m.mu.Unlock()
			m.notifyUpdate(task)

			if ffResult.MissingCount == 0 {
				m.mu.Lock()
				task.Status = StatusCompleted
				task.Progress = "All tracks verified present on disk (0s instant skip)"
				task.Logs = append(task.Logs, fmt.Sprintf("[%s] All %d tracks present on disk!", time.Now().Format("15:04:05"), ffResult.TotalTracks))
				m.mu.Unlock()
				m.notifyUpdate(task)
				m.persist()
				return
			}
		}
	}

	outputTemplate := filepath.Join(m.downloadDir, "{artist}", "{album}", "{title}.mp3")

	overwriteFlag := "skip"
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

	m.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = "Download and metadata sync complete"
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] All tracks downloaded! Fetching lyrics in background...", time.Now().Format("15:04:05")))
	m.mu.Unlock()

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
			if v, ok := evt["missing_lrc"].(float64); ok {
				m.appendLog(task, fmt.Sprintf("[%s] Lyrics: %d chanson(s) sans paroles — récupération en arrière-plan...", time.Now().Format("15:04:05"), int(v)))
				m.notifyUpdate(task)
			}
		case "success":
			if name, ok := evt["filename"].(string); ok {
				m.appendLog(task, fmt.Sprintf("[%s] ✅ Lyrics: %s", time.Now().Format("15:04:05"), name))
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
				m.appendLog(task, fmt.Sprintf("[%s] Lyrics: %d OK, %d introuvables.", time.Now().Format("15:04:05"), int(v), int(failed)))
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
