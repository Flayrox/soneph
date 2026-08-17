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

// Pin est une ligne de la table pins (référence texte — M3).
type Pin struct {
	Kind     string    `json:"kind"` // artist/album/playlist
	Value    string    `json:"value"`
	PinnedAt time.Time `json:"pinned_at,omitempty"`
}

// PlayerQueue est l'état persistant de la file de lecture (M3) : ordre des
// rel_paths + index courant. Stocké en JSON dans la table settings.
type PlayerQueue struct {
	Queue []string `json:"queue"`
	Index int      `json:"index"`
}

// PlayRecord est une écoute enregistrée (table history).
type PlayRecord struct {
	Path     string    `json:"path"`
	PlayedAt time.Time `json:"played_at"`
	Duration int       `json:"duration"` // secondes (ms_played/1000)
}

// Count est un agrégat de lecture (morceau le plus écouté).
type Count struct {
	Path  string `json:"path"`
	Plays int    `json:"plays"`
}

// ArtistCount / DayCount sont les agrégats du Stats (même forme que
// l'ancien store JSON, pour ne rien changer côté front).
type ArtistCount struct {
	Artist string `json:"artist"`
	Plays  int    `json:"plays"`
}

type DayCount struct {
	Day   string `json:"day"` // YYYY-MM-DD
	Plays int    `json:"plays"`
}

// Stats agrège toute l'historique : total d'écoutes, temps d'écoute,
// top artistes (segment de dossier), top morceaux et écoutes par jour
// sur les 14 derniers jours.
type Stats struct {
	TotalPlays   int           `json:"total_plays"`
	TotalSeconds int           `json:"total_seconds"`
	TopArtists   []ArtistCount `json:"top_artists"`
	TopTracks    []Count       `json:"top_tracks"`
	PlaysByDay   []DayCount    `json:"plays_by_day"`
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

	// Pins (M3) ─────────────────────────────────────────────────────
	ListPins() ([]Pin, error)
	AddPin(kind, value string) error
	RemovePin(kind, value string) error

	// Player queue (M3) ─────────────────────────────────────────────
	GetPlayerQueue() (PlayerQueue, error)
	SetPlayerQueue(q PlayerQueue) error

	// Likes (table likes, join tracks) ──────────────────────────────
	LikeTrack(path string) error
	UnlikeTrack(path string) error
	ListLikedPaths() ([]string, error)

	// History (table history, join tracks) ──────────────────────────
	AddPlay(path string, durationSec int) error
	RecentPlays(limit int) ([]PlayRecord, error)
	MostPlayed(limit int) ([]Count, error)
	TotalPlays() (int, error)
	Stats() (Stats, error)

	// RenameTrack déplace un chemin (single → album, doublon supprimé) :
	// les likes/history/playlists en base suivent via le track_id.
	RenameTrack(oldPath, newPath string) error

	// Jobs (file de tâches persistante, M4) ─────────────────────────
	CreateJob(j Job) error
	ListJobs(status string, limit int) ([]Job, error)
	UpdateJobStatus(id, status, errMsg string) error
}
