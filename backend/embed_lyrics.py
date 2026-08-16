#!/usr/bin/env python3
import os
import sys
import glob
import re
from mutagen.id3 import ID3, USLT, SYLT, TXXX, ID3NoHeaderError

def parse_lrc_for_sylt(lrc_text):
    """Parses LRC format into SYLT timestamp pairs [(text, timestamp_in_ms), ...]"""
    sylt_data = []
    time_regex = re.compile(r'\[(\d{2}):(\d{2})\.(\d{2,3})\]')
    for line in lrc_text.splitlines():
        match = time_regex.search(line)
        if match:
            minutes = int(match.group(1))
            seconds = int(match.group(2))
            ms_part = int(match.group(3))
            if ms_part < 100:
                ms_part *= 10
            total_ms = (minutes * 60 + seconds) * 1000 + ms_part
            text = time_regex.sub('', line).strip()
            if text:
                sylt_data.append((text, total_ms))
    return sylt_data

def strip_lrc_timestamps(lrc_text):
    clean_lines = []
    for line in lrc_text.splitlines():
        cleaned = re.sub(r'\[\d{2}:\d{2}\.\d{2,3}\]', '', line).strip()
        if cleaned:
            clean_lines.append(cleaned)
    return '\n'.join(clean_lines)

def embed_lrc_into_mp3(folder):
    lrc_files = glob.glob(os.path.join(folder, "**/*.lrc"), recursive=True)
    for lrc_file in lrc_files:
        mp3_file = lrc_file[:-4] + ".mp3"
        if os.path.exists(mp3_file):
            try:
                with open(lrc_file, 'r', encoding='utf-8') as f:
                    raw_lrc = f.read()

                plain_lyrics = strip_lrc_timestamps(raw_lrc)
                sylt_data = parse_lrc_for_sylt(raw_lrc)
                sync_type = "synced" if len(sylt_data) > 0 else "unsynced"

                try:
                    tags = ID3(mp3_file)
                except ID3NoHeaderError:
                    tags = ID3()

                # Clean existing lyrics frames & custom tags
                tags.delall('USLT')
                tags.delall('SYLT')
                tags.delall('TXXX:LYRICS_SYNC_TYPE')
                tags.delall('TXXX:HAS_LYRICS')

                # 1. Add USLT (Unsynchronized lyrics) for plain text view
                tags.add(USLT(encoding=3, lang='fra', desc='', text=plain_lyrics))
                tags.add(USLT(encoding=3, lang='eng', desc='', text=plain_lyrics))
                tags.add(USLT(encoding=3, lang='XXX', desc='', text=plain_lyrics))

                # 2. Add SYLT (Synchronized Lyrics) frame for time-synced playback
                if sylt_data:
                    tags.add(SYLT(encoding=3, lang='fra', format=2, type=1, desc='', text=sylt_data))
                    tags.add(SYLT(encoding=3, lang='eng', format=2, type=1, desc='', text=sylt_data))
                    tags.add(SYLT(encoding=3, lang='XXX', format=2, type=1, desc='', text=sylt_data))

                # 3. Add custom TXXX tags for metadata inspection ("synced" | "unsynced")
                tags.add(TXXX(encoding=3, desc='LYRICS_SYNC_TYPE', text=[sync_type]))
                tags.add(TXXX(encoding=3, desc='HAS_LYRICS', text=['true']))

                # Save specifically as ID3v2.3 for maximum player compatibility
                tags.save(mp3_file, v2_version=3)
                print(f"Successfully embedded ID3v2.3 ({sync_type}) lyrics into: {mp3_file}")
            except Exception as e:
                print(f"Failed to embed lyrics in {mp3_file}: {e}")

if __name__ == "__main__":
    if len(sys.argv) > 1:
        target = sys.argv[1]
    elif os.path.exists("/app/downloads"):
        target = "/app/downloads"
    else:
        target = "./downloads"
    embed_lrc_into_mp3(target)
