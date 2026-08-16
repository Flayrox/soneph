#!/usr/bin/env python3
"""
Pré-crée les dossiers d'album avant le passage métadonnées de spotdl.

Quand un morceau existe déjà sur disque (souvent un single téléchargé avant
son album), spotdl --overwrite metadata le déplace vers
{artist}/{album}/{title} et réécrit ses tags sans re-télécharger. Mais son
déplacement échoue si le dossier d'album n'existe pas encore (Path.replace
sans mkdir). Ce script calcule exactement les mêmes chemins de sortie que
spotdl (create_file_name) et crée les dossiers parents à l'avance.

Usage:
    precreate_dirs.py <download_dir> <output_template> <url>
"""
import os
import sys
import json

from spotdl.utils.config import SPOTIFY_OPTIONS
from spotdl.utils.formatter import create_file_name
from spotdl.utils.search import parse_query
from spotdl.utils.spotify import SpotifyClient


def main():
    download_dir = sys.argv[1] if len(sys.argv) > 1 else "./downloads"
    template = sys.argv[2] if len(sys.argv) > 2 else ""
    url = sys.argv[3] if len(sys.argv) > 3 else ""

    if not url:
        return

    try:
        # Mêmes réglages que le CLI spotdl (client anonyme par défaut).
        SpotifyClient.init(**SPOTIFY_OPTIONS)
        # Mêmes défauts que le CLI spotdl (album_type=None, pas de numbering).
        songs = parse_query([url], threads=4)
    except Exception as e:
        print(f"precreate_dirs: parse_query failed: {e}", file=sys.stderr)
        return

    created = 0
    for song in songs:
        if not getattr(song, "album_name", None):
            continue
        try:
            output_file = create_file_name(
                song=song,
                template=template,
                file_extension="mp3",
                restrict=None,
                short=False,
                file_name_length=None,
            )
            os.makedirs(output_file.parent, exist_ok=True)
            created += 1
        except Exception as e:
            print(f"precreate_dirs: {song.display_name}: {e}", file=sys.stderr)

    if created > 0:
        print(json.dumps({
            "precreated_dirs": created,
            "songs": len(songs),
        }))


if __name__ == "__main__":
    main()
