# 🎵 son<span text-color="rose">ephe</span> — High Quality Music Downloader & Auto-Sync

> **soneph** *(son + ephe)* est une plateforme moderne et ultra-rapide d'automatisation de téléchargement de musique (320kbps + métadonnées ID3v2 + paroles synchronisées `.lrc`) et de synchronisation transparente P2P vers l'app Musique / iTunes / iPhone.

---

## ✨ Features principales

- 🎧 **Qualité 320 kbps HD** : Extraction automatique depuis  / YouTube avec tags complets (Pochette, Artiste, Album, Paroles LRC).
- 🎤 **Lecteur Karaoké intégré** : Suivi des paroles synchronisées en direct dans le Dashboard Web.
- ⚡ **Backend Go (Gin)** : Moteur haute performance gérant les files d'attente et WebSockets temps réel.
- 📱 **Synchronisation iOS & Windows** : Intégration P2P via Syncthing pour injecter directement les sons dans l'app Musique / iTunes sans câble.
- 🐳 **Docker Native** : Déploiement en 1 seule commande avec Docker Compose — un seul conteneur web (le frontend Vite est embarqué dans le binaire Go).

---

## 🚀 Démarrage Rapide (Docker)

```bash
# 1. Cloner le dépôt
git clone https://github.com/votre-compte/soneph.git
cd soneph

# 2. Lancer la stack complète avec Docker Compose
docker compose up -d --build
```

### 🌐 Endpoints par défaut
- 🎨 **Dashboard Web + API (même origine)** : `http://localhost:8080`
- 🔄 **Console Syncthing UI** : `http://localhost:8384`

---

## 🛠️ Stack Technique

| Composant | Technologie |
| :--- | :--- |
| **Frontend** | Vite + React 18, TypeScript, Tailwind CSS (SPA 100 % client, embarquée dans le binaire Go) |
| **Backend** | Go (Gin Framework), WebSockets, `go:embed` pour servir le frontend |
| **Downloader** | `spotdl` CLI (Python 3.11 + FFmpeg) |
| **Sync Engine** | Syncthing P2P |
| **Container** | Docker & Docker Compose |

---

## 💻 Développement Local

Le frontend tourne sur Vite (`:5173`) et le backend Go sur `:8080`. Le dev server Vite proxy automatiquement `/api` (HTTP + WebSocket) vers le backend, donc tout est same-origin côté navigateur.

### Un seul script (recommandé)
```bash
./scripts/dev.sh
# → Frontend : http://localhost:5173
# → API      : http://localhost:8080
# Ctrl+C pour tout arrêter
```

Le script lance les deux serveurs, installe les dépendances frontend au premier lancement, surveille les ports et arrête tout proprement.

### Réglages téléchargement (variables d'environnement)

| Variable | Défaut | Rôle |
| :--- | :--- | :--- |
| `SPOTDL_WORKERS` | `4` | Nombre de processus spotdl parallèles (un par URL dans la file). Trop élevé → rate limiting /YouTube. |
| `SPOTDL_THREADS` | `6` | Chansons téléchargées en parallèle par processus spotdl. |
| `DOWNLOAD_DIR` | `./downloads` | Dossier de destination (en Docker : `/app/downloads`). |

> ⚡ Les paroles sont désormais récupérées **en arrière-plan** après le téléchargement : l'audio arrive vite, les `.lrc` suivent sans bloquer la file d'attente.

### Réglages depuis l'UI

La vue **Sync & Réglages** (barre latérale) permet de :
- démarrer / arrêter l'**auto-import l'app Musique** (watcher macOS, sans doublon),
- régler les **threads / workers** de téléchargement sans toucher au code.

> Sur macOS, l'auto-import copie les nouveaux fichiers vers le dossier « Automatically Add to Music » de l'app Musique. Sur un VPS (Linux), il est désactivé — la distribution se fait via Syncthing.

### À la main (alternative)
```bash
# Terminal 1 — Backend (Go)
cd backend
go run main.go

# Terminal 2 — Frontend (Vite)
cd frontend
npm install
npm run dev   # http://localhost:5173
```

> 💡 Pour builder le frontend dans le binaire Go (comme en production) : `cd frontend && npm run build:go` — copie `dist/` vers `backend/web/dist/` que le backend embarque via `go:embed`.

---

<p center>
Fait avec ❤️ par <b>son<span style="color: #f43f5e;">ephe</span></b>
</p>
