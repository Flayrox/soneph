#!/usr/bin/env bash
# Génère resources/icon.icns depuis resources/icon.png (le logo soneph,
# 1920×1920 fourni à la racine du repo) avec des outils macOS intégrés
# (sips + iconutil) — aucune dépendance.
set -euo pipefail
cd "$(dirname "$0")/resources"

if [ ! -f icon.png ]; then
  echo "icon.png introuvable — copie le logo du repo : cp ../Sonephe.png icon.png" >&2
  exit 1
fi

# PNG → ICNS (via iconset + iconutil, la méthode canonique)
rm -rf icon.iconset icon.icns
mkdir -p icon.iconset
for spec in "16 icon_16x16" "32 icon_16x16@2x" "32 icon_32x32" "64 icon_32x32@2x" \
            "128 icon_128x128" "256 icon_128x128@2x" "256 icon_256x256" \
            "512 icon_256x256@2x" "512 icon_512x512"; do
  size="${spec%% *}"
  name="${spec##* }"
  sips -z "$size" "$size" icon.png --out "icon.iconset/${name}.png" >/dev/null
done
cp icon.png "icon.iconset/icon_512x512@2x.png"
iconutil -c icns icon.iconset -o icon.icns
rm -rf icon.iconset

echo "✅ resources/icon.icns (depuis le logo soneph)"
