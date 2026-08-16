package handler

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"soneph-backend/pkg/config"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/syncmgr"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type API struct {
	downloader  *downloader.Manager
	scanner     *storage.Scanner
	importer    *syncmgr.Importer
	lyricsJobMu sync.Mutex
	lyricsJob   *lyricsRetryJob
}

type lyricsRetryJob struct {
	Status    string    `json:"status"` // "running" | "done" | "idle"
	StartedAt time.Time `json:"started_at"`
	Total     int       `json:"total"`
	Done      int       `json:"done"`
	Success   int       `json:"success"`
	Failed    int       `json:"failed"`
	Current   string    `json:"current"`
	Logs      []string  `json:"logs"`
}

type DownloadRequest struct {
	URL     string `json:"url" binding:"required"`
	Bitrate string `json:"bitrate"`
	Order   string `json:"order"`
}

func NewAPI(dl *downloader.Manager, sc *storage.Scanner, imp *syncmgr.Importer) *API {
	return &API{
		downloader: dl,
		scanner:    sc,
		importer:   imp,
		lyricsJob:  &lyricsRetryJob{Status: "idle"},
	}
}

func (a *API) CreateDownload(c *gin.Context) {
	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'url' is required."})
		return
	}

	task := a.downloader.AddTask(req.URL, req.Bitrate, req.Order)
	c.JSON(http.StatusAccepted, gin.H{
		"message": "Download task queued",
		"task":    task,
	})
}

func (a *API) GetTasks(c *gin.Context) {
	tasks := a.downloader.GetTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (a *API) GetDownloads(c *gin.Context) {
	files, err := a.scanner.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (a *API) DeleteDownload(c *gin.Context) {
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}

	err := a.scanner.DeleteFile(relPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (a *API) StreamFile(c *gin.Context) {
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}

	fullPath := a.scanner.DownloadDir + "/" + relPath
	c.File(fullPath)
}

func (a *API) GetCover(c *gin.Context) {
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}

	fullPath := filepath.Join(a.scanner.DownloadDir, filepath.Clean(relPath))
	if _, err := os.Stat(fullPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audio file not found"})
		return
	}

	h := md5.Sum([]byte(relPath))
	hashStr := hex.EncodeToString(h[:])
	coverDir := filepath.Join(a.scanner.DownloadDir, ".covers")
	coverPath := filepath.Join(coverDir, hashStr+".jpg")

	if _, err := os.Stat(coverPath); err != nil {
		pythonExec := downloader.GetPythonExec()
		extractScript := downloader.GetScriptPath("extract_cover.py")

		cmd := exec.Command(pythonExec, extractScript, fullPath, coverPath)
		_ = cmd.Run()
	}

	if _, err := os.Stat(coverPath); err == nil {
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(coverPath)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "No embedded artwork found"})
}

func (a *API) GetLyrics(c *gin.Context) {
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}

	fullPath := a.scanner.DownloadDir + "/" + relPath
	// Strip audio extension and append .lrc
	ext := filepath.Ext(fullPath)
	lrcPath := strings.TrimSuffix(fullPath, ext) + ".lrc"

	content, err := os.ReadFile(lrcPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lyrics file not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"lyrics": string(content),
	})
}

// ScanMissingLyrics returns all MP3 files that don't have a .lrc sidecar file.
func (a *API) ScanMissingLyrics(c *gin.Context) {
	pythonExec := downloader.GetPythonExec()
	lyricsScript := downloader.GetScriptPath("lyrics_retry.py")

	cmd := exec.Command(pythonExec, lyricsScript, a.scanner.DownloadDir, "--scan-only")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var scanComplete map[string]interface{}
	var missingList map[string]interface{}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt map[string]interface{}
		if json.Unmarshal([]byte(line), &evt) != nil {
			continue
		}
		if evt["type"] == "scan_complete" {
			scanComplete = evt
		}
		if evt["type"] == "missing_list" {
			missingList = evt
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"scan":    scanComplete,
		"missing": missingList,
	})
}

// RetryLyrics launches a background job to fetch synced lyrics for all MP3s missing .lrc files.
func (a *API) RetryLyrics(c *gin.Context) {
	a.lyricsJobMu.Lock()
	if a.lyricsJob.Status == "running" {
		a.lyricsJobMu.Unlock()
		c.JSON(http.StatusConflict, gin.H{
			"error": "A lyrics retry job is already running",
			"job":   a.lyricsJob,
		})
		return
	}
	a.lyricsJob = &lyricsRetryJob{
		Status:    "running",
		StartedAt: time.Now(),
		Logs:      []string{},
	}
	a.lyricsJobMu.Unlock()

	go func() {
		pythonExec := downloader.GetPythonExec()
		lyricsScript := downloader.GetScriptPath("lyrics_retry.py")

		cmd := exec.Command(pythonExec, lyricsScript, a.scanner.DownloadDir)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			a.lyricsJobMu.Lock()
			a.lyricsJob.Status = "done"
			a.lyricsJob.Logs = append(a.lyricsJob.Logs, "Error: "+err.Error())
			a.lyricsJobMu.Unlock()
			return
		}
		cmd.Stderr = cmd.Stdout
		_ = cmd.Start()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			var evt map[string]interface{}
			if json.Unmarshal([]byte(line), &evt) != nil {
				continue
			}
			a.lyricsJobMu.Lock()
			evtType, _ := evt["type"].(string)
			switch evtType {
			case "scan_complete":
				if v, ok := evt["missing_lrc"].(float64); ok {
					a.lyricsJob.Total = int(v)
				}
			case "retrying":
				if name, ok := evt["filename"].(string); ok {
					a.lyricsJob.Current = name
				}
				if v, ok := evt["index"].(float64); ok {
					a.lyricsJob.Done = int(v)
				}
			case "success":
				a.lyricsJob.Success++
				if name, ok := evt["filename"].(string); ok {
					a.lyricsJob.Logs = append([]string{"✅ " + name}, a.lyricsJob.Logs...)
				}
			case "failed":
				a.lyricsJob.Failed++
				if name, ok := evt["filename"].(string); ok {
					a.lyricsJob.Logs = append([]string{"❌ " + name}, a.lyricsJob.Logs...)
				}
			case "done":
				if v, ok := evt["success"].(float64); ok {
					a.lyricsJob.Success = int(v)
				}
				if v, ok := evt["failed"].(float64); ok {
					a.lyricsJob.Failed = int(v)
				}
			}
			if len(a.lyricsJob.Logs) > 100 {
				a.lyricsJob.Logs = a.lyricsJob.Logs[:100]
			}
			a.lyricsJobMu.Unlock()
		}

		_ = cmd.Wait()
		a.lyricsJobMu.Lock()
		a.lyricsJob.Status = "done"
		a.lyricsJobMu.Unlock()
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Lyrics retry job started",
		"job":     a.lyricsJob,
	})
}

// GetLyricsJobStatus returns the current state of the lyrics retry job.
func (a *API) GetLyricsJobStatus(c *gin.Context) {
	a.lyricsJobMu.Lock()
	defer a.lyricsJobMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"job": a.lyricsJob})
}

// GetSettings returns the current app settings (download workers/threads).
func (a *API) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, config.Load())
}

// SaveSettings persists new app settings.
func (a *API) SaveSettings(c *gin.Context) {
	var s config.Settings
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workers et threads (nombres) attendus"})
		return
	}
	if err := config.Save(s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Réglages enregistrés", "settings": config.Load()})
}

// GetSyncStatus returns the l'app Musique auto-import status.
func (a *API) GetSyncStatus(c *gin.Context) {
	c.JSON(http.StatusOK, a.importer.Status())
}

// StartSync launches the l'app Musique auto-importer watcher.
func (a *API) StartSync(c *gin.Context) {
	if err := a.importer.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a.importer.Status())
}

// StopSync stops the l'app Musique auto-importer watcher.
func (a *API) StopSync(c *gin.Context) {
	if err := a.importer.Stop(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a.importer.Status())
}

