<div align="center">

# 🎵 Soneph

**Ta bibliothèque Spotify, en MP3 320 kbps — avec métadonnées complètes, paroles synchronisées et import automatique dans l'app Musique.**

![Soneph](Sonephe.png)

</div>

Soneph est une app **macOS native** (Electron) + un **backend Go** qui télécharge tes playlists / albums / singles Spotify en MP3 de haute qualité, range tout proprement (`Artiste/Album/Titre.mp3`), écrit **toutes les métadonnées ID3** (pochette, piste, genre, producteurs, auteurs…), récupère les **paroles synchronisées**, et peut **importer automatiquement** chaque morceau dans l'app Musique d'Apple.

---

## ✨ Fonctionnalités

- **Téléchargement Spotify** (spotdl) en **128 / 192 / 320 kbps**, playlists, albums, artistes ou tracks uniques
- **Métadonnées ID3 complètes** : pochette, album, artiste album, année, genre, piste, disque, auteurs, **producteurs & musiciens** (TIPL/TMCL), ISRC, copyright
- **Single → Album sans re-téléchargement** : quand tu télécharges l'album d'un single déjà présent, le fichier est *déplacé* vers le bon dossier et ses tags sont réécrits — l'audio n'est jamais re-téléchargé
- **Paroles synchronisées** (`.lrc` + tags ID3) via lrclib / netease / musixmatch…, avec **source enregistrée dans les tags** (`LYRICS_SOURCE`) pour ne re-télécharger que ce qui peut être amélioré (texte brut → version synchronisée)
- **Création automatique de playlist** : coller un lien de playlist Spotify télécharge les sons manquants **et** crée la playlist — les morceaux déjà sur disque y sont ajoutés immédiatement, sans re-téléchargement
- **Stats qui suivent tes fichiers** : historique d'écoutes, likes et playlists sont ré-attachés automatiquement quand un fichier bouge (single → album), quand tu supprimes un doublon, ou quand tu supprimes un fichier dont une autre copie existe
- **Panneau « Plus de détails »** (clic droit) : qualité réelle, source des paroles, artistes, producteurs, lien Spotify
- **Auto-import dans l'app Musique** (macOS) : chaque nouveau fichier est copié dans « Automatically Add to Music » avec ses paroles — zéro doublon
- **Playlists** : création, ajout/retrait, réordonnancement, export `.m3u8`
- **UI native macOS** : dark mode, glassmorphism, queue de téléchargement en temps réel (WebSocket), bibliothèque, artistes, albums, favoris, top écoutes

---

## 📥 Installation (macOS)

> L'app cible **macOS (Apple Silicon / Intel)** avec l'app **Musique** installée. Python et `spotdl` sont requis pour le téléchargement (voir [Dépendances](#-dépendances)).

1. Télécharge le **DMG** depuis la page [Releases](https://github.com/Flayrox/soneph/releases)
2. Ouvre le DMG et glisse **Soneph** dans *Applications*
3. Première ouverture : clic droit → *Ouvrir* si Gatekeeper proteste (app non notarisée)

### Dépendances

Le téléchargement repose sur `spotdl` (Python). Installe-le une fois :

```bash
brew install python
pipx install spotdl          # ou : pip install spotdl
```

Soneph utilise l'interpréteur Python de `spotdl` lui-même pour ses scripts (tags ID3, paroles) — rien d'autre à installer.

---

## 🚀 Utilisation rapide

1. **Ouvre Soneph** — ta bibliothèque se trouve dans `~/Music/soneph` (les morceaux déjà présents y apparaissent automatiquement)
2. **Colle un lien Spotify** dans la barre du haut :
   - *Playlist* → les sons manquants sont téléchargés **et** la playlist est créée en même temps (les sons déjà là sont ajoutés direct)
   - *Album / Track / Artiste* → téléchargement simple
3. **Sync & Réglages → Start** pour activer l'auto-import : chaque morceau arrive dans l'app **Musique** avec ses paroles
4. Clic droit sur un morceau → **ℹ️ Plus de détails** (qualité, source des paroles, producteurs…), **ajouter à une playlist**, etc.

### Transférer sur iPhone

- **Câble** : Finder → iPhone → Musique → « Synchroniser la musique » (gratuit)
- **Sans câble** : « Synchroniser la bibliothèque » (iCloud, nécessite Apple Music / iTunes Match)

---

## 💻 Développement local

### Prérequis

| Outil | Version |
|---|---|
| Go | 1.22+ |
| Node.js | 18+ |
| Python | 3.11+ (`spotdl`, voir ci-dessus) |
| fswatch (optionnel) | `brew install fswatch` — watcher instantané au lieu du polling |

### Lancer en dev (backend + frontend)

```bash
make dev        # backend Go sur :8080 + dev server Vite sur :5173
```

- API + UI : `http://localhost:8080`
- Dev server Vite : `http://localhost:5173` (hot reload)

### Tester / vérifier

```bash
make test       # tests Go
make vet        # go vet
make build      # frontend embarqué + binaire Go dans backend/bin/
```

---

## 🖥️ Build de l'app macOS

```bash
make desktop    # frontend → Go → icône → .app Electron
```

Résultat : `desktop/dist/Soneph-darwin-arm64/Soneph.app`. Pour générer le **DMG** d'installation :

```bash
desktop/make-dmg.sh         # → desktop/dist/Soneph-<version>.dmg
```

---

## 🐳 Déploiement serveur (Docker)

Soneph fonctionne aussi en **serveur headless** (VPS) : l'UI est embarquée dans le binaire, la distribution vers tes appareils se fait alors via **Syncthing**.

```bash
docker compose up -d --build
```

| Variable | Défaut | Description |
|---|---|---|
| `PORT` | `8080` | Port HTTP |
| `DOWNLOAD_DIR` | `./downloads` | Dossier de la bibliothèque |
| `SONEPH_TOKEN` | *(vide)* | Si défini, protège l'API par Bearer token |
| `SONEPH_ENGINE` | `spotdl` | Binaire du moteur de téléchargement |
| `SONEPH_THREADS` | `6` | Téléchargements parallèles |

Docker monte `./downloads` en volume : tes fichiers survivent aux redémarrages et peuvent être partagés avec Syncthing.

---

## 🗂️ Structure du projet

```
├── backend/            # Serveur Go (gin) + scripts Python
│   ├── pkg/
│   │   ├── downloader/ # moteur spotdl (file, stats, métadonnées)
│   │   ├── handler/    # API REST + WebSocket
│   │   ├── history/    # écoutes, likes
│   │   ├── playlists/  # playlists JSON
│   │   ├── storage/    # scan de la bibliothèque, doublons
│   │   └── syncmgr/    # watcher d'auto-import Musique
│   ├── *.py            # helpers : tags ID3, paroles, identité, playlist…
│   └── web/dist/       # frontend compilé (embarqué dans le binaire)
├── frontend/           # UI React + Vite + TypeScript
├── desktop/            # app Electron macOS + scripts de build
├── scripts/            # dev.sh, watch_and_import.sh…
└── downloads/          # ta bibliothèque (jamais commitée)
```

### Scripts Python (backend/)

| Script | Rôle |
|---|---|
| `fast_filter.py` | détecte instantanément les morceaux déjà sur disque |
| `precreate_dirs.py` | pré-crée les dossiers d'album (single → album) |
| `tag_soneph.py` | marqueur `TXXX:SONEPH` + source + qualité réelle |
| `lyrics_retry.py` | paroles synchronisées + source enregistrée |
| `embed_lyrics.py` | paroles dans les tags ID3 (USLT/SYLT) |
| `scan_identity.py` | carte URL Spotify (WOAS) → chemins |
| `playlist_from_url.py` | résolution playlist → morceaux présents/manquants |
| `file_details.py` | dump complet des métadonnées ID3 |

---

## 📦 Release (open source)

Tout est prêt pour publier sur GitHub :

```bash
# 1. Versionner
git tag v1.0.0 && git push origin v1.0.0

# 2. Build complet + DMG + Release GitHub (nécessite gh : brew install gh)
./scripts/release.sh v1.0.0
```

`release.sh` fait : build frontend + Go + app Electron, génère le **DMG**, crée la **GitHub Release** avec le DMG en asset (notes générées à partir du changelog). Sans `gh`, il produit juste le DMG — tu le déposes à la main sur la page Release.

> ⚠️ Le DMG est un asset de Release GitHub, **pas** un fichier du dépôt git (265+ Mo).

---

## 🧪 Tests

```bash
make test       # go test ./... (backend)
cd frontend && npm run typecheck
```

---

## 📜 Licence

MIT — voir [LICENSE](LICENSE). *Non affilié à Spotify. Pour un usage personnel de musique dont tu détiens les droits.*

---

*Documentation historique (architecture détaillée de l'ancien backend) : [README_legacy.md](README_legacy.md).*
