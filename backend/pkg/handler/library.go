package handler

import (
	"net/http"
	"strconv"
	"strings"

	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/store"

	"github.com/gin-gonic/gin"
)

// GetDownloads retourne la liste complète de la bibliothèque (fichiers
// scannés, triés, avec métadonnées).
func (a *API) GetDownloads(c *gin.Context) {
	files, err := a.scanner.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// GetLibrary sert la bibliothèque depuis la base (M2 : SQLite source de
// vérité) : nombre total + morceaux (artiste/album résolus), paginés via
// limit/offset.
func (a *API) GetLibrary(c *gin.Context) {
	limit, offset := 200, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	tracks, err := a.st.ListTracks(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	count, err := a.st.CountTracks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count, "tracks": tracks})
}

// Search sert la recherche FTS5 (titre/artiste/album, préfixe) depuis la
// base. Requête : q (libre), limit (défaut 50).
func (a *API) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"query": "", "tracks": []store.Track{}})
		return
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	tracks, err := a.st.SearchTracks(q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"query": q, "tracks": tracks})
}

// Rescan synchronise la bibliothèque avec le disque : scan delta par mtime
// (jamais un scan complet d'écritures) puis upsert en base. Retourne les
// compteurs du sync.
func (a *API) Rescan(c *gin.Context) {
	files, err := a.scanner.ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stats, err := a.st.SyncLibrary(files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

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
