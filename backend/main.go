package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"soneph-backend/pkg/auth"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/handler"
	"soneph-backend/pkg/history"
	"soneph-backend/pkg/playlists"
	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/store"
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

	// Le dossier peut être un lien symbolique (ex. l'app desktop pointe sur
	// ~/Music/soneph → downloads/). filepath.Walk ne descend pas dans une
	// racine qui est un symlink : on résout le chemin réel une fois pour
	// toute — le scanner, le téléchargeur et les scripts voient tous le
	// même dossier physique.
	if resolved, err := filepath.EvalSymlinks(downloadDir); err == nil {
		downloadDir = resolved
	}

	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		slog.Error("failed to load embedded frontend", "err", err)
		os.Exit(1)
	}

	wsHub := handler.NewWSHub()

	dlManager := downloader.NewManager(downloadDir, wsHub.Broadcast)

	// Diagnostic précoce : si le moteur de téléchargement n'est pas installé
	// (ou hors PATH), chaque tâche échouera. On le signale dès le démarrage
	// pour que l'utilisateur voie le problème dans les logs du serveur.
	if downloader.EnginePath() == "" {
		slog.Warn("moteur de téléchargement introuvable — les téléchargements échoueront",
			"engine", downloader.EngineBin(),
			"hint", "pipx install spotdl (ou : pip install spotdl) puis relance l'app",
		)
	}
	scanner := storage.NewScanner(downloadDir)
	importer := syncmgr.New(downloadDir)
	playlistStore := playlists.New()
	historyStore := history.New()
	likesStore := history.NewLikes()

	// SQLite : source de vérité (M2). Migrations goose appliquées à
	// l'ouverture ; le scan initial (boot) peuple la base — les syncs
	// suivants sont des deltas par mtime (POST /api/rescan).
	dbPath := os.Getenv("SONEPH_DB")
	if dbPath == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			d = "."
		}
		dbPath = filepath.Join(d, "soneph", "soneph.db")
	}
	st, err := store.Open(dbPath)
	if err != nil {
		slog.Error("ouverture de la base impossible", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if files, err := scanner.ListFiles(); err == nil {
		stats, _ := st.SyncLibrary(files)
		slog.Info("scan initial (boot)", "db", dbPath, "scanned", stats.Scanned, "added", stats.Added, "updated", stats.Updated)
	} else {
		slog.Warn("scan initial impossible", "err", err)
	}

	api := handler.NewAPI(dlManager, scanner, importer, playlistStore, historyStore, likesStore, st, wsHub)

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

	// Routes API — enregistrées par le package handler (token + rate limit
	// appliqués dans RegisterRoutes). Le handshake WebSocket vit dans le même
	// groupe pour hériter des mêmes contrôles.
	api.RegisterRoutes(r)

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
