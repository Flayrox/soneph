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
	"regexp"
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

	// playlistTasks associe chaque tâche de téléchargement (task ID) à la
	// playlist créée en même temps qu'elle (lien playlist collé dans l'app).
	// À la fin du téléchargement, les morceaux manquants arrivés sur disque
	// y sont ajoutés (afterDownload).
	playlistTasksMu sync.Mutex
	playlistTasks   map[string]string
}

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

type DownloadRequest struct {
	URL     string `json:"url" binding:"required"`
	Bitrate string `json:"bitrate"`
	Order   string `json:"order"`
}

func NewAPI(dl *downloader.Manager, sc *storage.Scanner, imp *syncmgr.Importer, pls *playlists.Store, hist *history.Store, likes *history.LikesStore) *API {
	a := &API{
		downloader: dl,
		scanner:    sc,
		importer:   imp,
		playlists:  pls,
		history:    hist,
		likes:      likes,
		lyricsJob:  &lyricsRetryJob{Status: "idle"},
		playlistTasks: map[string]string{},
	}
	// Quand le moteur déplace un fichier (single → album), on migre les
	// stats (historique, likes, playlists) vers le nouveau chemin.
	dl.SetOnFilesMoved(a.migrateMovedStats)
	// Quand un téléchargement se termine, on complète la playlist créée en
	// même temps que lui (les morceaux manquants viennent d'arriver).
	dl.SetOnTaskDone(a.afterDownload)
	return a
}

// migrateMovedStats ré-attache les stats (écoutes, likes, playlists) aux
// nouveaux chemins des fichiers que le moteur a déplacés.
func (a *API) migrateMovedStats(moves []downloader.FileMove) {
	for _, mv := range moves {
		a.migrateStats(mv.OldRel, mv.NewRel)
	}
}

// migrateStats ré-attache les stats d'un ancien chemin vers un nouveau
// (fichier déplacé par le moteur, doublon supprimé, album supprimé…).
func (a *API) migrateStats(oldPath, newPath string) {
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return
	}
	a.history.Rename(oldPath, newPath)
	a.likes.Rename(oldPath, newPath)
	a.playlists.RenameTrack(oldPath, newPath)
}

// isPlaylistLink détecte un lien de playlist Spotify (open.spotify.com ou
// URI spotify:playlist:). Les albums ne déclenchent pas de création de
// playlist — l'app les regroupe déjà par album dans la Collection.
func isPlaylistLink(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(u, "/playlist/") || strings.Contains(u, "spotify:playlist:")
}

// resolvedTrack / resolvedMissing sont les formes JSON de playlist_from_url.py.
type resolvedTrack struct {
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	RelPath string `json:"rel_path"`
}

type resolvedMissing struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// resolvePlaylistURL interroge playlist_from_url.py (API embed Spotify) :
// nom de la playlist + morceaux déjà sur disque (matched) + manquants.
func (a *API) resolvePlaylistURL(url string) (name string, matched []resolvedTrack, missing []resolvedMissing, total int, err error) {
	pythonExec := downloader.GetPythonExec()
	script := downloader.GetScriptPath("playlist_from_url.py")
	cmd := exec.Command(pythonExec, script, a.scanner.DownloadDir, strings.TrimSpace(url))
	output, err := cmd.Output()
	if err != nil {
		return "", nil, nil, 0, err
	}
	var res struct {
		Name    string            `json:"name"`
		Matched []resolvedTrack   `json:"matched"`
		Missing []resolvedMissing `json:"missing"`
		Total   int               `json:"total"`
	}
	if err := json.Unmarshal(output, &res); err != nil {
		return "", nil, nil, 0, err
	}
	return res.Name, res.Matched, res.Missing, res.Total, nil
}

// createPlaylistForURL crée la playlist (morceaux déjà sur disque ajoutés
// immédiatement) et mémorise la correspondance task → playlist pour que
// afterDownload complète à la fin du téléchargement.
func (a *API) createPlaylistForURL(taskID, url string) *gin.H {
	name, matched, missing, total, err := a.resolvePlaylistURL(url)
	if err != nil {
		return nil
	}
	if name == "" {
		name = "Playlist"
	}
	pl, err := a.playlists.Create(name)
	if err != nil {
		return nil
	}
	added := 0
	for _, m := range matched {
		// Sécurité : on ne référence que des chemins valides de la bibliothèque.
		if _, err := a.scanner.ResolvePath(m.RelPath); err != nil {
			continue
		}
		if _, err := a.playlists.AddTrack(pl.ID, m.RelPath); err == nil {
			added++
		}
	}
	a.playlistTasksMu.Lock()
	a.playlistTasks[taskID] = pl.ID
	a.playlistTasksMu.Unlock()
	return &gin.H{
		"id":          pl.ID,
		"name":        pl.Name,
		"added_now":   added,
		"matched":     len(matched),
		"to_download": len(missing),
		"total":       total,
	}
}

// afterDownload complète la playlist créée pour ce téléchargement : les
// morceaux manquants viennent d'arriver sur disque, on les ajoute (AddTrack
// dédoublonne, donc re-ajouter un morceau déjà présent est sans effet).
// Lancé dans une goroutine pour ne jamais bloquer la file de téléchargement
// sur l'appel réseau de résolution.
func (a *API) afterDownload(task *downloader.DownloadTask) {
	if task == nil || task.Status != downloader.StatusCompleted {
		return
	}
	a.playlistTasksMu.Lock()
	plID := a.playlistTasks[task.ID]
	delete(a.playlistTasks, task.ID)
	a.playlistTasksMu.Unlock()
	if plID == "" {
		return
	}

	go func(playlistID, url string) {
		_, matched, _, _, err := a.resolvePlaylistURL(url)
		if err != nil {
			return
		}
		added := 0
		for _, m := range matched {
			if _, err := a.scanner.ResolvePath(m.RelPath); err != nil {
				continue
			}
			if _, err := a.playlists.AddTrack(playlistID, m.RelPath); err == nil {
				added++
			}
		}
		if added > 0 {
			a.downloader.Broadcast("playlist_updated", gin.H{
				"playlist_id": playlistID,
				"added":       added,
			})
		}
	}(plID, task.URL)
}

func (a *API) CreateDownload(c *gin.Context) {
	var req DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body. 'url' is required."})
		return
	}

	task := a.downloader.AddTask(req.URL, req.Bitrate, req.Order)

	// Lien playlist : téléchargement + création de la playlist en même temps.
	// Les morceaux déjà sur disque y sont ajoutés immédiatement ; les autres
	// le seront à la fin du téléchargement (afterDownload).
	var playlistInfo *gin.H
	if isPlaylistLink(req.URL) {
		playlistInfo = a.createPlaylistForURL(task.ID, req.URL)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "Download task queued",
		"task":     task,
		"playlist": playlistInfo,
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

	// Migration inverse : si une autre copie du même morceau (même URL
	// Spotify dans les tags) existe encore — par ex. on supprime le fichier
	// album et il reste le single — on recolle les stats dessus.
	var statsMigrated *gin.H
	identity := ""
	before := a.downloader.ScanIdentityMap()
	for id, paths := range before {
		for _, p := range paths {
			if p == relPath {
				identity = id
			}
		}
	}
	sibling := ""
	if identity != "" {
		for _, p := range before[identity] {
			if p != relPath {
				sibling = p
				break
			}
		}
	}

	err := a.scanner.DeleteFile(relPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if sibling != "" {
		a.migrateStats(relPath, sibling)
		statsMigrated = &gin.H{"from": relPath, "to": sibling}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "File deleted successfully",
		"stats_migrated": statsMigrated,
	})
}

// GetFileDetails returns the full ID3 metadata of one file (artists,
// producers, lyrics source, quality…) for the right-click details panel.
func (a *API) GetFileDetails(c *gin.Context) {
	relPath := c.Query("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query param 'path' is required"})
		return
	}
	if _, err := a.scanner.ResolvePath(relPath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pythonExec := downloader.GetPythonExec()
	detailsScript := downloader.GetScriptPath("file_details.py")
	cmd := exec.Command(pythonExec, detailsScript, a.scanner.DownloadDir, relPath)
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Impossible de lire les métadonnées du fichier"})
		return
	}
	var details map[string]interface{}
	if err := json.Unmarshal(output, &details); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Réponse du script illisible"})
		return
	}
	if msg, ok := details["error"].(string); ok {
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, details)
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

// CreatePlaylistFromURL crée une playlist dans l'app à partir d'un lien
// (playlist/album) en résolvant les morceaux DÉJÀ présents sur disque — sans
// rien télécharger. Idéal quand on a déjà les sons et qu'on veut juste la
// playlist. Les morceaux manquants sont renvoyés pour info.
func (a *API) CreatePlaylistFromURL(c *gin.Context) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'url' est requis"})
		return
	}

	pythonExec := downloader.GetPythonExec()
	script := downloader.GetScriptPath("playlist_from_url.py")
	cmd := exec.Command(pythonExec, script, a.scanner.DownloadDir, strings.TrimSpace(req.URL))
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Impossible de résoudre le lien"})
		return
	}
	var res struct {
		Name    string `json:"name"`
		Matched []struct {
			Title   string `json:"title"`
			Artist  string `json:"artist"`
			RelPath string `json:"rel_path"`
		} `json:"matched"`
		Missing []struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
		} `json:"missing"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(output, &res); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Réponse du script illisible"})
		return
	}
	if len(res.Matched) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Aucun morceau de ce lien n'est déjà sur disque — télécharge-le d'abord, puis crée la playlist",
			"missing": res.Missing,
		})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = res.Name
	}
	p, err := a.playlists.Create(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, m := range res.Matched {
		// Sécurité : on ne référence que des chemins valides de la bibliothèque.
		if _, err := a.scanner.ResolvePath(m.RelPath); err != nil {
			continue
		}
		_, _ = a.playlists.AddTrack(p.ID, m.RelPath)
	}

	c.JSON(http.StatusCreated, gin.H{
		"playlist": p,
		"matched":  len(res.Matched),
		"missing":  res.Missing,
		"total":    res.Total,
	})
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

// ── Dédoublonnage ────────────────────────────────────────────────────────

// FindDuplicates returns groups of library files that look like the same
// track (Apple Music rips, "(1)" copies…). The user keeps one per group.
func (a *API) FindDuplicates(c *gin.Context) {
	files, err := a.scanner.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	groups := storage.FindDuplicates(files)
	total := 0
	for _, g := range groups {
		total += len(g.Files) - 1
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups, "total": total})
}

// RemoveDuplicates deletes the given rel_paths (and their .lrc files). Les
// stats (écoutes, likes, playlists) des copies supprimées sont transférées
// au fichier gardé (keep_for : chemin supprimé → chemin conservé).
func (a *API) RemoveDuplicates(c *gin.Context) {
	var req struct {
		Paths   []string          `json:"paths"`
		KeepFor map[string]string `json:"keep_for"` // chemin supprimé → chemin gardé
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'paths' (liste de chemins) attendu"})
		return
	}
	deleted, failed := 0, 0
	var firstErr error
	for _, p := range req.Paths {
		if err := a.scanner.DeleteFile(p); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
		} else {
			deleted++
			// La copie supprimée laisse ses stats à la copie gardée.
			if keep, ok := req.KeepFor[p]; ok {
				a.migrateStats(p, keep)
			}
		}
	}
	if firstErr != nil {
		c.JSON(http.StatusMultiStatus, gin.H{"deleted": deleted, "failed": failed, "error": firstErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// ── Export playlists (iPhone / Syncthing) ────────────────────────────────

// ExportPlaylists writes one .m3u8 per playlist into the requested folder
// (a Syncthing folder or an iPhone mounted via USB). Tracks are referenced
// by their relative path so the same tree on the phone resolves them.
func (a *API) ExportPlaylists(c *gin.Context) {
	var req struct {
		Dir string `json:"dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Dir) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'dir' (dossier d'export) attendu"})
		return
	}

	dir := strings.TrimSpace(req.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Impossible de créer le dossier : " + err.Error()})
		return
	}

	files, err := a.scanner.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	byRel := make(map[string]storage.DownloadedFile, len(files))
	for _, f := range files {
		byRel[f.RelPath] = f
	}

	summaries, err := a.playlists.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var written []gin.H
	for _, s := range summaries {
		pl, err := a.playlists.Get(s.ID)
		if err != nil {
			continue
		}
		safe := sanitizeFilename(pl.Name)
		var sb strings.Builder
		sb.WriteString("#EXTM3U\r\n")
		for _, rel := range pl.Tracks {
			f, ok := byRel[rel]
			if !ok {
				continue
			}
			sb.WriteString("#EXTINF:-1," + f.Artist + " - " + f.Title + "\r\n")
			sb.WriteString(filepath.ToSlash(rel) + "\r\n")
		}
		out := filepath.Join(dir, safe+".m3u8")
		if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		written = append(written, gin.H{"name": safe + ".m3u8", "track_count": len(pl.Tracks)})
	}

	// Mémorise le dossier pour la prochaine fois.
	cfg := config.Load()
	cfg.PlaylistExportDir = dir
	_ = config.Save(cfg)

	c.JSON(http.StatusOK, gin.H{"dir": dir, "files": written, "count": len(written)})
}

// sanitizeFilename rend un nom de playlist sûr pour un nom de fichier.
func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|]+`)
	name = re.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Playlist"
	}
	return name
}

