#!/usr/bin/env python3
"""
lyrics_retry.py — Scan MP3s without .lrc files and retry fetching synced lyrics.

Usage:
  python3 lyrics_retry.py <download_dir> [--scan-only]
    --scan-only: just list missing .lrc files, don't retry
    (default): scan + retry synced lyrics for each missing song

Outputs newline-delimited JSON progress events for streaming.
"""
import os
import sys
import glob
import json
import time
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

# ── ID3 tag reading ───────────────────────────────────────────────────────────

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

# ── Scan ─────────────────────────────────────────────────────────────────────

def scan_missing_lrc(download_dir: str):
    """Return list of (mp3_path, lrc_path) tuples where lrc is missing."""
    mp3s = sorted(glob.glob(os.path.join(download_dir, "**", "*.mp3"), recursive=True))
    missing = []
    for mp3 in mp3s:
        lrc = os.path.splitext(mp3)[0] + ".lrc"
        if not os.path.exists(lrc):
            missing.append((mp3, lrc))
    return mp3s, missing

# ── Retry ─────────────────────────────────────────────────────────────────────

PREFERRED_PROVIDERS = ["lrclib", "netease", "musixmatch", "megalobiz", "genius"]

def retry_lyrics_for_song(mp3_path: str, lrc_path: str):
    """
    Try to fetch synced lyrics for a single MP3 and write its .lrc file.
    Returns True on success, False on failure.
    """
    try:
        import syncedlyrics
    except ImportError:
        return False, "syncedlyrics not installed"

    title, artist = get_mp3_tags(mp3_path)

    # Build search query from tags, fallback to filename
    if title and artist:
        query = f"{title} - {artist}"
    elif title:
        query = title
    else:
        query = os.path.splitext(os.path.basename(mp3_path))[0].strip()

    if not query:
        return False, "no query"

    def _search(synced_only: bool):
        try:
            return syncedlyrics.search(query, synced_only=synced_only, providers=PREFERRED_PROVIDERS)
        except TypeError:
            return syncedlyrics.search(query, synced_only=synced_only)
        except Exception:
            return None

    # Essaie d'abord les paroles synchronisées, puis le texte brut
    for synced_only in (True, False):
        try:
            lyrics = _search(synced_only)
        except Exception as e:
            return False, f"error: {e}"
        if lyrics and len(lyrics.strip()) > 10:
            try:
                with open(lrc_path, "w", encoding="utf-8") as f:
                    f.write(lyrics)
                return True, query
            except Exception as e:
                return False, f"write error: {e}"

    return False, f"no lyrics found for: {query}"

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

    all_mp3s, missing = scan_missing_lrc(download_dir)

    log("scan_complete", {
        "total_mp3s": len(all_mp3s),
        "missing_lrc": len(missing),
        "scan_only": scan_only,
    })

    if scan_only or not missing:
        log("missing_list", {
            "songs": [
                {
                    "mp3": mp3,
                    "filename": os.path.splitext(os.path.basename(mp3))[0],
                    "lrc": lrc,
                }
                for mp3, lrc in missing[:200]
            ]
        })
        return

    # Retry mode in parallel (8 threads)
    success = 0
    failed = 0
    total = len(missing)
    lock = threading.Lock()
    completed_counter = 0

    def process_item(item):
        nonlocal completed_counter, success, failed
        mp3_path, lrc_path = item
        filename = os.path.splitext(os.path.basename(mp3_path))[0]

        try:
            ok, detail = retry_lyrics_for_song(mp3_path, lrc_path)
        except Exception as e:
            ok, detail = False, f"unexpected error: {e}"

        with lock:
            completed_counter += 1
            idx = completed_counter
            if ok:
                success += 1
                log("success", {
                    "index": idx,
                    "total": total,
                    "filename": filename,
                    "query": detail,
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
        futures = [executor.submit(process_item, item) for item in missing]
        concurrent.futures.wait(futures)

    log("done", {
        "total": total,
        "success": success,
        "failed": failed,
    })

if __name__ == "__main__":
    main()
