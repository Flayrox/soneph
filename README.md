<div align="center">

# 🎵 Soneph

**Your Spotify library, as 320 kbps MP3s — with full metadata, synced lyrics and automatic import into the Apple Music app.**

![Soneph](Sonephe.png)

</div>

Soneph is a **native macOS app** (Electron) + a **Go backend** that downloads your Spotify playlists / albums / singles as high-quality MP3s, organizes everything cleanly (`Artist/Album/Title.mp3`), writes **complete ID3 metadata** (cover, track, genre, producers, songwriters…), fetches **synced lyrics**, and can **auto-import** every track into Apple Music.

---

## ✨ Features

- **Spotify downloads** (spotdl) at **128 / 192 / 320 kbps** — playlists, albums, artists or single tracks
- **Full ID3 metadata**: cover, album, album artist, year, genre, track, disc, songwriters, **producers & musicians** (TIPL/TMCL), ISRC, copyright
- **Single → Album without re-downloading**: when you download the album of a track you already have as a single, the file is *moved* to the right folder and its tags rewritten — the audio is never fetched again
- **Synced lyrics** (`.lrc` + ID3 tags) from lrclib / netease / musixmatch…, with the **source stored in the tags** (`LYRICS_SOURCE`) so only what can actually improve is re-fetched (plain text → synced)
- **Automatic playlist creation**: pasting a Spotify playlist link downloads the missing tracks **and** creates the playlist — tracks already on disk are added immediately, with zero re-downloads
- **Stats that follow your files**: play history, likes and playlists are re-attached automatically when a file moves (single → album), when you delete a duplicate, or when you delete a file that has another copy
- **"More details" panel** (right-click): real bitrate, lyrics source, artists, producers, Spotify link
- **Auto-import into Apple Music** (macOS): every new file is copied into "Automatically Add to Music" with its lyrics — no duplicates
- **Playlists**: create, add/remove, reorder, export `.m3u8`
- **Native macOS UI**: dark mode, glassmorphism, real-time download queue (WebSocket), library, artists, albums, favorites, top played

---

## 📥 Installation (macOS)

> The app targets **macOS (Apple Silicon / Intel)** with the **Music** app installed. Python + `spotdl` are required for downloads (see [Dependencies](#-dependencies)).

1. Download the **DMG** from the [Releases](https://github.com/Flayrox/soneph/releases) page
2. Open the DMG and drag **Soneph** into *Applications*
3. First launch: right-click → *Open* if Gatekeeper complains (unsigned app)

### Dependencies

Downloads rely on `spotdl` (Python). Install it once:

```bash
brew install python
pipx install spotdl          # or: pip install spotdl
```

Soneph uses spotdl's own Python interpreter for its helper scripts (ID3 tags, lyrics) — nothing else to install.

### Troubleshooting: `spotdl: executable file not found in $PATH`

This happens when the app is launched from **Finder/Dock**: macOS gives GUI apps a minimal `PATH` (`/usr/bin:/bin:…`), so tools installed with `pipx` (`~/.local/bin`), `pip --user` (`~/Library/Python/3.x/bin`) or Homebrew (`/opt/homebrew/bin`) are invisible to the server.

Soneph now searches these locations automatically (both the Electron launcher and the Go server), so reinstalling is usually enough:

```bash
pipx install spotdl
# ou : pip install spotdl
```

If the error persists, install `spotdl` inside a location on the minimal PATH and relaunch from the Terminal once (`open /Applications/Soneph.app`):

```bash
ln -s "$(which spotdl)" /usr/local/bin/spotdl
```

The same fix covers `ffmpeg` (used by spotdl) and `fswatch` (instant auto-import) — `brew install ffmpeg fswatch`.

---

## 🚀 Quick start

1. **Open Soneph** — your library lives in `~/Music/soneph` by default (tracks already present show up automatically; you can **change the folder** from *Sync & Settings*)
2. **Paste a Spotify link** in the top bar:
   - *Playlist* → the missing tracks are downloaded **and** the playlist is created at the same time (tracks already on disk are added directly)
   - *Album / Track / Artist* → plain download
3. **Sync & Settings → Start** to enable auto-import: every track lands in the **Music** app with its lyrics
4. Right-click a track → **ℹ️ More details** (bitrate, lyrics source, producers…), add to a playlist, etc.

### Getting your music on iPhone

- **Cable**: Finder → iPhone → Music → "Sync music" (free)
- **Wireless**: "Sync Library" (iCloud, requires Apple Music / iTunes Match)

---

## 💻 Local development

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.22+ |
| Node.js | 18+ |
| Python | 3.11+ (with `spotdl`, see above) |
| fswatch (optional) | `brew install fswatch` — instant watcher instead of polling |

### Run in dev (backend + frontend)

```bash
make dev        # Go backend on :8080 + Vite dev server on :5173
```

- API + UI: `http://localhost:8080`
- Vite dev server: `http://localhost:5173` (hot reload)

### Test / check

```bash
make test       # Go tests
make vet        # go vet
make build      # embedded frontend + Go binary in backend/bin/
```

---

## 🖥️ Building the macOS app

```bash
make desktop    # frontend → Go → icon → Electron .app
```

Result: `desktop/dist/Soneph-darwin-arm64/Soneph.app`. To produce the installable **DMG**:

```bash
desktop/make-dmg.sh         # → desktop/dist/Soneph-<version>.dmg
```

### Notarization (optional, removes the Gatekeeper warning)

Requires an Apple Developer account. See `desktop/notarize.sh`:

```bash
APPLE_ID=you@example.com APPLE_APP_PASSWORD=xxxx APPLE_TEAM_ID=XXXXXXXXXX \
  desktop/notarize.sh
```

---

## 🐳 Server deployment (Docker)

Soneph also runs as a **headless server** (VPS): the UI is embedded in the binary, and distribution to your devices is handled via **Syncthing**.

```bash
docker compose up -d --build
```

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `DOWNLOAD_DIR` | `./downloads` | Library folder |
| `SONEPH_TOKEN` | *(empty)* | If set, protects the API with a Bearer token |
| `SONEPH_ENGINE` | `spotdl` | Download engine binary |
| `SONEPH_THREADS` | `6` | Parallel downloads |

Docker mounts `./downloads` as a volume: your files survive restarts and can be shared via Syncthing.

---

## 🗂️ Project structure

```
├── backend/            # Go server (gin) + Python helpers
│   ├── pkg/
│   │   ├── downloader/ # spotdl engine (queue, stats, metadata)
│   │   ├── handler/    # REST API + WebSocket
│   │   ├── history/    # plays, likes
│   │   ├── playlists/  # JSON playlists
│   │   ├── storage/    # library scan, duplicates
│   │   └── syncmgr/    # Apple Music auto-import watcher
│   ├── *.py            # helpers: ID3 tags, lyrics, identity, playlist…
│   └── web/dist/       # compiled frontend (embedded in the Go binary)
├── frontend/           # React + Vite + TypeScript UI
├── desktop/            # Electron macOS app + build scripts
├── scripts/            # dev.sh, watch_and_import.sh…
└── downloads/          # your library (never committed)
```

### Python helpers (backend/)

| Script | Role |
|---|---|
| `precreate_dirs.py` | pre-creates album folders (single → album) |
| `tag_soneph.py` | `TXXX:SONEPH` marker + source + real bitrate |
| `lyrics_retry.py` | synced lyrics + recorded source |
| `embed_lyrics.py` | lyrics into ID3 tags (USLT/SYLT) |
| `scan_identity.py` | Spotify URL (WOAS) → paths map |
| `playlist_from_url.py` | playlist resolution → present/missing tracks |
| `file_details.py` | full ID3 metadata dump |

---

## 📦 Releases

Building and publishing a release is automated:

```bash
# 1. Tag the version
git tag v1.0.0 && git push origin v1.0.0

# 2. Full build + DMG + GitHub Release (requires gh: brew install gh)
./scripts/release.sh v1.0.0
```

`release.sh` builds the frontend + Go + Electron app, produces the **DMG**, and creates the **GitHub Release** with the DMG attached. Without `gh`, it only produces the DMG — upload it manually on the Release page.

> ⚠️ The DMG is a GitHub **Release asset**, not a git-tracked file (265+ MB).

A GitHub Action (`.github/workflows/release.yml`) can build and attach the DMG automatically on every tag — see the workflow file.

---

## 🧪 Tests

```bash
make test       # go test ./... (backend)
cd frontend && npm run typecheck
```

---

## 📜 License

MIT — see [LICENSE](LICENSE) for the full terms, including extensive disclaimers of warranty and liability, the no-affiliation notice (this project is independent of Spotify and Apple), and the user's sole responsibility for the legality of content downloaded with the app.

---

*Legacy documentation (detailed old-backend architecture): [README_legacy.md](README_legacy.md).*
