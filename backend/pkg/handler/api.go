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
	"strconv"
	"soneph-backend/pkg/config"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/history"
	"soneph-backend/pkg/playlists"
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
	playlists   *playlists.Store
	history     *history.Store
	likes       *history.LikesStore
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

func NewAPI(dl *downloader.Manager, sc *storage.Scanner, imp *syncmgr.Importer, pls *playlists.Store, hist *history.Store, likes *history.LikesStore) *API {
	return &API{
		downloader: dl,
		scanner:    sc,
		importer:   imp,
		playlists:  pls,
		history:    hist,
		likes:      likes,
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

	fullPath, err := a.scanner.ResolvePath(relPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.File(fullPath)
}

func (a *API) GetCover(c *gin.Context) {
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}

	fullPath, err := a.scanner.ResolvePath(relPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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

	fullPath, err := a.scanner.ResolvePath(relPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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

// ListPlaylists returns all playlists (id, name, track count).
func (a *API) ListPlaylists(c *gin.Context) {
	pls, err := a.playlists.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"playlists": pls})
}

// CreatePlaylist creates a new empty playlist.
func (a *API) CreatePlaylist(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'name' is required."})
		return
	}
	p, err := a.playlists.Create(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"playlist": p})
}

// DeletePlaylist removes a playlist and its file.
func (a *API) DeletePlaylist(c *gin.Context) {
	if err := a.playlists.Delete(c.Param("id")); err != nil {
		if err == playlists.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Playlist deleted"})
}

// GetPlaylist resolves a playlist's tracks against the scanned library
// (missing files are skipped).
func (a *API) GetPlaylist(c *gin.Context) {
	p, err := a.playlists.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
		return
	}
	files, err := a.scanner.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	byPath := make(map[string]storage.DownloadedFile, len(files))
	for _, f := range files {
		byPath[f.RelPath] = f
	}
	tracks := make([]storage.DownloadedFile, 0, len(p.Tracks))
	for _, tp := range p.Tracks {
		if f, ok := byPath[tp]; ok {
			tracks = append(tracks, f)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"playlist": gin.H{
			"id":    p.ID,
			"name":  p.Name,
			"tracks": tracks,
		},
	})
}

// AddPlaylistTrack appends a track (by rel_path) to a playlist.
func (a *API) AddPlaylistTrack(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'path' is required."})
		return
	}
	// Validate the path stays inside the downloads directory.
	if _, err := a.scanner.ResolvePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := a.playlists.AddTrack(c.Param("id"), req.Path)
	if err != nil {
		if err == playlists.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"playlist": p})
}

// RemovePlaylistTrack removes a track (by rel_path) from a playlist.
func (a *API) RemovePlaylistTrack(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}
	p, err := a.playlists.RemoveTrack(c.Param("id"), path)
	if err != nil {
		if err == playlists.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"playlist": p})
}

// ReorderPlaylist sets the playlist's track order (drag-and-drop reorder).
func (a *API) ReorderPlaylist(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'paths' is required."})
		return
	}
	p, err := a.playlists.Reorder(c.Param("id"), req.Paths)
	if err != nil {
		if err == playlists.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"playlist": p})
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

