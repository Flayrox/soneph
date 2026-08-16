#!/usr/bin/env python3
"""
Marqueur soneph dans les métadonnées ID3 de chaque fichier audio.

Inscrit (si absent, de façon idempotente) deux tags personnalisés :
  - TXXX:SONEPH        = "true"  → le fichier est géré par soneph
  - TXXX:SONEPH_SOURCE = URL     → d'où vient le morceau (playlist/album)

Usage:
    tag_soneph.py <dossier> [source_url]
"""
import os
import sys
import glob

from mutagen.id3 import ID3, TXXX, ID3NoHeaderError


def mp3_bitrate_kbps(f: str):
    """Bitrate réel du fichier (bps → kbps), ou None si illisible."""
    try:
        from mutagen.mp3 import MP3
        info = MP3(f).info
        if info and info.bitrate:
            return round(info.bitrate / 1000)
    except Exception:
        pass
    return None


def stamp(folder: str, source_url: str = "") -> int:
    files = glob.glob(os.path.join(folder, "**", "*.mp3"), recursive=True)
    stamped = 0
    for f in files:
        try:
            try:
                tags = ID3(f)
            except ID3NoHeaderError:
                tags = ID3()

            changed = False
            if not tags.getall("TXXX:SONEPH"):
                tags.add(TXXX(encoding=3, desc="SONEPH", text=["true"]))
                changed = True
            if source_url and not tags.getall("TXXX:SONEPH_SOURCE"):
                tags.add(TXXX(encoding=3, desc="SONEPH_SOURCE", text=[source_url]))
                changed = True

            # Qualité réelle du fichier : le programme sait si on est déjà au
            # meilleur débit possible sans avoir à ré-ouvrir le fichier.
            bitrate = mp3_bitrate_kbps(f)
            if bitrate:
                label = f"{bitrate}kbps"
                current = tags.getall("TXXX:SONEPH_QUALITY")
                if not current or str(current[0]) != label:
                    tags.delall("TXXX:SONEPH_QUALITY")
                    tags.add(TXXX(encoding=3, desc="SONEPH_QUALITY", text=[label]))
                    changed = True

            if changed:
                # ID3v2.3 pour compatibilité maximale (comme embed_lyrics.py).
                tags.save(f, v2_version=3)
                stamped += 1
        except Exception as e:
            print(f"⚠️ {f}: {e}", file=sys.stderr)
    return stamped


if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "./downloads"
    source = sys.argv[2] if len(sys.argv) > 2 else ""
    n = stamp(target, source)
    if n > 0:
        print(f"SONEPH tags: {n} fichier(s) marqué(s) dans les métadonnées.")
