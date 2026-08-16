#!/usr/bin/env bash
# Génère le DMG d'installation de Soneph à partir du .app packagé
# (glisser-déposer vers /Applications inclus).
#
# Usage : ./make-dmg.sh [version]    (depuis desktop/ ou via desktop/make-dmg.sh)
#   version par défaut : 1.0.0
set -euo pipefail
cd "$(dirname "$0")"

OUT_DIR="$(ls -d dist/*/ 2>/dev/null | head -1 || true)"
APP="${OUT_DIR}Soneph.app"
if [ ! -d "$APP" ]; then
  echo "❌ $APP introuvable — lance d'abord ./build.sh" >&2
  exit 1
fi

VERSION="${1:-1.0.0}"
DMG="dist/Soneph-${VERSION}.dmg"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

echo "── Assemblage du DMG (version $VERSION) ──"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"

hdiutil create -volname "Soneph" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
echo "✅ $DMG"
