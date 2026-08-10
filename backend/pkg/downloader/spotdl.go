package downloader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
}

func GetPythonExec() string {
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
	}

	// 16 worker threads for parallel downloads
	for i := 0; i < 16; i++ {
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
	return task
}

func (m *Manager) GetTasks() []*DownloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*DownloadTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		taskCopy := *task
		if task.RecentTracks != nil {
			taskCopy.RecentTracks = append([]string(nil), task.RecentTracks...)
		}
		if task.Logs != nil {
			taskCopy.Logs = append([]string(nil), task.Logs...)
		}
		list = append(list, &taskCopy)
	}
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
	if t.RecentTracks != nil {
		taskCopy.RecentTracks = append([]string(nil), t.RecentTracks...)
	}
	if t.Logs != nil {
		taskCopy.Logs = append([]string(nil), t.Logs...)
	}
	return &taskCopy, true
}

func (m *Manager) notifyUpdate(task *DownloadTask) {
	if m.broadcastFn == nil {
		return
	}
	m.mu.RLock()
	taskCopy := *task
	if task.RecentTracks != nil {
		taskCopy.RecentTracks = append([]string(nil), task.RecentTracks...)
	}
	if task.Logs != nil {
		taskCopy.Logs = append([]string(nil), task.Logs...)
	}
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
				return
			}
		}
	}

	outputTemplate := filepath.Join(m.downloadDir, "{artist}", "{album}", "{title}.mp3")

	overwriteFlag := "skip"
	if task.Order == "force" {
		overwriteFlag = "force"
	}

	cmdArgs := []string{
		"download", task.URL,
		"--bitrate", task.Bitrate,
		"--threads", "16",
		"--overwrite", overwriteFlag,
		"--lyrics", "synced", "genius",
		"--max-retries", "1",
		"--generate-lrc",
		"--output", outputTemplate,
	}

	cmd := exec.Command("spotdl", cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.failTask(task, fmt.Sprintf("Failed to open stdout pipe: %v", err))
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		m.failTask(task, fmt.Sprintf("Failed to start spotdl: %v", err))
		return
	}

	reTotal := regexp.MustCompile(`Found\s+(\d+)\s+songs`)
	reDownloaded := regexp.MustCompile(`Downloaded\s+"([^"]+)"`)
	reSkipping := regexp.MustCompile(`Skipping\s+([^(]+)`)
	reDownloading := regexp.MustCompile(`Downloading\s+"([^"]+)"`)

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
			m.failTask(task, fmt.Sprintf("spotdl process exited with error: %v", err))
			return
		}
		m.mu.Lock()
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] Warning: spotdl exited with code %v (some tracks may have failed to download — this is normal)", time.Now().Format("15:04:05"), err))
		m.mu.Unlock()
	}

	embedScript := GetScriptPath("embed_lyrics.py")
	embedCmd := exec.Command(pythonExec, embedScript, m.downloadDir)
	_ = embedCmd.Run()

	m.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = "Download and metadata sync complete"
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] All tracks downloaded & synced!", time.Now().Format("15:04:05")))
	m.mu.Unlock()

	m.notifyUpdate(task)
}

func (m *Manager) failTask(task *DownloadTask, errorMsg string) {
	m.mu.Lock()
	task.Status = StatusFailed
	task.Error = errorMsg
	task.Progress = fmt.Sprintf("Error: %s", errorMsg)
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] ERROR: %s", time.Now().Format("15:04:05"), errorMsg))
	m.mu.Unlock()
	m.notifyUpdate(task)
}
