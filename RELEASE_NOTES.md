## Soneph v1.0.0 🎵

Your Spotify library, as high-quality MP3s — with full metadata, synced lyrics and automatic import into Apple Music.

### 📥 Install

Download the **DMG**, open it, drag **Soneph** into *Applications*.

> Requires macOS with the Music app installed, plus Python + `spotdl`:
> `pipx install spotdl`

### ✨ What's new

**Downloads & metadata**
- Spotify downloads at 128 / 192 / 320 kbps — playlists, albums, artists or single tracks
- Full ID3 metadata: cover, album, track, genre, year, **producers & songwriters** (TIPL/TMCL), ISRC, copyright
- **Single → Album upgrade without re-downloading**: the file is moved to its album folder and its tags rewritten — the audio is never fetched again
- Real bitrate stamped in the tags (`SONEPH_QUALITY`), download source recorded (`SONEPH_SOURCE`)

**Lyrics**
- Synced lyrics (`.lrc` + ID3 tags) from lrclib / netease / musixmatch…
- Source stored in the tags (`LYRICS_SOURCE`): only what can actually improve is re-fetched (plain text → synced), the best version is never re-downloaded

**Playlists**
- Paste a Spotify playlist link → the missing tracks are downloaded **and** the playlist is created at the same time; tracks already on disk are added instantly, zero re-downloads
- Create, add/remove, reorder, export `.m3u8`

**Stats that follow your files**
- Play history, likes and playlists are re-attached automatically when a file moves (single → album), when you delete a duplicate, or when a file with another copy is deleted
- "More details" panel (right-click): bitrate, lyrics source, artists, producers, Spotify link

**macOS app**
- Native dark UI with glassmorphism, real-time download queue (WebSocket)
- **Auto-import into Apple Music** — every new file lands in the Music app with its lyrics, no duplicates
- Choose your **library folder** from Sync & Settings
- Custom Soneph icon

### 🧰 Tech

Go (gin) backend + React/Vite/TypeScript frontend + Electron app + Python helpers (spotdl, mutagen).

### 📜 License

MIT — not affiliated with Spotify. For personal use of music you have the rights to.
