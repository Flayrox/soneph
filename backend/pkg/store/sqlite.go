package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pilote SQLite pur Go, zéro CGO

	"soneph-backend/pkg/storage"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLiteStore est l'implémentation SQLite de Store.
type SQLiteStore struct {
	db *sql.DB
}

// ErrNotFound est renvoyé quand une ligne demandée n'existe pas.
var ErrNotFound = errors.New("store: ligne introuvable")

// Open ouvre (ou crée) la base SQLite au chemin donné et applique les
// migrations goose embarquées avant de retourner le store prêt à l'emploi.
// Le DSN force WAL + foreign_keys + busy_timeout, indépendamment des PRAGMA
// de la migration 0001 (qui restent fidèles au schéma de référence).
func Open(dbPath string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: ouverture sqlite: %w", err)
	}
	// Une seule connexion : WAL + écrivain unique évitent SQLITE_BUSY à
	// cette échelle mono-instance (et goose partage la même connexion).
	db.SetMaxOpenConns(1)

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: dialect goose: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrations goose: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

// ── helpers ───────────────────────────────────────────────────────────────

const timeLayout = "2006-01-02 15:04:05"

// parseTime accepte les deux formats possibles de DATETIME SQLite :
// RFC3339Nano (écrit par SyncLibrary, mtime du fichier) et le format par
// défaut « YYYY-MM-DD HH:MM:SS » (CURRENT_TIMESTAMP).
func parseTime(v string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t
	}
	if t, err := time.Parse(timeLayout, v); err == nil {
		return t
	}
	return time.Time{}
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// ftsQuery transforme une saisie libre en requête MATCH FTS5 sûre : chaque
// mot est nettoyé (seuls lettres/chiffres restent) puis cherché en préfixe.
// « ra di » → `"ra"* AND "di"*` (recherche incrémentale pendant la frappe).
func ftsQuery(q string) string {
	// Découpe sur TOUT caractère non alphanumérique (espaces, ponctuation…)
	// puis ré-émet chaque mot en préfixe quoté : l'utilisateur ne peut pas
	// injecter d'opérateur FTS5 (OR, NOT, guillemets…) dans la requête.
	fields := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, `"`+f+`"*`)
	}
	return strings.Join(parts, " AND ")
}

// ── Library ───────────────────────────────────────────────────────────────

// trackColumns est le SELECT commun (nom d'artiste/album résolus).
const trackColumns = `
	SELECT t.id, t.path, t.title,
	       COALESCE(ar.name, ''), COALESCE(al.title, ''),
	       COALESCE(t.track_no, 0), COALESCE(t.duration_ms, 0),
	       COALESCE(t.bitrate, 0), COALESCE(t.format, ''), COALESCE(t.size_bytes, 0),
	       COALESCE(t.isrc, ''), COALESCE(t.fingerprint, ''), COALESCE(t.acoustid, ''),
	       COALESCE(t.lyrics_path, ''), COALESCE(t.lyrics_synced, 0), COALESCE(t.quality_score, 0),
	       t.added_at, t.updated_at
	  FROM tracks t
	  LEFT JOIN artists ar ON ar.id = t.artist_id
	  LEFT JOIN albums al ON al.id = t.album_id`

func scanTrack(row interface{ Scan(...any) error }) (Track, error) {
	var t Track
	var artist, album string
	var trackNo, durationMs, bitrate int
	var size int64
	var lyricsSynced, quality int
	var addedAt, updatedAt string
	err := row.Scan(&t.ID, &t.Path, &t.Title, &artist, &album,
		&trackNo, &durationMs, &bitrate, &t.Format, &size,
		&t.ISRC, &t.Fingerprint, &t.AcoustID,
		&t.LyricsPath, &lyricsSynced, &quality,
		&addedAt, &updatedAt)
	if err != nil {
		return t, err
	}
	t.Artist, t.Album = artist, album
	t.TrackNo, t.DurationMs, t.Bitrate = trackNo, durationMs, bitrate
	t.SizeBytes = size
	t.LyricsSynced = lyricsSynced == 1
	t.QualityScore = quality
	t.AddedAt, t.UpdatedAt = parseTime(addedAt), parseTime(updatedAt)
	return t, nil
}

// SyncLibrary upsert les fichiers scannés dans artists/albums/tracks et
// maintient l'index FTS5. C'est un vrai delta : un fichier dont le mtime
// n'a pas changé (updated_at identique) est ignoré sans aucune écriture.
func (s *SQLiteStore) SyncLibrary(files []storage.DownloadedFile) (SyncStats, error) {
	var stats SyncStats
	stats.Scanned = len(files)

	tx, err := s.db.Begin()
	if err != nil {
		return stats, fmt.Errorf("store: début de transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback silencieux après commit

	artistID := func(name string) (int64, error) {
		if _, err := tx.Exec(`INSERT INTO artists(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
			return 0, err
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM artists WHERE name = ?`, name).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	albumID := func(artistID int64, title string) (int64, error) {
		if _, err := tx.Exec(`INSERT INTO albums(artist_id, title) VALUES(?,?) ON CONFLICT(artist_id, title) DO NOTHING`, artistID, title); err != nil {
			return 0, err
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM albums WHERE artist_id = ? AND title = ?`, artistID, title).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}

	for _, f := range files {
		arID, err := artistID(f.Artist)
		if err != nil {
			return stats, fmt.Errorf("store: artiste %q: %w", f.Artist, err)
		}
		var alID any
		if f.Album != "" {
			id, err := albumID(arID, f.Album)
			if err != nil {
				return stats, fmt.Errorf("store: album %q: %w", f.Album, err)
			}
			alID = id
		}

		// Delta : même mtime → rien à écrire. On mémorise l'existence AVANT
		// l'upsert pour compter Added vs Updated (err est réutilisé ensuite).
		existed := true
		mtime := f.ModTime.UTC().Format(time.RFC3339Nano)
		var existingUpdated string
		err = tx.QueryRow(`SELECT updated_at FROM tracks WHERE path = ?`, f.RelPath).Scan(&existingUpdated)
		if err == sql.ErrNoRows {
			existed = false
		} else if err != nil {
			return stats, fmt.Errorf("store: lecture %q: %w", f.RelPath, err)
		} else if existingUpdated == mtime {
			stats.Unchanged++
			continue
		}

		title := f.Title
		if title == "" {
			title = f.FileName
		}
		lyricsSynced := 0
		if f.LyricsType == "synced" {
			lyricsSynced = 1
		}
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(f.RelPath)), ".")
		// track_no, duration_ms, bitrate, isrc : inconnus au scan (tags lus
		// par file_details.py — porté en Go en M5), laissés NULL pour l'instant.
		_, err = tx.Exec(`
			INSERT INTO tracks(path, title, artist_id, album_id, format,
			                   size_bytes, lyrics_path, lyrics_synced, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?)
			ON CONFLICT(path) DO UPDATE SET
			  title = excluded.title, artist_id = excluded.artist_id,
			  album_id = excluded.album_id, size_bytes = excluded.size_bytes,
			  format = excluded.format, lyrics_path = excluded.lyrics_path,
			  lyrics_synced = excluded.lyrics_synced, updated_at = excluded.updated_at`,
			f.RelPath, title, arID, alID, nullStr(format),
			nullNum64(f.Size), nullStr(f.LrcPath), lyricsSynced, mtime)
		if err != nil {
			return stats, fmt.Errorf("store: morceau %q: %w", f.RelPath, err)
		}

		// FTS5 en content='' : l'index est maintenu manuellement. Une table
		// contentless ne supporte pas DELETE ni le SELECT d'existence (une
		// rowid absente renvoie une ligne de NULL) : on se fie au flag existed
		// (ligne dans tracks) et on retire l'ancienne entrée via la commande
		// 'delete', qui exige les valeurs EXACTES de l'index — on les relit
		// depuis tracks/artists/albums (d'où elles viennent à l'insertion).
		var id int64
		if err := tx.QueryRow(`SELECT id FROM tracks WHERE path = ?`, f.RelPath).Scan(&id); err != nil {
			return stats, fmt.Errorf("store: récupération id %q: %w", f.RelPath, err)
		}
		if existed {
			var oldTitle, oldArtist, oldAlbum string
			err := tx.QueryRow(`
					SELECT t.title, ar.name, COALESCE(al.title, '')
					  FROM tracks t
					  LEFT JOIN artists ar ON ar.id = t.artist_id
					  LEFT JOIN albums al ON al.id = t.album_id
					 WHERE t.id = ?`, id).Scan(&oldTitle, &oldArtist, &oldAlbum)
			if err != nil {
				return stats, fmt.Errorf("store: lecture ancienne entrée FTS %q: %w", f.RelPath, err)
			}
			if _, err := tx.Exec(`INSERT INTO tracks_fts(tracks_fts, rowid, title, artist, album) VALUES('delete', ?, ?, ?, ?)`,
				id, oldTitle, oldArtist, oldAlbum); err != nil {
				return stats, fmt.Errorf("store: retrait FTS %q: %w", f.RelPath, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO tracks_fts(rowid, title, artist, album) VALUES(?,?,?,?)`,
			id, title, f.Artist, f.Album); err != nil {
			return stats, err
		}

		if existed {
			stats.Updated++
		} else {
			stats.Added++
		}
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("store: commit: %w", err)
	}
	return stats, nil
}

// nullNum évite d'écrire 0 comme valeur explicite (NULL = inconnu).
func nullNum(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullNum64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func (s *SQLiteStore) ListTracks(limit, offset int) ([]Track, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.Query(trackColumns+` ORDER BY ar.name, al.title, t.track_no LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: liste des morceaux: %w", err)
	}
	defer rows.Close()

	tracks := make([]Track, 0, 32)
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (s *SQLiteStore) TrackByPath(path string) (Track, error) {
	row := s.db.QueryRow(trackColumns+` WHERE t.path = ?`, path)
	t, err := scanTrack(row)
	if err == sql.ErrNoRows {
		return t, ErrNotFound
	}
	return t, err
}

func (s *SQLiteStore) CountTracks() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tracks`).Scan(&n)
	return n, err
}

func (s *SQLiteStore) SearchTracks(query string, limit int) ([]Track, error) {
	q := ftsQuery(query)
	if q == "" || limit <= 0 {
		return []Track{}, nil
	}
	rows, err := s.db.Query(`
		SELECT t.id, t.path, t.title,
		       COALESCE(ar.name, ''), COALESCE(al.title, ''),
		       COALESCE(t.track_no, 0), COALESCE(t.duration_ms, 0),
		       COALESCE(t.bitrate, 0), COALESCE(t.format, ''), COALESCE(t.size_bytes, 0),
		       COALESCE(t.isrc, ''), COALESCE(t.fingerprint, ''), COALESCE(t.acoustid, ''),
		       COALESCE(t.lyrics_path, ''), COALESCE(t.lyrics_synced, 0), COALESCE(t.quality_score, 0),
		       t.added_at, t.updated_at
		  FROM tracks_fts f
		  JOIN tracks t ON t.id = f.rowid
		  LEFT JOIN artists ar ON ar.id = t.artist_id
		  LEFT JOIN albums al ON al.id = t.album_id
		 WHERE tracks_fts MATCH ?
		 ORDER BY f.rank
		 LIMIT ?`, q, limit)
	if err != nil {
		return nil, fmt.Errorf("store: recherche %q: %w", query, err)
	}
	defer rows.Close()

	tracks := make([]Track, 0, 32)
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// ── Settings ──────────────────────────────────────────────────────────────

func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return v, err
}

func (s *SQLiteStore) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ── Jobs ──────────────────────────────────────────────────────────────────

func (s *SQLiteStore) CreateJob(j Job) error {
	if j.ID == "" {
		return errors.New("store: job sans id")
	}
	if j.MaxAttempts == 0 {
		j.MaxAttempts = 3
	}
	_, err := s.db.Exec(`
		INSERT INTO jobs(id, type, payload, status, priority, attempts, max_attempts, error)
		VALUES(?,?,?,?,?,?,?,?)`,
		j.ID, j.Type, j.Payload, j.Status, j.Priority, j.Attempts, j.MaxAttempts, nullStr(j.Error))
	if err != nil {
		return fmt.Errorf("store: création du job %s: %w", j.ID, err)
	}
	return nil
}

func (s *SQLiteStore) ListJobs(status string, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `
		SELECT id, type, payload, status, priority, attempts, max_attempts,
		       COALESCE(error, ''), created_at, started_at, finished_at
		  FROM jobs`
	var args []any
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY priority DESC, created_at LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: liste des jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var createdAt string
		var startedAt, finishedAt sql.NullString
		if err := rows.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Priority,
			&j.Attempts, &j.MaxAttempts, &j.Error, &createdAt, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		j.CreatedAt = parseTime(createdAt)
		if startedAt.Valid {
			t := parseTime(startedAt.String)
			j.StartedAt = &t
		}
		if finishedAt.Valid {
			t := parseTime(finishedAt.String)
			j.FinishedAt = &t
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (s *SQLiteStore) UpdateJobStatus(id, status, errMsg string) error {
	res, err := s.db.Exec(`
		UPDATE jobs SET
		  status = ?,
		  error = CASE WHEN ? = '' THEN error ELSE ? END,
		  started_at = CASE WHEN ? = 'running' THEN COALESCE(started_at, CURRENT_TIMESTAMP) ELSE started_at END,
		  finished_at = CASE WHEN ? IN ('done', 'failed') THEN CURRENT_TIMESTAMP ELSE NULL END
		 WHERE id = ?`,
		status, errMsg, errMsg, status, status, id)
	if err != nil {
		return fmt.Errorf("store: maj du job %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	slog.Debug("job mis à jour", "id", id, "status", status)
	return nil
}
