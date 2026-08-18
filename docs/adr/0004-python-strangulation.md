# ADR 0004 — Strangulation du pipeline Python (M5/M6)

- Statut : accepté (M5 + M6 parts 1 et 2 appliqués)
- Date : 2026-08-17 (mise à jour 2026-08-18)

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
| `tag_soneph.py` | `pkg/tags.StampSoneph` (écrivain ID3 : TXXX splice + atomic) | part 2 — supprimé |
| `lyrics_retry.py` / `embed_lyrics.py` | — (fournisseur de paroles + écriture USLT/SYLT) | part 3 |
| `patch_lyrics_timeout.py` | — (patch spotdl au build Docker, sans lien runtime) | part 3 |

### Ce qui reste (part 3) et pourquoi

Il reste les scripts de **paroles** : un fournisseur de paroles synchronisées
(`lyrics_retry.py`) et l'écriture USLT/SYLT (`embed_lyrics.py`), plus
`precreate_dirs.py` (reproduit les chemins de dossier de spotdl) et le patch
du build Docker. Le moteur de téléchargement reste spotdl (Python) : la
strangulation complète du Python suivra le port du moteur lui-même.

## Écrivain ID3 (M6 part 2)

`pkg/tags/write.go` ajoute la brique d'écriture manquante (dhowden/tag ne
sait que lire) : `StampSoneph(dir, url)` est le port fidèle de
`tag_soneph.py` — TXXX:SONEPH, TXXX:SONEPH_SOURCE (ajoutés seulement si
absents, idempotent), TXXX:SONEPH_QUALITY (débit **moyen** réel, parité
mutagen `MP3.info.bitrate`, toujours rafraîchi si différent).

Choix de conception :

- **Splice octet pour octet** : les frames existantes sont préservées à
  l'identique (aucune frame inconnue perdue), seule la liste des frames
  TXXX gérées est modifiée ; la version du tag existant est conservée
  (ID3v2.3 fraîche si le fichier n'en avait pas).
- **Idempotence** : aucun octet écrit quand rien ne change (2e passage = 0).
- **Écriture atomique** : temp dans le même dossier + rename, permissions
  préservées — un crash ne laisse jamais un MP3 à moitié réécrit.
- **Robustesse** : tags v2.2/v2.3/v2.4, extended header, unsynchronisation,
  descriptions encodées en latin-1/UTF-16/UTF-8 — détectées quel que soit
  l'outil qui les a écrites (mutagen, spotdl…).

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
