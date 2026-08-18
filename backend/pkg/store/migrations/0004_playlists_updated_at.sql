-- migrations/0004_playlists_updated_at.sql
-- La table playlists du schéma §4 n'a pas de updated_at, mais l'API et le
-- front l'exposent (hérité du store JSON) : nouvelle migration, jamais
-- d'édition de 0001.
-- +goose Up
ALTER TABLE playlists ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP;
UPDATE playlists SET updated_at = created_at;

-- +goose Down
ALTER TABLE playlists DROP COLUMN updated_at;
