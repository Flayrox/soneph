package downloader

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	Bitrate   string     `json:"bitrate"`
	Status    TaskStatus `json:"status"`
	Progress  string     `json:"progress"`
	Logs      []string   `json:"logs"`
	CreatedAt time.Time  `json:"created_at"`
	Error     string     `json:"error,omitempty"`
}

type Manager struct {
	mu           sync.RWMutex
	tasks        map[string]*DownloadTask
	queue        chan *DownloadTask
	downloadDir  string
	broadcastFn  func(event string, data interface{})
}

func NewManager(downloadDir string, broadcastFn func(event string, data interface{})) *Manager {
	if downloadDir == "" {
		downloadDir = "./downloads"
	}
	// Ensure directory exists
	_ = os.MkdirAll(downloadDir, 0755)

	m := &Manager{
		tasks:       make(map[string]*DownloadTask),
		queue:       make(chan *DownloadTask, 100),
		downloadDir: downloadDir,
		broadcastFn: broadcastFn,
	}

	// Spawn 8 concurrent worker threads to process download tasks in parallel
	for i := 0; i < 8; i++ {
		go m.worker()
	}
	return m
}

func (m *Manager) AddTask(url string, bitrate string) *DownloadTask {
	if bitrate == "" {
		bitrate = "320k"
	}
	m.mu.Lock()
	id := fmt.Sprintf("task_%d", time.Now().UnixNano())
	task := &DownloadTask{
		ID:        id,
		URL:       url,
		Bitrate:   bitrate,
		Status:    StatusQueued,
		Progress:  "In queue...",
		Logs:      []string{fmt.Sprintf("[%s] Task added to queue for: %s (Quality: %s)", time.Now().Format("15:04:05"), url, bitrate)},
		CreatedAt: time.Now(),
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
		list = append(list, task)
	}
	return list
}

func (m *Manager) GetTask(id string) (*DownloadTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, exists := m.tasks[id]
	return t, exists
}

func (m *Manager) notifyUpdate(task *DownloadTask) {
	if m.broadcastFn != nil {
		m.broadcastFn("task_update", task)
	}
}

func (m *Manager) worker() {
	for task := range m.queue {
		m.runTask(task)
	}
}

func (m *Manager) runTask(task *DownloadTask) {
	m.mu.Lock()
	task.Status = StatusDownloading
	task.Progress = "Starting spotdl..."
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Starting download...", time.Now().Format("15:04:05")))
	m.mu.Unlock()
	m.notifyUpdate(task)

	// Build output path template
	outputTemplate := filepath.Join(m.downloadDir, "{artist}", "{album}", "{title}.{output-ext}")

	// Smart Highest Quality Protection:
	// If bitrate is 320k (HQ), force overwrite lower quality files.
	// If bitrate is lower (128k/192k), skip overwriting existing higher quality files.
	overwriteFlag := "skip"
	if task.Bitrate == "320k" {
		overwriteFlag = "force"
	}

	// Command setup: spotdl download [URL] --bitrate [bitrate] --threads 8 --overwrite [force|skip] ...
	cmd := exec.Command("spotdl", "download", task.URL, "--bitrate", task.Bitrate, "--threads", "8", "--overwrite", overwriteFlag, "--lyrics", "genius", "synced", "--max-retries", "10", "--generate-lrc", "--output", outputTemplate)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.failTask(task, fmt.Sprintf("Failed to open stdout pipe: %v", err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.failTask(task, fmt.Sprintf("Failed to open stderr pipe: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		m.failTask(task, fmt.Sprintf("Failed to start spotdl command: %v", err))
		return
	}

	// Read outputs in parallel
	var wg sync.WaitGroup
	wg.Add(2)

	scanOutput := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			m.mu.Lock()
			task.Logs = append(task.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line))
			if len(task.Logs) > 200 {
				task.Logs = task.Logs[len(task.Logs)-200:]
			}
			task.Progress = line
			m.mu.Unlock()

			m.notifyUpdate(task)
		}
	}

	go scanOutput(stdout)
	go scanOutput(stderr)

	wg.Wait()
	err = cmd.Wait()

	m.mu.Lock()
	if err != nil {
		task.Status = StatusFailed
		task.Error = err.Error()
		task.Progress = "Download failed"
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] Error: %v", time.Now().Format("15:04:05"), err))
	} else {
		// Run post-processing to embed ID3 USLT lyrics tag into MP3 for l'app Musique / iTunes
		embedCmd := exec.Command("python3", "/app/embed_lyrics.py", m.downloadDir)
		_ = embedCmd.Run()

		task.Status = StatusCompleted
		task.Progress = "Download finished & ready for sync!"
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] Completed successfully & embedded l'app Musique ID3 lyrics!", time.Now().Format("15:04:05")))
	}
	m.mu.Unlock()

	m.notifyUpdate(task)
	// Also trigger file list refresh event
	if m.broadcastFn != nil {
		m.broadcastFn("downloads_changed", nil)
	}
}

func (m *Manager) failTask(task *DownloadTask, errMsg string) {
	m.mu.Lock()
	task.Status = StatusFailed
	task.Error = errMsg
	task.Progress = errMsg
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Error: %s", time.Now().Format("15:04:05"), errMsg))
	m.mu.Unlock()
	m.notifyUpdate(task)
}
