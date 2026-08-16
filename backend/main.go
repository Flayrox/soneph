package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/handler"
	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/syncmgr"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// The built Vite frontend lives in web/dist (see backend/web/dist/index.html
// placeholder for local builds; the Dockerfile overwrites it with the real
// build). It is embedded so the binary serves both the API and the SPA on
// the same origin — no Node server, no CORS needed in production.
//
//go:embed web/dist
var webDist embed.FS

func main() {
	downloadDir := os.Getenv("DOWNLOAD_DIR")
	if downloadDir == "" {
		if _, err := os.Stat("/app/downloads"); err == nil {
			downloadDir = "/app/downloads"
		} else if _, err := os.Stat("../downloads"); err == nil {
			// Local dev from backend/: use the repo-root downloads folder so
			// the same files are served, synced and imported as in Docker.
			downloadDir = "../downloads"
		} else {
			downloadDir = "./downloads"
		}
	}

	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatalf("Failed to load embedded frontend: %v", err)
	}

	wsHub := handler.NewWSHub()

	dlManager := downloader.NewManager(downloadDir, wsHub.Broadcast)
	scanner := storage.NewScanner(downloadDir)
	importer := syncmgr.New(downloadDir)
	api := handler.NewAPI(dlManager, scanner, importer)

	r := gin.Default()

	// Configure CORS for development (Vite dev server on :5173)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API Routes
	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/download", api.CreateDownload)
		apiGroup.GET("/tasks", api.GetTasks)
		apiGroup.GET("/downloads", api.GetDownloads)
		apiGroup.DELETE("/downloads", api.DeleteDownload)
		apiGroup.GET("/stream", api.StreamFile)
		apiGroup.GET("/cover", api.GetCover)
		apiGroup.GET("/lyrics", api.GetLyrics)
		apiGroup.GET("/lyrics/missing", api.ScanMissingLyrics)
		apiGroup.POST("/lyrics/retry", api.RetryLyrics)
		apiGroup.GET("/lyrics/retry", api.GetLyricsJobStatus)
		apiGroup.GET("/settings", api.GetSettings)
		apiGroup.POST("/settings", api.SaveSettings)
		apiGroup.GET("/sync/status", api.GetSyncStatus)
		apiGroup.POST("/sync/start", api.StartSync)
		apiGroup.POST("/sync/stop", api.StopSync)
		apiGroup.GET("/ws", wsHub.HandleWS)
	}

	// SPA fallback: serve the embedded frontend for any non-API route.
	// http.FileServer resolves / -> index.html and serves static assets;
	// any missing path falls back to index.html (client-side routing).
	staticServer := http.FileServer(http.FS(distFS))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}
		name := strings.TrimPrefix(p, "/")
		if _, err := fs.Stat(distFS, name); err != nil {
			c.Request.URL.Path = "/"
		}
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	// Some environments (CI, containers) inject PORT=0 for dynamic allocation;
	// fall back to the default so the web UI + API stay reachable.
	port := os.Getenv("PORT")
	if port == "" || port == "0" {
		port = "8080"
	}

	log.Printf("🚀 soneph Backend running on port :%s (API + Web UI)", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
