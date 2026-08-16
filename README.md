# 🎵 son<span text-color="rose">ephe</span> — High Quality Music Downloader & Auto-Sync

> **soneph** *(son + ephe)* est une plateforme moderne et ultra-rapide d'automatisation de téléchargement de musique (320kbps + métadonnées ID3v2 + paroles synchronisées `.lrc`) et de synchronisation transparente P2P vers tes apps de lecture (Musique / iPhone).

---

## ✨ Features principales

- 🎧 **Qualité 320 kbps HD** : Extraction automatique avec tags complets (Pochette, Artiste, Album, Paroles LRC).
- 🎤 **Lecteur Karaoké intégré** : Suivi des paroles synchronisées en direct dans le Dashboard Web.
- ❤️ **Likes & Accueil** : Cœurs sur chaque morceau, vue **Accueil** avec dernières écoutes, top morceaux et favoris (historique persistant côté serveur).
- 🎵 **Playlists** : Création, ajout/retrait de morceaux et lecture en boucle depuis la bibliothèque.
- 🪟 **Interface Liquid Glass** : Effet de verre liquide (réfraction + flou) via `liquid-glass-react` sur le lecteur et la barre d'import.
- ⚡ **Backend Go (Gin)** : Moteur haute performance gérant les files d'attente et WebSockets temps réel.
- 📱 **Synchronisation iOS & Windows** : Intégration P2P via Syncthing pour injecter directement les sons dans tes apps de lecture sans câble.
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
| **Frontend** | Vite + React 19, TypeScript, Tailwind CSS, `liquid-glass-react` (SPA 100 % client, embarquée dans le binaire Go) |
| **Backend** | Go (Gin Framework), WebSockets, `go:embed` pour servir le frontend |
| **Downloader** | Moteur de téléchargement Python 3.11 + FFmpeg |
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
| `SONEPH_WORKERS` | `4` | Nombre de processus du moteur en parallèle (un par URL dans la file). Trop élevé → rate limiting des plateformes. |
| `SONEPH_THREADS` | `6` | Chansons téléchargées en parallèle par processus du moteur. |
| `SONEPH_ENGINE` | *(vide)* | Remplace le binaire du moteur de téléchargement (utile si tu l'installes sous un autre nom). |
| `DOWNLOAD_DIR` | `./downloads` | Dossier de destination (en Docker : `/app/downloads`). |
| `SONEPH_TOKEN` | *(vide)* | Si défini, **protège toute l'API** : chaque requête `/api/*` (et le WebSocket) doit présenter le token. |
| `SONEPH_HISTORY_FILE` | `~/.config/soneph/history.json` | Fichier d'historique d'écoute (dernières écoutes + top). |
| `SONEPH_LIKES_FILE` | `~/.config/soneph/likes.json` | Fichier des morceaux aimés. |
| `LOG_FORMAT` | `text` | `json` pour des logs structurés exploitables par un outil. |

> ⚡ Les paroles sont désormais récupérées **en arrière-plan** après le téléchargement : l'audio arrive vite, les `.lrc` suivent sans bloquer la file d'attente.

> 💾 La file d'attente est **persistée** (`queue.json`) : si le backend redémarre, les téléchargements en cours sont re-filés automatiquement.

### 🔒 Sécuriser l'API (important si tu exposes le serveur)

L'API est **ouverte par défaut** (mode local). Dès que ton backend est accessible depuis l'extérieur (VPS, LAN), protège-le :

1. **Token** : définis `SONEPH_TOKEN` (dans le `.env` du compose, ou l'env du process). Toute requête `/api/*` devra alors passer par `Authorization: Bearer <token>` (ou `?token=` pour le WebSocket). Dans l'UI → **Sync & Réglages → API Token**, pour enregistrer le token dans ton navigateur.
2. **HTTPS** : derrière un reverse proxy. Exemple minimal avec Caddy :

```caddyfile
soneph.example.com {
    reverse_proxy localhost:8080
}
```

```bash
# Docker : monte un volume pour que Caddy gère les certificats
caddy run --config Caddyfile
```

3. **Rate limiting** : déjà actif côté API (120 req/min/IP) — une protection de base contre le bourrage.

> 💡 La page web elle-même reste publique (elle ne fait rien sans token) ; tu peux aussi la protéger entièrement avec l'auth basic de Caddy si tu préfères.

### Playlists 🎧

Une section **Playlists** dans la barre latérale permet de créer des playlists, d'y ajouter n'importe quel morceau (bouton **+** sur une ligne de la bibliothèque), de les écouter dans l'ordre (bouton **Play All**) et de les supprimer. Les playlists sont stockées en JSON côté serveur (`~/.config/soneph/playlists/`) et survivent aux redémarrages.

### Sections de l'UI

- **Toutes les musiques** : toute la bibliothèque (recherche, tri, lecture)
- **Playlists** : tes playlists
- **Téléchargements** : file d'attente, progression, morceaux récents et échecs
- **Paroles** : gestion des paroles synchronisées · **Sync & Réglages** : auto-import + réglages + token

### Réglages depuis l'UI

La vue **Sync & Réglages** (barre latérale) permet de :
- démarrer / arrêter l'**auto-import** (watcher macOS, sans doublon),
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

### Makefile
```bash
make dev        # = scripts/dev.sh (backend + Vite)
make build      # frontend embarqué + binaire Go dans backend/bin/
make test       # tests Go du backend
make vet
```

> 💡 Pour builder le frontend dans le binaire Go (comme en production) : `make build` (ou `cd frontend && npm run build:go`) — copie `dist/` vers `backend/web/dist/` que le backend embarque via `go:embed`.

### 🧪 Tests
```bash
cd backend && go test ./...
```
Couvre : le parsing de la sortie du moteur de téléchargement (le point fragile), le scanner de bibliothèque (avec la protection anti-`../`), la persistance/reprise de la file d'attente, l'auth token + rate limiting, et la config.

### ⚠️ Nettoyage git — Syncthing
Les fichiers runtime de Syncthing (`syncthing_config/index-v2/main.db`, `syncthing.lock`) ne devraient **jamais** être commités — ils bougent à chaque sync et polluent l'historique. Si tu les as déjà commités :

```bash
git rm -r --cached syncthing_config   # retire de git, garde les fichiers sur disque
# puis commit le .gitignore + cette suppression
```

Ils sont désormais ignorés par `.gitignore`.

---

<p center>
Fait avec ❤️ par <b>son<span style="color: #f43f5e;">ephe</span></b>
</p>
