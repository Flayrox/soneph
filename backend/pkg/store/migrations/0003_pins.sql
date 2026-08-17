-- migrations/0003_pins.sql
-- Les pins du front référencent des VALEURS (nom d'artiste, nom d'album,
-- id de playlist) et non des id entiers : les playlists ne sont pas encore
-- en base (M3 part 2). La table pins du schéma §4 (kind, ref_id INTEGER)
-- est remplacée par une version à référence texte — jamais d'édition de
-- 0001, uniquement une nouvelle migration.
-- +goose Up
DROP TABLE pins;
CREATE TABLE pins (
  kind TEXT NOT NULL,               -- artist/album/playlist
  value TEXT NOT NULL,
  pinned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (kind, value)
);

-- +goose Down
DROP TABLE pins;
CREATE TABLE pins (
  kind TEXT NOT NULL,
  ref_id INTEGER NOT NULL,
  pinned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (kind, ref_id)
);
