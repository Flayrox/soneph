#!/usr/bin/env python3
import sys
import os

def extract_cover():
    if len(sys.argv) < 3:
        sys.exit(1)

    mp3_path = sys.argv[1]
    output_path = sys.argv[2]

    if not os.path.exists(mp3_path):
        sys.exit(1)

    try:
        from mutagen.mp3 import MP3
        from mutagen.id3 import APIC

        audio = MP3(mp3_path)
        if audio.tags:
            for tag in audio.tags.values():
                if isinstance(tag, APIC):
                    os.makedirs(os.path.dirname(output_path), exist_ok=True)
                    with open(output_path, "wb") as f:
                        f.write(tag.data)
                    sys.exit(0)
    except Exception:
        pass

    sys.exit(1)

if __name__ == "__main__":
    extract_cover()
