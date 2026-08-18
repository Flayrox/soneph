-- migrations/0002_settings.sql
-- Réglages d'exécution (workers, threads, dossier d'export…) en base, à la
-- place du fichier settings.json. Ajouté en M2 car le schéma de référence
-- (§4) ne contient pas de table settings : toute évolution passe par une
-- nouvelle migration, jamais par l'édition d'une migration appliquée.
-- +goose Up
CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- +goose Down
DROP TABLE settings;
