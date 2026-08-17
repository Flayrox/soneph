package handler

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"soneph-backend/pkg/config"
	"soneph-backend/pkg/downloader"

	"github.com/gin-gonic/gin"
)

// lyricsRetryJob est l'état de la tâche de re-tentative de paroles synced
// (lancée via POST /api/lyrics/retry), poussé au front via GET /api/lyrics/retry.
type lyricsRetryJob struct {
	Status    string    `json:"status"` // "running" | "done" | "idle"
	StartedAt time.Time `json:"started_at"`
	Total     int       `json:"total"`
	Done      int       `json:"done"`
	Success   int       `json:"success"`
	Failed    int       `json:"failed"`
	Kept      int       `json:"kept"` // paroles texte brut déjà là, pas de version synced dispo
	Current   string    `json:"current"`
	Logs      []string  `json:"logs"`
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

// GetSyncStatus returns the auto-import status.
func (a *API) GetSyncStatus(c *gin.Context) {
	c.JSON(http.StatusOK, a.importer.Status())
}

// StartSync launches the auto-importer watcher.
func (a *API) StartSync(c *gin.Context) {
	if err := a.importer.Start(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a.importer.Status())
}

// StopSync stops the auto-importer watcher.
func (a *API) StopSync(c *gin.Context) {
	if err := a.importer.Stop(); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, a.importer.Status())
}

// Scrobble records a play event so the Home view can show recent listens.
func (a *API) Scrobble(c *gin.Context) {
	var req struct {
		Path     string `json:"path"`
		Duration int    `json:"duration"` // seconds, optional
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'path' is required."})
		return
	}
	// Only accept paths inside the downloads directory.
	if _, err := a.scanner.ResolvePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Duration < 0 {
		req.Duration = 0
	}
	a.history.Add(req.Path, req.Duration)
	c.JSON(http.StatusOK, gin.H{"message": "Play recorded"})
}

// GetStats returns aggregated listening stats for the Stats module.
func (a *API) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, a.history.Stats())
}

// GetRecentHistory returns the last played tracks, most recent first.
func (a *API) GetRecentHistory(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	recs := a.history.Recent(limit)
	c.JSON(http.StatusOK, gin.H{"history": recs})
}

// GetTopTracks returns the most played tracks, descending.
func (a *API) GetTopTracks(c *gin.Context) {
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	c.JSON(http.StatusOK, gin.H{"top": a.history.MostPlayed(limit)})
}

// GetLikes returns the set of liked rel_paths.
func (a *API) GetLikes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"likes": a.likes.List()})
}

// AddLike likes a track.
func (a *API) AddLike(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'path' is required."})
		return
	}
	if _, err := a.scanner.ResolvePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := a.likes.Add(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Track liked"})
}

// RemoveLike unlikes a track.
func (a *API) RemoveLike(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}
	if _, err := a.likes.Remove(path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Track unliked"})
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
				// Les candidats sont les fichiers sans paroles + ceux en texte
				// brut (à synchroniser).
				total := 0.0
				if v, ok := evt["missing_lrc"].(float64); ok {
					total += v
				}
				if v, ok := evt["unsynced_lrc"].(float64); ok {
					total += v
				}
				a.lyricsJob.Total = int(total)
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
			case "kept":
				a.lyricsJob.Kept++
				if name, ok := evt["filename"].(string); ok {
					a.lyricsJob.Logs = append([]string{"ℹ️ " + name + " (texte brut, pas de version synced)"}, a.lyricsJob.Logs...)
				}
			case "done":
				if v, ok := evt["success"].(float64); ok {
					a.lyricsJob.Success = int(v)
				}
				if v, ok := evt["failed"].(float64); ok {
					a.lyricsJob.Failed = int(v)
				}
				if v, ok := evt["kept"].(float64); ok {
					a.lyricsJob.Kept = int(v)
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
