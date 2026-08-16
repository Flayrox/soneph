#!/usr/bin/env bash
# ====================================================================
# soneph — Dev Launcher
# Lance le backend Go (:8080) et le dev server Vite (:5173) en même
# temps, avec un arrêt propre sur Ctrl+C.
# Usage : ./scripts/dev.sh
# ====================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_LOG="${TMPDIR:-/tmp}/soneph-backend.log"
FRONTEND_LOG="${TMPDIR:-/tmp}/soneph-frontend.log"

BACKEND_PID=""
FRONTEND_PID=""

port_in_use() {
  lsof -ti :"$1" >/dev/null 2>&1
}

cleanup() {
  trap - INT TERM EXIT
  echo ""
  echo "🛑 Arrêt des serveurs..."
  [ -n "$BACKEND_PID" ] && kill "$BACKEND_PID" 2>/dev/null || true
  [ -n "$FRONTEND_PID" ] && kill "$FRONTEND_PID" 2>/dev/null || true
  # Ne laisse rien traîner sur nos ports
  lsof -ti :8080 2>/dev/null | xargs kill 2>/dev/null || true
  lsof -ti :5173 2>/dev/null | xargs kill 2>/dev/null || true
  echo "✅ Tout est arrêté."
  exit 0
}
trap cleanup INT TERM EXIT

# ── Vérifications préalables ──────────────────────────────────────
if ! command -v go >/dev/null 2>&1; then
  echo "❌ 'go' introuvable. Installe Go (https://go.dev/dl/) puis relance." >&2
  exit 1
fi
if ! command -v npm >/dev/null 2>&1; then
  echo "❌ 'npm' introuvable. Installe Node.js (https://nodejs.org/) puis relance." >&2
  exit 1
fi

if port_in_use 8080; then
  echo "⚠️  Le port 8080 est déjà utilisé — le backend ne pourra pas démarrer." >&2
fi
if port_in_use 5173; then
  echo "⚠️  Le port 5173 est déjà utilisé — le frontend ne pourra pas démarrer." >&2
fi

# ── Dépendances frontend ──────────────────────────────────────────
if [ ! -d "$ROOT/frontend/node_modules" ]; then
  echo "📦 Installation des dépendances frontend (première fois)..."
  (cd "$ROOT/frontend" && npm install)
fi

# ── Démarrage ─────────────────────────────────────────────────────
echo "🎵 soneph — dev servers"
echo "   🌐 Frontend : http://localhost:5173"
echo "   ⚙️  API      : http://localhost:8080"
echo "   📜 Logs     : $BACKEND_LOG | $FRONTEND_LOG"
echo "   (Ctrl+C pour tout arrêter)"
echo ""

(cd "$ROOT/backend" && go run .) >"$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

(cd "$ROOT/frontend" && npm run dev) >"$FRONTEND_LOG" 2>&1 &
FRONTEND_PID=$!

# Attend les serveurs : on s'arrête dès que l'un des deux meurt.
while kill -0 "$BACKEND_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
  sleep 1
done

if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
  echo "❌ Le backend Go s'est arrêté de manière inattendue. Dernières lignes :" >&2
  tail -20 "$BACKEND_LOG" >&2
fi
if ! kill -0 "$FRONTEND_PID" 2>/dev/null; then
  echo "❌ Le frontend Vite s'est arrêté de manière inattendue. Dernières lignes :" >&2
  tail -20 "$FRONTEND_LOG" >&2
fi
