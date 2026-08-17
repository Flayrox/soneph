package handler

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"soneph-backend/pkg/tags"

	"github.com/gin-gonic/gin"
)

// GetFileDetails returns the full ID3 metadata of one file (artists,
// producers, lyrics source, quality…) for the right-click details panel.
// M6 : lecture directe en Go (pkg/tags.FileDetails, port de
// file_details.py) — plus de sous-processus Python.
func (a *API) GetFileDetails(c *gin.Context) {
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

	details := tags.FileDetails(fullPath, relPath)
	if msg, ok := details["error"].(string); ok {
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, details)
}

// StreamFile sert le fichier audio (range requests gérées par http.ServeFile
// via c.File, ce qui permet le seek dans le player).
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

// GetCover extrait (ou sert en cache) la pochette embarquée d'un fichier.
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
		// M6 : extraction APIC en Go (pkg/tags.Cover, port de
		// extract_cover.py) — plus de sous-processus Python.
		if data, cerr := tags.Cover(fullPath); cerr == nil {
			_ = os.MkdirAll(coverDir, 0o755)
			_ = os.WriteFile(coverPath, data, 0o644)
		}
	}

	if _, err := os.Stat(coverPath); err == nil {
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(coverPath)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "No embedded artwork found"})
}

// GetLyrics retourne le fichier .lrc sidecar d'un morceau, s'il existe.
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
