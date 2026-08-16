#!/usr/bin/env python3
"""
lyrics_retry.py — Scan MP3s and fetch synced lyrics when missing or plain.

Règles « meilleures paroles possibles » :
  - .lrc avec horodatage (synced)   → c'est le meilleur dispo, on ne retouche pas.
  - .lrc sans horodatage (unsynced) → on tente une version synchronisée.
  - pas de .lrc                      → on récupère (synced puis texte brut).

À chaque succès, la source (fournisseur) est inscrite dans les tags ID3 du
MP3 : TXXX:LYRICS_SOURCE = "lrclib" | "netease" | "musixmatch" | ... pour
savoir d'où viennent les paroles et si une re-récupération vaut le coup.

Usage:
  python3 lyrics_retry.py <download_dir> [--scan-only]
    --scan-only: just list retry candidates, don't fetch
    (default): scan + fetch synced lyrics for each candidate

Outputs newline-delimited JSON progress events for streaming.
"""
import os
import sys
import glob
import json
import re
import socket
import threading
import concurrent.futures

def log(event_type: str, data: dict):
    """Print a JSON event line for streaming to the Go backend."""
    print(json.dumps({"type": event_type, **data}), flush=True)

# ── Timeout helper ────────────────────────────────────────────────────────────
# ⚠️  Un timeout via signal.SIGALRM ne peut PAS être utilisé dans les threads
# (ValueError: signal only works in main thread) — le pool parallèle ci-dessous
# échouerait en silence. On borne donc chaque requête réseau avec un timeout
# de socket par défaut, qui s'applique à tous les threads.

socket.setdefaulttimeout(6)

# ── ID3 tag reading / writing ────────────────────────────────────────────────

def get_mp3_tags(path: str):
    """Read title and artist from MP3 ID3 tags using mutagen."""
    try:
        from mutagen.mp3 import MP3
        from mutagen.id3 import ID3NoHeaderError
        audio = MP3(path)
        tags = audio.tags
        if tags is None:
            return None, None
        title = str(tags.get("TIT2", "")).strip() or None
        artist = str(tags.get("TPE1", "")).strip() or None
        return title, artist
    except Exception:
        return None, None


def set_lyrics_source(mp3_path: str, provider: str):
    """Inscrit TXXX:LYRICS_SOURCE (le fournisseur des paroles) dans le MP3."""
    try:
        from mutagen.id3 import ID3, TXXX, ID3NoHeaderError
        try:
            tags = ID3(mp3_path)
        except ID3NoHeaderError:
            tags = ID3()
        tags.delall("TXXX:LYRICS_SOURCE")
        tags.add(TXXX(encoding=3, desc="LYRICS_SOURCE", text=[provider]))
        # ID3v2.3 pour compatibilité maximale (comme embed_lyrics.py).
        tags.save(mp3_path, v2_version=3)
    except Exception:
        pass


def get_lyrics_source(mp3_path: str):
    """Lit TXXX:LYRICS_SOURCE s'il existe (pour ne pas re-tenter à vide)."""
    try:
        from mutagen.id3 import ID3
        tags = ID3(mp3_path)
        frames = tags.getall("TXXX:LYRICS_SOURCE")
        if frames:
            return str(frames[0]).strip() or None
    except Exception:
        pass
    return None

# ── Scan ─────────────────────────────────────────────────────────────────────

RE_TIMESTAMP = re.compile(r"\[\d{2}:\d{2}[.:]\d{2,3}\]")


def scan_lyrics(download_dir: str):
    """Classifie chaque MP3 : (synced, unsynced, missing)."""
    mp3s = sorted(glob.glob(os.path.join(download_dir, "**", "*.mp3"), recursive=True))
    synced, unsynced, missing = [], [], []
    for mp3 in mp3s:
        lrc = os.path.splitext(mp3)[0] + ".lrc"
        if not os.path.exists(lrc):
            missing.append((mp3, lrc))
            continue
        try:
            with open(lrc, encoding="utf-8", errors="ignore") as f:
                content = f.read(65536)
        except Exception:
            missing.append((mp3, lrc))
            continue
        if RE_TIMESTAMP.search(content):
            synced.append((mp3, lrc))
        else:
            unsynced.append((mp3, lrc))
    return mp3s, synced, unsynced, missing

# ── Retry ─────────────────────────────────────────────────────────────────────

PREFERRED_PROVIDERS = ["lrclib", "netease", "musixmatch", "megalobiz", "genius"]


def fetch_lyrics_with_source(query: str, synced_only: bool):
    """
    Cherche les paroles fournisseur par fournisseur (pour connaître la source)
    et retourne (lyrics, provider) ou (None, None).
    """
    import syncedlyrics
    for provider in PREFERRED_PROVIDERS:
        try:
            lyrics = syncedlyrics.search(
                query, synced_only=synced_only, providers=[provider]
            )
        except TypeError:
            # Ancienne version de syncedlyrics sans paramètre providers : on
            # ne peut pas connaître la source, on marque "unknown".
            try:
                lyrics = syncedlyrics.search(query, synced_only=synced_only)
            except Exception:
                lyrics = None
            if lyrics and len(lyrics.strip()) > 10:
                return lyrics, "unknown"
            return None, None
        except Exception:
            lyrics = None
        if lyrics and len(lyrics.strip()) > 10:
            return lyrics, provider
    return None, None


def retry_lyrics_for_song(mp3_path: str, lrc_path: str, has_existing_lrc: bool):
    """
    Essaie de récupérer des paroles synchronisées pour un MP3 et écrit son
    .lrc + le tag TXXX:LYRICS_SOURCE. Retourne (status, detail) où status
    est "success" | "failed" | "kept".
    """
    try:
        import syncedlyrics  # noqa: F401  (vérifie que c'est installé)
    except ImportError:
        return "failed", "syncedlyrics not installed"

    title, artist = get_mp3_tags(mp3_path)

    # Build search query from tags, fallback to filename
    if title and artist:
        query = f"{title} - {artist}"
    elif title:
        query = title
    else:
        query = os.path.splitext(os.path.basename(mp3_path))[0].strip()

    if not query:
        return "failed", "no query"

    # Priorité absolue aux paroles synchronisées. Pour un fichier qui a déjà
    # un .lrc en texte brut, on ne fait QUE la tentative synced : s'il n'y a
    # rien de mieux, on garde l'existant (pas de downgrade).
    lyrics, provider = fetch_lyrics_with_source(query, synced_only=True)
    if lyrics:
        try:
            with open(lrc_path, "w", encoding="utf-8") as f:
                f.write(lyrics)
            set_lyrics_source(mp3_path, provider)
            return "success", query
        except Exception as e:
            return "failed", f"write error: {e}"

    if has_existing_lrc:
        return "kept", "already plain lyrics, no synced version available"

    # Pas de .lrc du tout : on retombe sur le texte brut comme filet de
    # sécurité (source "unknown" car le fournisseur n'est pas garanti).
    lyrics, provider = fetch_lyrics_with_source(query, synced_only=False)
    if lyrics:
        try:
            with open(lrc_path, "w", encoding="utf-8") as f:
                f.write(lyrics)
            set_lyrics_source(mp3_path, provider or "unknown")
            return "success", query
        except Exception as e:
            return "failed", f"write error: {e}"

    return "failed", f"no lyrics found for: {query}"

# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    if len(sys.argv) > 1 and not sys.argv[1].startswith("--"):
        download_dir = sys.argv[1]
    elif os.path.exists("/app/downloads"):
        download_dir = "/app/downloads"
    else:
        download_dir = "./downloads"

    scan_only = "--scan-only" in sys.argv

    log("scan_start", {"download_dir": download_dir})

    all_mp3s, synced, unsynced, missing = scan_lyrics(download_dir)
    candidates = missing + unsynced

    log("scan_complete", {
        "total_mp3s": len(all_mp3s),
        "missing_lrc": len(missing),
        "unsynced_lrc": len(unsynced),
        "synced_lrc": len(synced),
        "scan_only": scan_only,
    })

    log("missing_list", {
        "songs": [
            {
                "mp3": mp3,
                "filename": os.path.splitext(os.path.basename(mp3))[0],
                "lrc": lrc,
                "status": "missing" if (mp3, lrc) in missing else "unsynced",
            }
            for mp3, lrc in candidates[:200]
        ]
    })

    if scan_only or not candidates:
        return

    # Retry mode in parallel (8 threads)
    success = 0
    failed = 0
    kept = 0
    total = len(candidates)
    lock = threading.Lock()
    completed_counter = 0

    def process_item(item):
        nonlocal completed_counter, success, failed, kept
        mp3_path, lrc_path = item
        filename = os.path.splitext(os.path.basename(mp3_path))[0]
        has_existing_lrc = os.path.exists(lrc_path)

        try:
            status, detail = retry_lyrics_for_song(mp3_path, lrc_path, has_existing_lrc)
        except Exception as e:
            status, detail = "failed", f"unexpected error: {e}"

        with lock:
            completed_counter += 1
            idx = completed_counter
            if status == "success":
                success += 1
                log("success", {
                    "index": idx,
                    "total": total,
                    "filename": filename,
                    "query": detail,
                })
            elif status == "kept":
                kept += 1
                log("kept", {
                    "index": idx,
                    "total": total,
                    "filename": filename,
                    "reason": detail,
                })
            else:
                failed += 1
                log("failed", {
                    "index": idx,
                    "total": total,
                    "filename": filename,
                    "reason": detail,
                })

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as executor:
        futures = [executor.submit(process_item, item) for item in candidates]
        concurrent.futures.wait(futures)

    log("done", {
        "total": total,
        "success": success,
        "failed": failed,
        "kept": kept,
    })

if __name__ == "__main__":
    main()
