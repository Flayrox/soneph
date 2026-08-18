package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"soneph-backend/pkg/config"
	"soneph-backend/pkg/fastfilter"
	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/store"
	"soneph-backend/pkg/tags"

	"github.com/gin-gonic/gin"
)

// ListPlaylists returns all playlists (id, name, track count).
func (a *API) ListPlaylists(c *gin.Context) {
	pls, err := a.st.ListPlaylists()
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
	p, err := a.st.CreatePlaylist(req.Name)
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

	// M6 : résolution en Go (pkg/fastfilter.ResolvePlaylist + carte
	// d'identité pkg/tags.IdentityMap) — plus de sous-processus Python.
	identity, ierr := tags.IdentityMap(a.scanner.DownloadDir)
	if ierr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Impossible de résoudre le lien"})
		return
	}
	res, rerr := fastfilter.ResolvePlaylist(strings.TrimSpace(req.URL), nil, identity)
	if rerr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Impossible de résoudre le lien"})
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
	p, err := a.st.CreatePlaylist(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, m := range res.Matched {
		// Sécurité : on ne référence que des chemins valides de la bibliothèque.
		if _, err := a.scanner.ResolvePath(m.RelPath); err != nil {
			continue
		}
		_, _ = a.st.AddPlaylistTrack(p.ID, m.RelPath)
	}

	c.JSON(http.StatusCreated, gin.H{
		"playlist": p,
		"matched":  len(res.Matched),
		"missing":  res.Missing,
		"total":    res.Total,
	})
}

// DeletePlaylist removes a playlist (et ses pistes, par cascade FK).
func (a *API) DeletePlaylist(c *gin.Context) {
	if err := a.st.DeletePlaylist(c.Param("id")); err != nil {
		if err == store.ErrNotFound {
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
	p, err := a.st.GetPlaylist(c.Param("id"))
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
			"id":     p.ID,
			"name":   p.Name,
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
	p, err := a.st.AddPlaylistTrack(c.Param("id"), req.Path)
	if err != nil {
		if err == store.ErrNotFound {
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
	p, err := a.st.RemovePlaylistTrack(c.Param("id"), path)
	if err != nil {
		if err == store.ErrNotFound {
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
	p, err := a.st.ReorderPlaylist(c.Param("id"), req.Paths)
	if err != nil {
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"playlist": p})
}

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

	summaries, err := a.st.ListPlaylists()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var written []gin.H
	for _, s := range summaries {
		pl, err := a.st.GetPlaylist(s.ID)
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
