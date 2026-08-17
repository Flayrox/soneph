# ADR-0001 — SQLite comme source de vérité

- **Statut** : accepté
- **Date** : 2026-08-17
- **Portée** : roadmap M2 → M15

## Contexte

soneph stocke aujourd'hui tout son état hors base : scan du système de
fichiers + fichiers JSON (playlists, historique, réglages, `queue.json`) +
`localStorage` côté front. Conséquences :

- pas de requêtes structurées (recherche, stats, smart playlists) ;
- aucune transaction — les écritures partielles corrompent l'état ;
- une seule instance, pas de reprise après crash (`kill -9` en plein
  téléchargement perd la file) ;
- le front est le seul dépôt de certaines données (pins, likes, file
  d'attente), impossible d'ouvrir l'app depuis un second navigateur.

Un serveur de base de données externe (Postgres standalone, MySQL…) est
hors de question pour un produit self-hosted mono-instance : installation
supplémentaire, mémoire, complexité d'exploitation.

## Décision

SQLite est la source de vérité unique de soneph. Implémentation :
`modernc.org/sqlite` (driver pur Go, **zéro CGO**), pour conserver le
binaire statique unique. Le schéma vit dans `migrations/NNNN_name.sql`,
appliqué par `goose` au démarrage. Toutes les lectures/écritures passent
par `pkg/store`, une interface `Store` — le mode Postgres-compatible plus
tard est un remplacement d'implémentation, pas un changement de conception.

- Les fichiers (audio, pochettes, paroles) restent sur disque ; la base
  indexe leurs chemins relatifs à `DOWNLOAD_DIR`.
- Les données existantes (JSON, localStorage) sont migrées au premier
  démarrage, puis les fichiers sources sont supprimés du dépôt.
- `PRAGMA journal_mode = WAL` : lectures concurrentes non bloquantes, ce
  qui tient à 100k+ morceaux.
- Recherche via FTS5 (`tracks_fts`), indexée en arrière-plan.

## Conséquences

**Positives** : requêtes structurées (stats, smart playlists), transactions
fiables, reprise après crash (file de jobs en base), multi-navigateur, base
solide pour le streaming et la compatibilité Subsonic.

**Négatives** : une dépendance de plus ; les migrations exigent de la
rigueur (jamais d'édition d'une migration appliquée, uniquement de
nouvelles).

**Risques** : l'écriture de SQL hors de `pkg/store` est interdite — sinon
le remplacement Postgres devient impossible. `modernc.org/sqlite` est plus
lent que CGO sur les scans massifs ; acceptable à cette échelle (WAL + FTS).
