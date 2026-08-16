#!/usr/bin/env python3
"""
file_details.py — dump complet des métadonnées ID3 d'un fichier (pour le
panneau « Plus de détails » au clic droit) : artistes, album, producteurs,
musiciens, qualité, source des paroles, URL Spotify…

Usage:
    file_details.py <download_dir> <rel_path>
Sortie : JSON (stdout), {"error": ...} si le fichier n'existe pas.
"""
import os
import sys
import re
import json

from mutagen.mp3 import MP3
from mutagen.id3 import ID3, ID3NoHeaderError

RE_TIMESTAMP = re.compile(r"\[\d{2}:\d{2}[.:]\d{2,3}\]")


def frame(tags, key):
    try:
        v = tags.get(key)
        return str(v) if v else None
    except Exception:
        return None


def main():
    if len(sys.argv) < 3:
        print(json.dumps({"error": "usage: file_details.py <download_dir> <rel_path>"}))
        return
    folder, rel = sys.argv[1], sys.argv[2]
    full = os.path.join(folder, rel)
    if not os.path.exists(full):
        print(json.dumps({"error": "file not found"}))
        return

    details = {
        "rel_path": rel,
        "file_name": os.path.basename(full),
    }

    try:
        audio = MP3(full)
        if audio.info:
            if audio.info.bitrate:
                details["bitrate"] = f"{round(audio.info.bitrate / 1000)}kbps"
            if audio.info.length:
                details["duration_seconds"] = round(audio.info.length)
    except Exception:
        pass

    try:
        tags = ID3(full)
    except ID3NoHeaderError:
        tags = None

    if tags is not None:
        details["title"] = frame(tags, "TIT2")
        details["artist"] = frame(tags, "TPE1")
        details["album"] = frame(tags, "TALB")
        details["album_artist"] = frame(tags, "TPE2")
        details["year"] = frame(tags, "TDRC")
        details["genre"] = frame(tags, "TCON")
        details["track"] = frame(tags, "TRCK")
        details["disc"] = frame(tags, "TPOS")
        details["writer"] = frame(tags, "TEXT")
        details["isrc"] = frame(tags, "TSRC")
        details["copyright"] = frame(tags, "TCOP")
        details["publisher"] = frame(tags, "TPUB") or frame(tags, "TENC")
        details["spotify_url"] = frame(tags, "WOAS")
        details["comment"] = frame(tags, "COMM::XXX")

        try:
            tipl = tags.getall("TIPL")
            details["involved_people"] = [list(p) for p in tipl[0].people] if tipl else []
        except Exception:
            details["involved_people"] = []
        try:
            tmcl = tags.getall("TMCL")
            details["musicians"] = [list(p) for p in tmcl[0].people] if tmcl else []
        except Exception:
            details["musicians"] = []

        custom = {}
        for txxx in tags.getall("TXXX"):
            try:
                custom[txxx.desc] = str(txxx)
            except Exception:
                pass
        details["custom_tags"] = custom
        details["lyrics_source"] = custom.get("LYRICS_SOURCE")
        details["quality"] = custom.get("SONEPH_QUALITY") or details.get("bitrate")
        details["source_url"] = custom.get("SONEPH_SOURCE")
        details["lyrics_sync_type"] = custom.get("LYRICS_SYNC_TYPE")

    # Type de paroles depuis le .lrc (synced / unsynced / none)
    lrc = os.path.splitext(full)[0] + ".lrc"
    details["has_lyrics"] = os.path.exists(lrc)
    details["lyrics_type"] = "none"
    if os.path.exists(lrc):
        try:
            with open(lrc, encoding="utf-8", errors="ignore") as f:
                content = f.read(65536)
            details["lyrics_type"] = "synced" if RE_TIMESTAMP.search(content) else "unsynced"
        except Exception:
            pass

    print(json.dumps(details, ensure_ascii=False))


if __name__ == "__main__":
    main()
