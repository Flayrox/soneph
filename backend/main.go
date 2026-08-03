package main

import (
	"log"
	"os"
	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/handler"
	"soneph-backend/pkg/storage"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	downloadDir := os.Getenv("DOWNLOAD_DIR")
	if downloadDir == "" {
		downloadDir = "/app/downloads"
	}

	wsHub := handler.NewWSHub()

	dlManager := downloader.NewManager(downloadDir, wsHub.Broadcast)
	scanner := storage.NewScanner(downloadDir)
	api := handler.NewAPI(dlManager, scanner)

	r := gin.Default()

	// Configure CORS for Next.js frontend
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
		apiGroup.GET("/lyrics", api.GetLyrics)
		apiGroup.GET("/lyrics/missing", api.ScanMissingLyrics)
		apiGroup.POST("/lyrics/retry", api.RetryLyrics)
		apiGroup.GET("/lyrics/retry", api.GetLyricsJobStatus)
		apiGroup.GET("/ws", wsHub.HandleWS)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 soneph Backend running on port :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
