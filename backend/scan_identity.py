#!/usr/bin/env python3
"""
scan_identity.py — carte {identité → chemins} de la bibliothèque.

L'identité stable d'un morceau est son URL Spotify (tag WOAS) : c'est ce que
spotdl utilise pour retrouver un fichier et le déplacer vers son album quand
il est re-téléchargé dans un autre contexte (single → album). En comparant
cette carte avant/après un téléchargement, le backend sait quels fichiers ont
bougé et peut migrer les stats (historique, likes, playlists) vers le nouveau
chemin.

Usage (CLI) :
    scan_identity.py <download_dir>
Sortie : JSON {"url:https://open.spotify.com/track/...": ["rel/path1.mp3", ...]}
"""
import os
import sys
import glob
import json

from mutagen.id3 import ID3


def identity_map(folder: str) -> dict:
    """Renvoie {identité → [rel_path, ...]} en lisant les tags WOAS."""
    out = {}
    for f in sorted(glob.glob(os.path.join(folder, "**", "*.mp3"), recursive=True)):
        rel = os.path.relpath(f, folder)
        try:
            tags = ID3(f)
            woas = tags.get("WOAS")
            if woas:
                out.setdefault("url:" + str(woas).strip(), []).append(rel)
        except Exception:
            pass
    return out


def main():
    folder = sys.argv[1] if len(sys.argv) > 1 else "./downloads"
    print(json.dumps(identity_map(folder)))


if __name__ == "__main__":
    main()
