package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"soneph-backend/pkg/auth"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/handler"
	"soneph-backend/pkg/history"
	"soneph-backend/pkg/playlists"
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
	// Structured logs. Default = text; LOG_FORMAT=json for machines.
	if os.Getenv("LOG_FORMAT") == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	}

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
		slog.Error("failed to load embedded frontend", "err", err)
		os.Exit(1)
	}

	wsHub := handler.NewWSHub()

	dlManager := downloader.NewManager(downloadDir, wsHub.Broadcast)
	scanner := storage.NewScanner(downloadDir)
	importer := syncmgr.New(downloadDir)
	playlistStore := playlists.New()
	historyStore := history.New()
	likesStore := history.NewLikes()
	api := handler.NewAPI(dlManager, scanner, importer, playlistStore, historyStore, likesStore)

	r := gin.Default()

	// Configure CORS for development (Vite dev server on :5173). Auth uses a
	// Bearer token header, not cookies, so credentials stay disabled.
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Auth-Token"},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	}))

	// API Routes — protected by an optional token (SONEPH_TOKEN) and a
	// per-IP rate limit. The WebSocket handshake lives here too, so it
	// inherits the same checks (token via ?token= query param).
	apiGroup := r.Group("/api")
	apiGroup.Use(auth.RequireToken(), auth.RateLimit(120, time.Minute))
	{
		apiGroup.POST("/download", api.CreateDownload)
		apiGroup.GET("/tasks", api.GetTasks)
		apiGroup.GET("/downloads", api.GetDownloads)
		apiGroup.DELETE("/downloads", api.DeleteDownload)
		apiGroup.GET("/stream", api.StreamFile)
		apiGroup.GET("/file/details", api.GetFileDetails)
		apiGroup.GET("/cover", api.GetCover)
		apiGroup.GET("/lyrics", api.GetLyrics)
		apiGroup.GET("/lyrics/missing", api.ScanMissingLyrics)
		apiGroup.POST("/lyrics/retry", api.RetryLyrics)
		apiGroup.GET("/lyrics/retry", api.GetLyricsJobStatus)
		apiGroup.GET("/settings", api.GetSettings)
		apiGroup.POST("/settings", api.SaveSettings)
		apiGroup.GET("/playlists", api.ListPlaylists)
		apiGroup.POST("/playlists", api.CreatePlaylist)
		apiGroup.POST("/playlists/from-url", api.CreatePlaylistFromURL)
		apiGroup.DELETE("/playlists/:id", api.DeletePlaylist)
		apiGroup.GET("/playlists/:id", api.GetPlaylist)
		apiGroup.POST("/playlists/:id/tracks", api.AddPlaylistTrack)
		apiGroup.DELETE("/playlists/:id/tracks", api.RemovePlaylistTrack)
		apiGroup.POST("/playlists/:id/order", api.ReorderPlaylist)
		apiGroup.POST("/scrobble", api.Scrobble)
		apiGroup.GET("/history/recent", api.GetRecentHistory)
		apiGroup.GET("/history/top", api.GetTopTracks)
		apiGroup.GET("/stats", api.GetStats)
		apiGroup.GET("/likes", api.GetLikes)
		apiGroup.POST("/likes", api.AddLike)
		apiGroup.DELETE("/likes", api.RemoveLike)
		apiGroup.GET("/sync/status", api.GetSyncStatus)
		apiGroup.POST("/sync/start", api.StartSync)
		apiGroup.POST("/sync/stop", api.StopSync)
		apiGroup.GET("/duplicates", api.FindDuplicates)
		apiGroup.POST("/duplicates/remove", api.RemoveDuplicates)
		apiGroup.POST("/playlists/export", api.ExportPlaylists)
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

	slog.Info("backend started", "port", port, "downloads", downloadDir, "auth", auth.TokenEnabled(), "api", "/api", "ui", "/")
	if err := r.Run(":" + port); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
