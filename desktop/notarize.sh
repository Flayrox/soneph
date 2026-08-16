#!/usr/bin/env bash
# Notarisation macOS de l'app Soneph — supprime l'avertissement Gatekeeper
# à la première ouverture (« l'app provient d'un développeur non identifié »).
#
# Prérequis :
#   1. Un certificat "Developer ID Application" dans le trousseau
#      (Apple Developer, 99 $/an) — codesign le détecte automatiquement
#   2. Un Apple ID + un mot de passe spécifique à l'app
#      (appleid.apple.com → Connexion et sécurité → Mots de passe d'app)
#
# Usage :
#   APPLE_ID=ton@email.com \
#   APPLE_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx \
#   APPLE_TEAM_ID=XXXXXXXXXX \
#   desktop/notarize.sh
#
# Variables optionnelles :
#   CODESIGN_IDENTITY  identité de signature (défaut : "Developer ID Application")
set -euo pipefail
cd "$(dirname "$0")"

: "${APPLE_ID:?APPLE_ID manquant — ton Apple ID (email)}"
: "${APPLE_APP_PASSWORD:?APPLE_APP_PASSWORD manquant — mot de passe d'app (appleid.apple.com)}"
: "${APPLE_TEAM_ID:?APPLE_TEAM_ID manquant — Team ID (developer.apple.com/account)}"

IDENTITY="${CODESIGN_IDENTITY:-Developer ID Application}"
OUT_DIR="$(ls -d dist/*/ 2>/dev/null | head -1 || true)"
APP="${OUT_DIR}Soneph.app"

if [ ! -d "$APP" ]; then
  echo "❌ $APP introuvable — lance d'abord ./build.sh" >&2
  exit 1
fi

echo "── 1/4 Signature (Developer ID + hardened runtime) ──"
codesign --force --deep --options runtime --sign "$IDENTITY" "$APP"

echo "── 2/4 Vérification signature ──"
codesign --verify --deep --strict "$APP"

echo "── 3/4 Notarisation (notarytool) ──"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
ditto -c -k --keepParent "$APP" "$STAGE/Soneph.zip"

xcrun notarytool submit "$STAGE/Soneph.zip" \
  --apple-id "$APPLE_ID" \
  --password "$APPLE_APP_PASSWORD" \
  --team-id "$APPLE_TEAM_ID" \
  --wait

echo "── 4/4 Agrafage du ticket + vérification ──"
xcrun stapler staple "$APP"
spctl -a -vv "$APP" && echo "✅ $APP notarisé — plus d'avertissement Gatekeeper"

echo ""
echo "Reconstruis maintenant le DMG (il doit contenir l'app notarisée) :"
echo "  desktop/make-dmg.sh 1.0.0"
