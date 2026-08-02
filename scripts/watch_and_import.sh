#!/usr/bin/env bash
# ====================================================================
# soneph : Automatic Background l'app Musique Importer for macOS
# ====================================================================
# Runs silently on your Mac and auto-imports all downloaded MP3s
# into l'app Musique as soon as soneph finishes downloading them!
# ====================================================================

DOWNLOADS_DIR="/Users/ephe/Desktop/dev/ephelstudio/downloads"

echo "🎵 soneph Auto-Importer active! Watching: $DOWNLOADS_DIR"

while true; do
    osascript -e "
    tell application \"Music\"
        set downloadFolder to POSIX file \"$DOWNLOADS_DIR\"
        add downloadFolder
    end tell" > /dev/null 2>&1
    sleep 10
done
