#!/usr/bin/env bash
# Release open source Soneph :
#   1. build complet (frontend + Go + app Electron + icône)
#   2. DMG d'installation
#   3. GitHub Release avec le DMG en asset (si gh est installé et connecté)
#
# Usage : ./scripts/release.sh v1.0.0
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Usage: $0 v1.0.0" >&2
  exit 1
fi
VERSION="${VERSION#v}"   # accepte "1.0.0" comme "v1.0.0"
TAG="v${VERSION}"

echo "── 1/3 Build de l'app ──"
(cd desktop && ./build.sh)

echo "── 2/3 DMG ──"
(cd desktop && ./make-dmg.sh "$VERSION")
DMG="desktop/dist/Soneph-${VERSION}.dmg"

echo "── 3/3 GitHub Release ──"
NOTES=$(cat <<'EOF'
### 📥 Installation
Télécharge le DMG, ouvre-le, glisse **Soneph** dans *Applications*.

### ✨ Nouveautés
- Téléchargement Spotify (128/192/320 kbps) avec métadonnées ID3 complètes
- Single → album sans re-téléchargement (déplacement + réécriture des tags)
- Paroles synchronisées avec source enregistrée dans les tags
- Lien de playlist = téléchargement + création de playlist en une fois
- Stats (écoutes, likes, playlists) qui suivent les fichiers déplacés/supprimés
- Auto-import dans l'app Musique (macOS)
EOF
)

if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
  if ! git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "── Tag $TAG ──"
    git tag "$TAG"
    git push origin "$TAG"
  fi
  gh release create "$TAG" "$DMG" --title "Soneph $VERSION" --notes "$NOTES"
  echo "✅ Release $TAG créée avec le DMG : $DMG"
else
  echo "ℹ️  gh (GitHub CLI) n'est pas disponible ou non connecté."
  echo "   → Le DMG est prêt : $DMG"
  echo "   → Crée la release à la main sur github.com/Flayrox/soneph/releases/new"
  echo "     (tag: $TAG) et dépose le DMG en asset."
  echo "   → Astuce : brew install gh && gh auth login"
fi
