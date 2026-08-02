# 🏗️ soneph — Architecture Complete & Guide de Déploiement

soneph est une solution clé en main d'automatisation de téléchargement de musique  en haute qualité (320kbps + métadonnées + paroles synchronisées `.lrc`) et de synchronisation transparente vers l'app Musique / iTunes sur iPhone via Syncthing.

---

## 🌟 Stack Technique

- **Frontend** : Next.js 14 (App Router) + TypeScript + Tailwind CSS (Style  Dark).
- **Backend** : Go (Gin Gonic) + WebSockets pour la progression en temps réel.
- **Téléchargeur CLI** : `spotdl` (Python + FFmpeg).
- **Synchronisation P2P** : Syncthing.
- **Déploiement** : Docker & Docker Compose.

---

## 🚀 Déploiement Rapide sur VPS

### 1. Cloner / Copier les fichiers sur le VPS
```bash
git clone <votre-repo>
cd ephelstudio
```

### 2. Lancer l'infrastructure Docker
```bash
docker compose up -d --build
```

Vos services seront disponibles aux adresses suivantes :
- 🎨 **Interface Dashboard soneph** : `http://<IP-DE-VOTRE-VPS>:3000`
- ⚙️ **Backend Go API & WebSockets** : `http://<IP-DE-VOTRE-VPS>:8080`
- 🔄 **Console Syncthing Web UI** : `http://<IP-DE-VOTRE-VPS>:8384`

---

## 📲 Liaison Syncthing & Import Automatique (Mac / Windows)

### 1. Appairer Syncthing (VPS ➔ Ordi)
1. Ouvrez l'interface Syncthing de votre VPS (`http://<IP-DE-VOTRE-VPS>:8384`).
2. Récupérez son **ID d'appareil** (Menu *Actions* > *Afficher l'ID*).
3. Ouvrez **Syncthing** sur votre Mac/PC et ajoutez cet ID d'appareil.
4. Sur le VPS, partagez le dossier `/downloads` avec votre ordi.

### 2. Configurer l'Auto-Import l'app Musique / iTunes

####  Sur Mac (l'app Musique)
1. Associez le dossier Syncthing à un dossier local (ex: `~/Music/SpotSync`).
2. Ouvrez l'application **Configuration des actions de dossier** sur macOS.
3. Attachez le script AppleScript fourni : [`scripts/import_to_music.scpt`](file:///Users/ephe/Desktop/dev/ephelstudio/scripts/import_to_music.scpt).
4. Dès qu'un fichier arrive, il est injecté dans l'app Musique et synchronisé sur iCloud pour apparaître sur votre iPhone.

#### 🪟 Sur Windows (iTunes)
1. Dans Syncthing sur Windows, définissez le dossier de destination vers :  
   `C:\Users\<VotreNomUtilisateur>\Music\iTunes\iTunes Media\Ajouter automatiquement à iTunes`
2. iTunes se charge du reste automatiquement !

---

## 🧪 Tests locaux & Débogage

Pour exécuter le backend Go localement :
```bash
cd backend
go run main.go
```

Pour exécuter le frontend Next.js localement :
```bash
cd frontend
npm install
npm run dev
```
