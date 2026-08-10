# 🎵 son<span text-color="rose">ephe</span> — High Quality Music Downloader & Auto-Sync

> **soneph** *(son + ephe)* est une plateforme moderne et ultra-rapide d'automatisation de téléchargement de musique (320kbps + métadonnées ID3v2 + paroles synchronisées `.lrc`) et de synchronisation transparente P2P vers l'app Musique / iTunes / iPhone.

---

## ✨ Features principales

- 🎧 **Qualité 320 kbps HD** : Extraction automatique depuis  / YouTube avec tags complets (Pochette, Artiste, Album, Paroles LRC).
- 🎤 **Lecteur Karaoké intégré** : Suivi des paroles synchronisées en direct dans le Dashboard Web.
- ⚡ **Backend Go (Gin)** : Moteur haute performance gérant les files d'attente et WebSockets temps réel.
- 📱 **Synchronisation iOS & Windows** : Intégration P2P via Syncthing pour injecter directement les sons dans l'app Musique / iTunes sans câble.
- 🐳 **Docker Native** : Déploiement en 1 seule commande avec Docker Compose.

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
- 🎨 **Dashboard Web (Next.js)** : `http://localhost:3000`
- ⚙️ **API Engine (Go)** : `http://localhost:8080`
- 🔄 **Console Syncthing UI** : `http://localhost:8384`

---

## 🛠️ Stack Technique

| Composant | Technologie |
| :--- | :--- |
| **Frontend** | Next.js 14, TypeScript, Tailwind CSS |
| **Backend** | Go (Gin Framework), WebSockets |
| **Downloader** | `spotdl` CLI (Python 3.11 + FFmpeg) |
| **Sync Engine** | Syncthing P2P |
| **Container** | Docker & Docker Compose |

---

## 💻 Développement Local

### Backend (Go)
```bash
cd backend
go run main.go
```

### Frontend (Next.js)
```bash
cd frontend
npm install
npm run dev
```

---

<p center>
Fait avec ❤️ par <b>son<span style="color: #f43f5e;">ephe</span></b>
</p>
