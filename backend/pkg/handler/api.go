package handler

import (
	"sync"
	"time"

	"soneph-backend/pkg/auth"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/history"
	"soneph-backend/pkg/playlists"
	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/syncmgr"

	"github.com/gin-gonic/gin"
)

// API regroupe les dépendances partagées par tous les handlers HTTP.
type API struct {
	downloader  *downloader.Manager
	scanner     *storage.Scanner
	importer    *syncmgr.Importer
	playlists   *playlists.Store
	history     *history.Store
	likes       *history.LikesStore
	wsHub       *WSHub
	lyricsJobMu sync.Mutex
	lyricsJob   *lyricsRetryJob

	// playlistTasks associe chaque tâche de téléchargement (task ID) à la
	// playlist créée en même temps qu'elle (lien playlist collé dans l'app).
	// À la fin du téléchargement, les morceaux manquants arrivés sur disque
	// y sont ajoutés (afterDownload).
	playlistTasksMu sync.Mutex
	playlistTasks   map[string]string
}

// NewAPI construit l'API avec ses dépendances et câble les callbacks du
// moteur de téléchargement (migration des stats, complétion des playlists).
func NewAPI(dl *downloader.Manager, sc *storage.Scanner, imp *syncmgr.Importer, pls *playlists.Store, hist *history.Store, likes *history.LikesStore, hub *WSHub) *API {
	a := &API{
		downloader:    dl,
		scanner:       sc,
		importer:      imp,
		playlists:     pls,
		history:       hist,
		likes:         likes,
		wsHub:         hub,
		lyricsJob:     &lyricsRetryJob{Status: "idle"},
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

// RegisterRoutes enregistre toutes les routes /api sur le routeur, protégées
// par le token optionnel (SONEPH_TOKEN) et la limite de débit par IP. La
// poignée de main.go garde le reste côté serveur (CORS, SPA fallback).
func (a *API) RegisterRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api")
	apiGroup.Use(auth.RequireToken(), auth.RateLimit(120, time.Minute))
	{
		apiGroup.POST("/download", a.CreateDownload)
		apiGroup.GET("/tasks", a.GetTasks)
		apiGroup.GET("/downloads", a.GetDownloads)
		apiGroup.DELETE("/downloads", a.DeleteDownload)
		apiGroup.GET("/stream", a.StreamFile)
		apiGroup.GET("/file/details", a.GetFileDetails)
		apiGroup.GET("/cover", a.GetCover)
		apiGroup.GET("/lyrics", a.GetLyrics)
		apiGroup.GET("/lyrics/missing", a.ScanMissingLyrics)
		apiGroup.POST("/lyrics/retry", a.RetryLyrics)
		apiGroup.GET("/lyrics/retry", a.GetLyricsJobStatus)
		apiGroup.GET("/settings", a.GetSettings)
		apiGroup.POST("/settings", a.SaveSettings)
		apiGroup.GET("/playlists", a.ListPlaylists)
		apiGroup.POST("/playlists", a.CreatePlaylist)
		apiGroup.POST("/playlists/from-url", a.CreatePlaylistFromURL)
		apiGroup.DELETE("/playlists/:id", a.DeletePlaylist)
		apiGroup.GET("/playlists/:id", a.GetPlaylist)
		apiGroup.POST("/playlists/:id/tracks", a.AddPlaylistTrack)
		apiGroup.DELETE("/playlists/:id/tracks", a.RemovePlaylistTrack)
		apiGroup.POST("/playlists/:id/order", a.ReorderPlaylist)
		apiGroup.POST("/scrobble", a.Scrobble)
		apiGroup.GET("/history/recent", a.GetRecentHistory)
		apiGroup.GET("/history/top", a.GetTopTracks)
		apiGroup.GET("/stats", a.GetStats)
		apiGroup.GET("/likes", a.GetLikes)
		apiGroup.POST("/likes", a.AddLike)
		apiGroup.DELETE("/likes", a.RemoveLike)
		apiGroup.GET("/sync/status", a.GetSyncStatus)
		apiGroup.POST("/sync/start", a.StartSync)
		apiGroup.POST("/sync/stop", a.StopSync)
		apiGroup.GET("/duplicates", a.FindDuplicates)
		apiGroup.POST("/duplicates/remove", a.RemoveDuplicates)
		apiGroup.POST("/playlists/export", a.ExportPlaylists)
		apiGroup.GET("/ws", a.wsHub.HandleWS)
	}
}
