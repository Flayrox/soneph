package handler

import (
	"net/http"

	"soneph-backend/pkg/storage"

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
