#!/usr/bin/env bash
# ====================================================================
# soneph — l'app Musique Auto-Importer (macOS)
# ====================================================================
# Surveille le dossier de téléchargements (downloads/ du repo, ou le
# dossier synchronisé par Syncthing) et copie chaque NOUVEAU fichier
# audio (mp3/m4a/flac) — avec son .lrc s'il existe — dans le dossier
# « Automatically Add to Music » de l'app Musique.
#
# Music.app importe le fichier automatiquement puis le DÉPLACE hors du
# dossier : aucun doublon, aucune permission d'automatisation, aucun
# AppleScript. Un fichier d'état (~/.soneph/imported.txt) garantit
# qu'un fichier n'est jamais importé deux fois.
#
# ⚠️ Si tu avais attaché l'ancienne « Folder Action » (import_to_music.scpt)
#    au dossier synchronisé, détache-la (clic droit sur le dossier →
#    Folder Actions → Detach), sinon tu garderas des doublons.
#
# Usage :
#   ./scripts/watch_and_import.sh                     # dossier par défaut
#   SONEPH_DOWNLOADS=/chemin/dossier-syncthing ./scripts/watch_and_import.sh
#   SONEPH_AUTO_ADD_DIR=/chemin/perso      ./scripts/watch_and_import.sh
#   SONEPH_IMPORT_ALL=1                    ./scripts/watch_and_import.sh
#        # 1er lancement : importer AUSSI la bibliothèque déjà présente
#
# Dépendance optionnelle : fswatch (brew install fswatch) → réaction
# instantanée. Sans lui, bascule automatique en mode polling (5 s).
# ====================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

DOWNLOADS_DIR="${SONEPH_DOWNLOADS:-$REPO_ROOT/downloads}"
AUTO_ADD_DIR_OVERRIDE="${SONEPH_AUTO_ADD_DIR:-}"
STATE_DIR="${SONEPH_STATE_DIR:-$HOME/.soneph}"
STATE_FILE="$STATE_DIR/imported.txt"

POLL_INTERVAL=5          # mode polling (sans fswatch)
DEFER_RESCAN=10          # re-scan après un fichier différé (s)
FRESH_GRACE=30           # sans .lrc : attendre N s après la dernière écriture

# ── Dossier « Automatically Add to Music » ─────────────────────────
find_auto_add_dir() {
  local candidates=(
    "$HOME/Music/Music/Media.localized/Automatically Add to Music.localized"
    "$HOME/Music/Music/Automatically Add to Music"
    "$HOME/Music/iTunes/iTunes Media/Automatically Add to Music"
    "$HOME/Music/iTunes/iTunes Media/Automatically Add to iTunes"
  )
  local dir
  for dir in "${candidates[@]}"; do
    if [ -d "$dir" ]; then
      echo "$dir"
      return 0
    fi
  done
  # Dernier recours : le dossier peut être ailleurs (bibliothèque externe…)
  if command -v mdfind >/dev/null 2>&1; then
    local found
    found="$(mdfind "kMDItemFSName == 'Automatically Add to Music.localized'c" 2>/dev/null | head -1)"
    if [ -n "$found" ]; then
      echo "$found"
      return 0
    fi
  fi
  return 1
}

# ── Est-ce que le fichier est prêt à être importé ? ────────────────
# Un fichier est importé quand :
#   • personne ne l'écrit (lsof), et
#   • son .lrc existe (paroles prêtes) — import immédiat,
#     OU il n'a pas été modifié depuis FRESH_GRACE s (cas sans paroles).
file_ready() {
  local f="$1" lrc mtime now age
  # Encore ouvert (spotdl / embed / syncthing en train d'écrire) → pas prêt
  if lsof "$f" >/dev/null 2>&1; then
    return 1
  fi
  lrc="${f%.*}.lrc"
  if [ -f "$lrc" ]; then
    # .lrc présent mais peut-être encore en cours d'écriture
    if lsof "$lrc" >/dev/null 2>&1; then
      return 1
    fi
    return 0
  fi
  mtime="$(stat -f %m "$f" 2>/dev/null || echo 0)"
  now="$(date +%s)"
  age=$((now - mtime))
  [ "$age" -ge "$FRESH_GRACE" ]
}

# ── Import des nouveaux fichiers ───────────────────────────────────
# Le fichier d'état contient les chemins absolus déjà importés.
# comm -13 = fichiers audio présents sur le disque mais PAS dans l'état.
import_new_files() {
  local file rel lrc imported=0 deferred=0

  while IFS= read -r file; do
    [ -z "$file" ] && continue
    rel="${file#"$DOWNLOADS_DIR"/}"
    [[ "$rel" == .* ]] && continue          # fichiers cachés

    if ! file_ready "$file"; then
      deferred=1
      continue
    fi

    if cp -p "$file" "$AUTO_ADD_DIR/"; then
      echo "$file" >> "$STATE_FILE"
      echo "  ✅ Importé : $(basename "$file")"
      imported=1
      lrc="${file%.*}.lrc"
      if [ -f "$lrc" ]; then
        cp -p "$lrc" "$AUTO_ADD_DIR/" 2>/dev/null || true
      fi
    else
      echo "  ⚠️  Échec copie : $rel" >&2
    fi
  done < <(comm -13 \
      <(sort -u "$STATE_FILE" 2>/dev/null) \
      <(find "$DOWNLOADS_DIR" -type f \( -iname "*.mp3" -o -iname "*.m4a" -o -iname "*.flac" \) 2>/dev/null | sort -u))

  if [ "$imported" -eq 1 ]; then
    echo ""
  fi
  # Retourne 0 si rien à re-scan, 1 si des fichiers sont encore en cours
  return "$deferred"
}

# ── Démarrage ──────────────────────────────────────────────────────
if [ ! -d "$DOWNLOADS_DIR" ]; then
  echo "❌ Dossier introuvable : $DOWNLOADS_DIR" >&2
  echo "   Passe le bon chemin via SONEPH_DOWNLOADS." >&2
  exit 1
fi

if [ -n "$AUTO_ADD_DIR_OVERRIDE" ]; then
  AUTO_ADD_DIR="$AUTO_ADD_DIR_OVERRIDE"
else
  AUTO_ADD_DIR="$(find_auto_add_dir || true)"
fi
if [ -z "$AUTO_ADD_DIR" ]; then
  echo "❌ Dossier « Automatically Add to Music » introuvable." >&2
  echo "   Ouvre l'app Musique une fois (elle crée ce dossier), puis relance." >&2
  exit 1
fi

mkdir -p "$STATE_DIR"

# Premier lancement : marquer la bibliothèque existante comme déjà vue
# pour éviter d'importer (et dupliquer) tout ce qui est déjà dans l'app.
if [ ! -f "$STATE_FILE" ]; then
  touch "$STATE_FILE"
  if [ "${SONEPH_IMPORT_ALL:-0}" != "1" ]; then
    find "$DOWNLOADS_DIR" -type f \( -iname "*.mp3" -o -iname "*.m4a" -o -iname "*.flac" \) 2>/dev/null >> "$STATE_FILE"
    echo "   ℹ️  Bibliothèque existante marquée comme déjà importée."
    echo "      Seuls les NOUVEAUX fichiers seront ajoutés à Musique."
    echo "      (SONEPH_IMPORT_ALL=1 pour tout importer au 1er lancement)"
  fi
fi

# ── Boucle principale ──────────────────────────────────────────────
MODE="poll"
if command -v fswatch >/dev/null 2>&1; then
  MODE="fswatch"
fi

echo "🎵 soneph — l'app Musique Auto-Importer"
echo "   📁 Surveillance : $DOWNLOADS_DIR"
echo "   🍎 Import vers   : $AUTO_ADD_DIR"
echo "   👀 Mode         : $MODE"
echo "   (Ctrl+C pour arrêter)"
echo ""

trap 'echo ""; echo "🛑 Auto-Importer arrêté."; exit 0' INT TERM

while true; do
  deferred=0
  import_new_files || deferred=1

  if [ "$deferred" -eq 1 ]; then
    # Des fichiers sont encore en cours d'écriture : on re-scannera
    # sans attendre un événement fswatch.
    sleep "$DEFER_RESCAN"
    continue
  fi

  if [ "$MODE" = "fswatch" ]; then
    # Attend le prochain changement dans le dossier (fichiers audio)
    fswatch -1 -r \
      --include='\.(mp3|m4a|flac)$' \
      --exclude='\.lrc$' \
      "$DOWNLOADS_DIR" >/dev/null 2>&1 || {
        echo "   ⚠️ fswatch indisponible → mode polling" >&2
        MODE="poll"
        sleep "$POLL_INTERVAL"
      }
  else
    sleep "$POLL_INTERVAL"
  fi
done
