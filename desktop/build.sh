#!/usr/bin/env bash
# Build complet de l'app macOS :
#   1. frontend Vite → backend/web/dist (embarqué dans le binaire Go)
#   2. binaire Go  → backend/bin/soneph-server
#   3. icône .icns
#   4. .app Electron (dist/Soneph-darwin-arm64/Soneph.app) avec le binaire
#      Go dans Resources + icône appliquée
#
# Usage : ./build.sh        (ou : npm run build)
set -euo pipefail
cd "$(dirname "$0")"
ROOT="$(cd .. && pwd)"

echo "── 1/4 Frontend (Vite) ──"
(cd "$ROOT/frontend" && { [ -d node_modules ] || { npm ci 2>/dev/null || npm install; }; } && npm run build:go)

echo "── 2/4 Binaire Go ──"
(cd "$ROOT/backend" && mkdir -p bin && go build -o bin/soneph-server .)

echo "── 3/4 Icône ──"
./make-icon.sh

echo "── 4/4 App Electron ──"
if [ ! -d node_modules ]; then
  npm install
fi
npm run pack

# Le dossier de sortie est dist/<Nom>-darwin-<arch>/
OUT_DIR="$(ls -d dist/*/ 2>/dev/null | head -1 || true)"
APP="${OUT_DIR}Soneph.app"
if [ ! -d "$APP" ]; then
  echo "❌ Soneph.app introuvable dans dist/" >&2
  exit 1
fi

# Binaire Go dans Resources
mkdir -p "$APP/Contents/Resources/bin"
cp "$ROOT/backend/bin/soneph-server" "$APP/Contents/Resources/bin/soneph-server"

# Scripts Python helpers dans Resources/bin, à côté du binaire : le backend
# les résout par rapport à son propre binaire (GetScriptPath → Resources/bin).
# Sans eux, les playlists, paroles, tags et stats échouent silencieusement
# dans l'app packagée (le cwd n'est pas fiable, le dossier ne contient que
# le binaire).
cp "$ROOT"/backend/*.py "$APP/Contents/Resources/bin/"

# Watcher d'auto-import dans Resources/scripts (résolu par le backend par
# rapport à son propre binaire — sinon « introuvable » dans l'app packagée)
mkdir -p "$APP/Contents/Resources/scripts"
cp "$ROOT/scripts/watch_and_import.sh" "$APP/Contents/Resources/scripts/watch_and_import.sh"

# Icône : la copier dans le bundle et la déclarer dans Info.plist
cp resources/icon.icns "$APP/Contents/Resources/icon.icns"
/usr/libexec/PlistBuddy -c "Set :CFBundleIconFile icon.icns" "$APP/Contents/Info.plist" 2>/dev/null \
  || /usr/libexec/PlistBuddy -c "Add :CFBundleIconFile string icon.icns" "$APP/Contents/Info.plist"

# Signature ad-hoc : évite les alertes Gatekeeper pour un build local.
if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "$APP" >/dev/null 2>&1 || true
fi

echo ""
echo "✅ $APP"
echo "   Lance : open \"$APP\""
