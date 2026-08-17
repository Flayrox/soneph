// Package store est l'unique point d'accès aux données de soneph (SQLite).
// Aucun SQL ne doit exister hors de ce package : c'est ce qui rend possible
// le remplacement ultérieur par Postgres (mode compatible) sans toucher au
// reste du code.
package store

import (
	"time"

	"soneph-backend/pkg/storage"
)

// Artist est la forme JSON d'une ligne de la table artists.
type Artist struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	MBID     string `json:"mbid,omitempty"`
	SortName string `json:"sort_name,omitempty"`
}

// Album est la forme JSON d'une ligne de la table albums.
type Album struct {
	ID        int64  `json:"id"`
	ArtistID  int64  `json:"artist_id"`
	Title     string `json:"title"`
	MBID      string `json:"mbid,omitempty"`
	Year      int    `json:"year,omitempty"`
	CoverPath string `json:"cover_path,omitempty"`
}

// Track est la forme JSON d'une ligne de la table tracks, dénormalisée pour
// l'API : Artist et Album sont les noms résolus par jointure.
type Track struct {
	ID           int64     `json:"id"`
	Path         string    `json:"path"`
	Title        string    `json:"title"`
	Artist       string    `json:"artist,omitempty"`
	Album        string    `json:"album,omitempty"`
	TrackNo      int       `json:"track_no,omitempty"`
	DurationMs   int       `json:"duration_ms,omitempty"`
	Bitrate      int       `json:"bitrate,omitempty"`
	Format       string    `json:"format,omitempty"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	ISRC         string    `json:"isrc,omitempty"`
	Fingerprint  string    `json:"fingerprint,omitempty"`
	AcoustID     string    `json:"acoustid,omitempty"`
	LyricsPath   string    `json:"lyrics_path,omitempty"`
	LyricsSynced bool      `json:"lyrics_synced"`
	QualityScore int       `json:"quality_score,omitempty"`
	AddedAt      time.Time `json:"added_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SyncStats résume un SyncLibrary : combien de fichiers scannés, ajoutés,
// mis à jour ou inchangés (delta par mtime).
type SyncStats struct {
	Scanned   int `json:"scanned"`
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
}

// Job est la forme JSON d'une ligne de la table jobs (file de tâches, M4).
type Job struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Payload     string     `json:"payload"`
	Status      string     `json:"status"` // queued/running/done/failed
	Priority    int        `json:"priority"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Store est l'interface que tout accès aux données doit utiliser. Les
// handlers et le moteur de téléchargement ne connaissent jamais SQL.
type Store interface {
	Close() error

	// Library ────────────────────────────────────────────────────────
	// SyncLibrary insère/maj les fichiers scannés (delta par mtime :
	// un fichier inchangé n'écrit rien). Ne supprime jamais de lignes —
	// la réconciliation des fichiers disparus arrive avec M6.
	SyncLibrary(files []storage.DownloadedFile) (SyncStats, error)
	ListTracks(limit, offset int) ([]Track, error)
	TrackByPath(path string) (Track, error)
	CountTracks() (int, error)
	// SearchTracks cherche par titre/artiste/album (FTS5, préfixe).
	SearchTracks(query string, limit int) ([]Track, error)

	// Settings (table settings, migration 0002) ─────────────────────
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error

	// Jobs (file de tâches persistante, M4) ─────────────────────────
	CreateJob(j Job) error
	ListJobs(status string, limit int) ([]Job, error)
	UpdateJobStatus(id, status, errMsg string) error
}
