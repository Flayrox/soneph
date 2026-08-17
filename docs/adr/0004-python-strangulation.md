# ADR 0004 — Strangulation du pipeline Python (M5/M6)

- Statut : accepté (M5 + M6 part 1 appliqués)
- Date : 2026-08-17

## Contexte

Le backend héritait de neuf scripts Python (`backend/*.py`) lancés en
sous-processus depuis les handlers et le moteur de téléchargement. Chaque
appel impliquait :

- de trouver l'interpréteur (spécialement celui de spotdl, pour ses
  dépendances mutagen / syncedlyrics) — `GetPythonExec()` ;
- de localiser le script dans quatre emplacements possibles —
  `GetScriptPath()` ;
- un `exec.Command` avec parsing de la sortie JSON ;
- l'échec silencieux quand l'app était lancée depuis le Finder (PATH minimal)
  ou que le Python n'avait pas les dépendances.

En parallèle, l'architecture a convergé (M2–M5) vers « SQLite source de
vérité + file jobs persistante » : chaque brique Python était un point de
défaillance externe sans lien avec cette architecture.

## Décision

Porter en Go — avec la même sortie JSON, les mêmes raisons et les mêmes
gardes-fous que le script — puis **supprimer** le script Python. Un port
n'est pas « refactoré » : il est verrouillé par des tests table-driven sur
fixtures locales, puis vérifié en live, puis le `.py` est effacé (ainsi que
sa copie Dockerfile et sa ligne README).

| Script | Port Go | M6 |
|---|---|---|
| `fast_filter.py` | `pkg/fastfilter` (O(N), MAX_PAGES, pages répétées) | M5 — supprimé |
| `scan_identity.py` | `pkg/tags.IdentityMap` (WOAS → chemins) | part 1 — supprimé |
| `file_details.py` | `pkg/tags.FileDetails` (mêmes clés JSON) | part 1 — supprimé |
| `extract_cover.py` | `pkg/tags.Cover` (APIC) | part 1 — supprimé |
| `playlist_from_url.py` | `pkg/fastfilter.ResolvePlaylist` (par identité WOAS) | part 1 — supprimé |
| `precreate_dirs.py` | — (reproduit les chemins de spotdl) | part 2 |
| `tag_soneph.py` | — (écrit des frames TXXX) | part 2 |
| `lyrics_retry.py` / `embed_lyrics.py` | — (fournisseur de paroles + écriture USLT/SYLT) | part 2 |
| `patch_lyrics_timeout.py` | — (patch spotdl au build Docker, sans lien runtime) | part 2 |

### Ce qui reste (part 2) et pourquoi

Les quatre derniers scripts exigent une brique que dhowden/tag ne fournit
pas : un **écrivain ID3** (frames TXXX, USLT, SYLT) et un **fournisseur de
paroles synchronisées**. Le moteur de téléchargement reste spotdl (Python) :
la strangulation complète du Python suivra le port du moteur lui-même.

## Conséquences

- Plus aucun `exec.Command` Python dans les handlers (`tracks.go`,
  `downloads.go`, `playlists.go`) ni dans la carte d'identité du moteur.
- `pkg/tags` (lecture seule, zéro CGO) centralise la lecture ID3 : le parseur
  MPEG (durée/débit) y a été déplacé depuis `pkg/store` pour être partagé.
- Parité vérifiée : les clés JSON de `FileDetails` sont identiques à celles
  de `file_details.py` (le panneau « Plus de détails » du frontend n'a pas
  changé), et `ResolvePlaylist` produit la même correspondance par identité
  (`url:https://open.spotify.com/track/<id>`).
- Les scripts restants (`backend/*.py`) sont listés dans le README avec leur
  rôle, et la copie `backend/*.py` du build desktop continue de fonctionner
  (elle embarque simplement moins de fichiers).
