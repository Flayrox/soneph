-- migrations/0001_init.sql
-- Les PRAGMA sont wrappés en blocs StatementBegin/End : le parseur goose
-- exécute chaque bloc verbatim (il rejette un PRAGMA nu en début de fichier).
-- Le DSN du store force aussi journal_mode(WAL) et foreign_keys(ON) à
-- l'ouverture, indépendamment de cette migration.
-- +goose Up
-- +goose StatementBegin
PRAGMA journal_mode = WAL;
-- +goose StatementEnd
-- +goose StatementBegin
PRAGMA foreign_keys = ON;
-- +goose StatementEnd

CREATE TABLE artists (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  mbid TEXT,
  sort_name TEXT,
  UNIQUE(name)
);

CREATE TABLE albums (
  id INTEGER PRIMARY KEY,
  artist_id INTEGER NOT NULL REFERENCES artists(id),
  title TEXT NOT NULL,
  mbid TEXT,
  year INTEGER,
  cover_path TEXT,
  UNIQUE(artist_id, title)
);

CREATE TABLE tracks (
  id INTEGER PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,          -- relative to DOWNLOAD_DIR
  title TEXT NOT NULL,
  artist_id INTEGER REFERENCES artists(id),
  album_id INTEGER REFERENCES albums(id),
  track_no INTEGER,
  duration_ms INTEGER,
  bitrate INTEGER,
  format TEXT,
  size_bytes INTEGER,
  isrc TEXT,
  fingerprint TEXT,                   -- chromaprint (M6)
  acoustid TEXT,
  lyrics_path TEXT,
  lyrics_synced INTEGER DEFAULT 0,
  quality_score INTEGER DEFAULT 0,
  added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE playlists (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  source_url TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE playlist_tracks (
  playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
  track_id INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  PRIMARY KEY (playlist_id, track_id)
);

CREATE TABLE likes (
  track_id INTEGER PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
  liked_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE history (
  id INTEGER PRIMARY KEY,
  track_id INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
  played_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  ms_played INTEGER DEFAULT 0
);

CREATE TABLE jobs (
  id TEXT PRIMARY KEY,                -- uuid
  type TEXT NOT NULL,                 -- download/lyrics/rescan/fingerprint/…
  payload TEXT NOT NULL,              -- JSON
  status TEXT NOT NULL DEFAULT 'queued',  -- queued/running/done/failed
  priority INTEGER DEFAULT 0,
  attempts INTEGER DEFAULT 0,
  max_attempts INTEGER DEFAULT 3,
  error TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  started_at DATETIME,
  finished_at DATETIME
);

CREATE TABLE pins (
  kind TEXT NOT NULL,                 -- artist/album/playlist
  ref_id INTEGER NOT NULL,
  pinned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (kind, ref_id)
);

CREATE VIRTUAL TABLE tracks_fts USING fts5(title, artist, album, content='');
CREATE INDEX idx_tracks_isrc ON tracks(isrc);
CREATE INDEX idx_tracks_fingerprint ON tracks(fingerprint);
CREATE INDEX idx_history_played ON history(played_at);

-- +goose Down
-- Schéma initial (M2) : la base est la source de vérité, pas de rollback
-- prévu. Toute évolution passe par une nouvelle migration.

