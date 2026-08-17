package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"soneph-backend/pkg/downloader"

	"github.com/gin-gonic/gin"
)

// DownloadRequest est le corps du POST /api/download.
type DownloadRequest struct {
	URL     string `json:"url" binding:"required"`
	Bitrate string `json:"bitrate"`
	Order   string `json:"order"`
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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		// Ne plus échouer silencieusement : en app packagée, le script peut
		// être introuvable (GetScriptPath) ou le Python sans mutagen.
		slog.Error("resolvePlaylistURL: script failed", "url", url, "script", script, "err", err, "stderr", stderr.String())
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
		slog.Error("createPlaylistForURL: resolution failed — playlist non créée", "url", url, "err", err)
		return nil
	}
	if name == "" {
		name = "Playlist"
	}
	pl, err := a.st.CreatePlaylist(name)
	if err != nil {
		return nil
	}
	added := 0
	for _, m := range matched {
		// Sécurité : on ne référence que des chemins valides de la bibliothèque.
		if _, err := a.scanner.ResolvePath(m.RelPath); err != nil {
			continue
		}
		if _, err := a.st.AddPlaylistTrack(pl.ID, m.RelPath); err == nil {
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
			if _, err := a.st.AddPlaylistTrack(playlistID, m.RelPath); err == nil {
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

// CreateDownload enfile une tâche de téléchargement (morceau, album ou
// playlist). Pour un lien playlist, la playlist est créée en parallèle.
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

// GetTasks retourne l'état courant de la file de téléchargement.
func (a *API) GetTasks(c *gin.Context) {
	tasks := a.downloader.GetTasks()
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// GetJobs liste les jobs récents de la file M4 (tous statuts, tous types),
// pour le panneau « jobs » du frontend. Chaque transition d'état y est
// aussi poussée en direct sur le WebSocket (job_update) — le GET ne sert
// qu'à la première hydratation et aux re-synchronisations.
func (a *API) GetJobs(c *gin.Context) {
	jobList, err := a.st.ListJobs("", 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobList})
}

// DeleteDownload supprime un fichier de la bibliothèque. Si une autre copie
// du même morceau (même URL Spotify dans les tags) existe encore, les stats
// (écoutes, likes, playlists) y sont migrées.
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
