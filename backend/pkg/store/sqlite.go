package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
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

// ── Playlists (M3 part 2) ────────────────────────────────────────────────

// parsePlaylistID convertit « pl_<n> » (forme API) en id entier de la base.
func parsePlaylistID(id string) (int64, error) {
	if !strings.HasPrefix(id, "pl_") {
		return 0, ErrNotFound
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(id, "pl_"), 10, 64)
	if err != nil || n <= 0 {
		return 0, ErrNotFound
	}
	return n, nil
}

func fmtPlaylistID(n int64) string {
	return "pl_" + strconv.FormatInt(n, 10)
}

func (s *SQLiteStore) ListPlaylists() ([]PlaylistSummary, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, COUNT(pt.track_id), p.created_at, p.updated_at
		  FROM playlists p
		  LEFT JOIN playlist_tracks pt ON pt.playlist_id = p.id
		 GROUP BY p.id
		 ORDER BY p.updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: liste des playlists: %w", err)
	}
	defer rows.Close()

	out := make([]PlaylistSummary, 0, 8)
	for rows.Next() {
		var s PlaylistSummary
		var id int64
		var created, updated string
		if err := rows.Scan(&id, &s.Name, &s.TrackCount, &created, &updated); err != nil {
			return nil, err
		}
		s.ID = fmtPlaylistID(id)
		s.CreatedAt, s.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreatePlaylist(name string) (Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Playlist"
	}
	res, err := s.db.Exec(`INSERT INTO playlists(name) VALUES(?)`, name)
	if err != nil {
		return Playlist{}, fmt.Errorf("store: création de la playlist: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Playlist{}, err
	}
	return Playlist{
		ID:        fmtPlaylistID(id),
		Name:      name,
		Tracks:    []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (s *SQLiteStore) GetPlaylist(id string) (Playlist, error) {
	pid, err := parsePlaylistID(id)
	if err != nil {
		return Playlist{}, ErrNotFound
	}
	var p Playlist
	var rowID int64
	var created, updated string
	err = s.db.QueryRow(`SELECT id, name, created_at, updated_at FROM playlists WHERE id = ?`, pid).
		Scan(&rowID, &p.Name, &created, &updated)
	if err == sql.ErrNoRows {
		return Playlist{}, ErrNotFound
	}
	if err != nil {
		return Playlist{}, err
	}
	p.ID = fmtPlaylistID(rowID)
	p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
	p.Tracks = []string{}

	rows, err := s.db.Query(`
		SELECT t.path FROM playlist_tracks pt JOIN tracks t ON t.id = pt.track_id
		 WHERE pt.playlist_id = ? ORDER BY pt.position`, pid)
	if err != nil {
		return Playlist{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return Playlist{}, err
		}
		p.Tracks = append(p.Tracks, path)
	}
	return p, rows.Err()
}

func (s *SQLiteStore) DeletePlaylist(id string) error {
	pid, err := parsePlaylistID(id)
	if err != nil {
		return ErrNotFound
	}
	res, err := s.db.Exec(`DELETE FROM playlists WHERE id = ?`, pid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil // playlist_tracks supprimées par ON DELETE CASCADE
}

// AddPlaylistTrack ajoute un morceau en fin de playlist (dédoublonné : la PK
// playlist_id+track_id l'interdit par construction). Retourne la playlist
// à jour.
func (s *SQLiteStore) AddPlaylistTrack(id, path string) (Playlist, error) {
	pid, err := parsePlaylistID(id)
	if err != nil {
		return Playlist{}, ErrNotFound
	}
	trackID, err := s.ensureTrack(s.db, path)
	if err != nil {
		return Playlist{}, err
	}
	_, err = s.db.Exec(`
		INSERT OR IGNORE INTO playlist_tracks(playlist_id, track_id, position)
		VALUES(?, ?, (SELECT COALESCE(MAX(position) + 1, 0) FROM playlist_tracks WHERE playlist_id = ?))`,
		pid, trackID, pid)
	if err != nil {
		return Playlist{}, fmt.Errorf("store: ajout à la playlist %s: %w", id, err)
	}
	return s.GetPlaylist(id)
}

func (s *SQLiteStore) RemovePlaylistTrack(id, path string) (Playlist, error) {
	pid, err := parsePlaylistID(id)
	if err != nil {
		return Playlist{}, ErrNotFound
	}
	_, err = s.db.Exec(`
		DELETE FROM playlist_tracks
		 WHERE playlist_id = ? AND track_id = (SELECT id FROM tracks WHERE path = ?)`, pid, path)
	if err != nil {
		return Playlist{}, err
	}
	return s.GetPlaylist(id)
}

// ReorderPlaylist remplace l'ordre de la playlist par la liste fournie
// (drag-and-drop). Même sémantique que l'ancien store JSON : chemins
// inconnus ignorés, doublons écartés, morceaux non mentionnés conservés en
// fin (ordre d'origine).
func (s *SQLiteStore) ReorderPlaylist(id string, paths []string) (Playlist, error) {
	pid, err := parsePlaylistID(id)
	if err != nil {
		return Playlist{}, ErrNotFound
	}
	current, err := s.GetPlaylist(id)
	if err != nil {
		return Playlist{}, err
	}
	known := make(map[string]bool, len(current.Tracks))
	for _, t := range current.Tracks {
		known[t] = true
	}
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, t := range paths {
		if !known[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	for _, t := range current.Tracks {
		if !seen[t] {
			out = append(out, t)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Playlist{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, pid); err != nil {
		return Playlist{}, err
	}
	for i, path := range out {
		trackID, err := s.ensureTrack(tx, path)
		if err != nil {
			return Playlist{}, err
		}
		if _, err := tx.Exec(`INSERT INTO playlist_tracks(playlist_id, track_id, position) VALUES(?,?,?)`, pid, trackID, i); err != nil {
			return Playlist{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE playlists SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, pid); err != nil {
		return Playlist{}, err
	}
	if err := tx.Commit(); err != nil {
		return Playlist{}, err
	}
	return s.GetPlaylist(id)
}

// ── Pins (M3) ─────────────────────────────────────────────────────────────

var validPinKinds = map[string]bool{"artist": true, "album": true, "playlist": true}

func (s *SQLiteStore) ListPins() ([]Pin, error) {
	rows, err := s.db.Query(`SELECT kind, value, pinned_at FROM pins ORDER BY pinned_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: liste des épingles: %w", err)
	}
	defer rows.Close()

	pins := make([]Pin, 0, 8)
	for rows.Next() {
		var p Pin
		var pinnedAt string
		if err := rows.Scan(&p.Kind, &p.Value, &pinnedAt); err != nil {
			return nil, err
		}
		p.PinnedAt = parseTime(pinnedAt)
		pins = append(pins, p)
	}
	return pins, rows.Err()
}

func (s *SQLiteStore) AddPin(kind, value string) error {
	if !validPinKinds[kind] {
		return fmt.Errorf("kind invalide : %q (attendu artist/album/playlist)", kind)
	}
	if value == "" {
		return errors.New("value requise")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO pins(kind, value) VALUES(?, ?)`, kind, value)
	return err
}

func (s *SQLiteStore) RemovePin(kind, value string) error {
	_, err := s.db.Exec(`DELETE FROM pins WHERE kind = ? AND value = ?`, kind, value)
	return err
}

// ── Player queue (M3) ─────────────────────────────────────────────────────

func (s *SQLiteStore) GetPlayerQueue() (PlayerQueue, error) {
	raw, err := s.GetSetting("player_queue")
	if err != nil {
		if err == ErrNotFound {
			return PlayerQueue{Queue: []string{}}, nil
		}
		return PlayerQueue{}, err
	}
	var q PlayerQueue
	if err := json.Unmarshal([]byte(raw), &q); err != nil || q.Queue == nil {
		// Donnée corrompue ou absente → file vide plutôt qu'une erreur.
		return PlayerQueue{Queue: []string{}}, nil
	}
	return q, nil
}

func (s *SQLiteStore) SetPlayerQueue(q PlayerQueue) error {
	if q.Queue == nil {
		q.Queue = []string{}
	}
	if q.Index < 0 || q.Index >= len(q.Queue) {
		q.Index = 0
	}
	data, err := json.Marshal(q)
	if err != nil {
		return fmt.Errorf("store: sérialisation de la file: %w", err)
	}
	return s.SetSetting("player_queue", string(data))
}

// ── Likes (M3) ────────────────────────────────────────────────────────────

// rowQuerier est satisfaite par *sql.DB et *sql.Tx : ensureTrack peut
// s'exécuter dans une transaction (sinon, avec MaxOpenConns(1), la requête
// sur le pool bloquerait pendant qu'une tx tient la seule connexion).
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// ensureTrack garantit qu'une ligne tracks existe pour un chemin (fichier
// apparu après le dernier scan) : le prochain rescan l'enrichit. Ligne
// minimale : path + titre dérivé du nom de fichier.
func (s *SQLiteStore) ensureTrack(q rowQuerier, path string) (int64, error) {
	var id int64
	err := q.QueryRow(`SELECT id FROM tracks WHERE path = ?`, path).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	title := path
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		title = path[i+1:]
	}
	res, err := q.Exec(`INSERT INTO tracks(path, title) VALUES(?, ?)`, path, title)
	if err != nil {
		return 0, fmt.Errorf("store: création du morceau %q: %w", path, err)
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, err
	}
	// Index FTS5 (contentless : insertion manuelle) pour rester cohérent.
	_, err = q.Exec(`INSERT INTO tracks_fts(rowid, title, artist, album) VALUES(?,?,?,?)`, id, title, "", "")
	return id, err
}

func (s *SQLiteStore) LikeTrack(path string) error {
	id, err := s.ensureTrack(s.db, path)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO likes(track_id) VALUES(?)`, id)
	return err
}

func (s *SQLiteStore) UnlikeTrack(path string) error {
	_, err := s.db.Exec(`DELETE FROM likes WHERE track_id = (SELECT id FROM tracks WHERE path = ?)`, path)
	return err
}

func (s *SQLiteStore) ListLikedPaths() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT t.path FROM likes l JOIN tracks t ON t.id = l.track_id
		 ORDER BY l.liked_at DESC, t.path`)
	if err != nil {
		return nil, fmt.Errorf("store: liste des likes: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ── History (M3) ──────────────────────────────────────────────────────────

func (s *SQLiteStore) AddPlay(path string, durationSec int) error {
	id, err := s.ensureTrack(s.db, path)
	if err != nil {
		return err
	}
	ms := durationSec * 1000
	if durationSec <= 0 {
		ms = 0
	}
	// Écoute back-to-back du même morceau : on rafraîchit la dernière ligne
	// au lieu d'en insérer une nouvelle (même comportement que l'ancien
	// store JSON, pour ne pas polluer l'historique).
	var lastTrack int64
	err = s.db.QueryRow(`SELECT track_id FROM history ORDER BY played_at DESC, id DESC LIMIT 1`).Scan(&lastTrack)
	if err == nil && lastTrack == id {
		_, err = s.db.Exec(`
			UPDATE history SET played_at = CURRENT_TIMESTAMP, ms_played = ?
			 WHERE id = (SELECT id FROM history ORDER BY played_at DESC, id DESC LIMIT 1)`, ms)
		return err
	}
	_, err = s.db.Exec(`INSERT INTO history(track_id, ms_played) VALUES(?, ?)`, id, ms)
	if err != nil {
		return fmt.Errorf("store: enregistrement de l'écoute %q: %w", path, err)
	}
	return nil
}

func (s *SQLiteStore) RecentPlays(limit int) ([]PlayRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT t.path, h.played_at, h.ms_played
		  FROM history h JOIN tracks t ON t.id = h.track_id
		 ORDER BY h.played_at DESC, h.id DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: historique récent: %w", err)
	}
	defer rows.Close()

	recs := make([]PlayRecord, 0, limit)
	for rows.Next() {
		var r PlayRecord
		var playedAt string
		if err := rows.Scan(&r.Path, &playedAt, &r.Duration); err != nil {
			return nil, err
		}
		r.Duration /= 1000 // ms_played → secondes (forme API historique)
		r.PlayedAt = parseTime(playedAt)
		recs = append(recs, r)
	}
	return recs, rows.Err()
}

func (s *SQLiteStore) MostPlayed(limit int) ([]Count, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT t.path, COUNT(*) AS plays
		  FROM history h JOIN tracks t ON t.id = h.track_id
		 GROUP BY t.path ORDER BY plays DESC, t.path LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: top morceaux: %w", err)
	}
	defer rows.Close()

	out := make([]Count, 0, limit)
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Path, &c.Plays); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) TotalPlays() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM history`).Scan(&n)
	return n, err
}

// Stats reproduit l'agrégation de l'ancien store JSON : artiste = premier
// segment du chemin relatif, fenêtre de 14 jours pour le graphique.
func (s *SQLiteStore) Stats() (Stats, error) {
	rows, err := s.db.Query(`
		SELECT t.path, h.ms_played, h.played_at
		  FROM history h JOIN tracks t ON t.id = h.track_id`)
	if err != nil {
		return Stats{}, fmt.Errorf("store: stats: %w", err)
	}
	defer rows.Close()

	st := Stats{}
	artists := map[string]int{}
	days := map[string]int{}
	now := time.Now()

	for rows.Next() {
		var path string
		var ms int
		var playedAt string
		if err := rows.Scan(&path, &ms, &playedAt); err != nil {
			return Stats{}, err
		}
		st.TotalPlays++
		st.TotalSeconds += ms / 1000

		// artiste = premier segment du layout {artiste}/{album}/{titre}.
		if i := strings.IndexAny(path, `/\`); i > 0 {
			artists[path[:i]]++
		}

		t := parseTime(playedAt)
		day := t.Format("2006-01-02")
		if diff := now.Sub(t); diff >= 0 && diff <= 14*24*time.Hour {
			days[day]++
		}
	}
	if err := rows.Err(); err != nil {
		return Stats{}, err
	}

	for a, c := range artists {
		st.TopArtists = append(st.TopArtists, ArtistCount{Artist: a, Plays: c})
	}
	sort.Slice(st.TopArtists, func(i, j int) bool {
		if st.TopArtists[i].Plays == st.TopArtists[j].Plays {
			return st.TopArtists[i].Artist < st.TopArtists[j].Artist
		}
		return st.TopArtists[i].Plays > st.TopArtists[j].Plays
	})
	if len(st.TopArtists) > 10 {
		st.TopArtists = st.TopArtists[:10]
	}

	st.TopTracks, err = s.MostPlayed(10)
	if err != nil {
		return Stats{}, err
	}

	// 14 jours pleins pour un graphique continu (0 les jours calmes).
	for i := 13; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		st.PlaysByDay = append(st.PlaysByDay, DayCount{Day: d, Plays: days[d]})
	}
	return st, nil
}

// RenameTrack déplace un chemin de morceau. Les likes/history/playlists de
// la base référencent le track_id : rien d'autre à migrer — c'est l'intérêt
// des FK par rapport aux stores JSON (migrateStats du handler devient un
// simple UPDATE).
func (s *SQLiteStore) RenameTrack(oldPath, newPath string) error {
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return nil
	}
	_, err := s.db.Exec(`UPDATE tracks SET path = ? WHERE path = ?`, newPath, oldPath)
	if err != nil {
		return fmt.Errorf("store: déplacement %q → %q: %w", oldPath, newPath, err)
	}
	return nil
}
