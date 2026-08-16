# 🪟 Configuration Syncthing sur Windows

Pour que les musiques téléchargées sur votre VPS arrivent automatiquement sur votre iPhone depuis un PC Windows, l'app musicale (iTunes / Musique) possède un **dossier magique natif**.

---

## 🚀 Étapes de configuration (5 minutes)

### 1. Installer l'app musicale & Syncthing sur Windows
* Installez **iTunes** (ou l'app **Musique**) pour Windows.
* Téléchargez et installez **Syncthing** (ou [SyncthingTray](https://github.com/Martchus/syncthingtray)).

### 2. Configurer le dossier de destination Syncthing sur Windows
Dans l'interface Web de Syncthing (`http://localhost:8384`), acceptez le dossier partagé `/downloads` en provenance de votre VPS.

Définissez le chemin du dossier local de destination exactement sur le dossier iTunes natif :
`C:\Users\<VotreNomUtilisateur>\Music\iTunes\iTunes Media\Ajouter automatiquement à iTunes`  
*(Ou `C:\Users\<VotreNomUtilisateur>\Music\iTunes\iTunes Media\Automatically Add to iTunes` selon la langue).*

### 3. La magie opère !
1. Dès que vous lancez un téléchargement depuis l'interface **soneph** sur le VPS, les fichiers MP3 + LRC arrivent dans ce dossier sur votre PC.
2. **L'app détecte immédiatement le fichier**, le lit, l'intègre proprement dans la bibliothèque avec sa pochette et ses métadonnées, et déplace le fichier dans sa structure propre.
3. Si l'option **Synchroniser la bibliothèque (iCloud)** est cochée dans les préférences, le son est uploadé sur iCloud et disponible **dans les 10 secondes sur votre iPhone** ! 📲✨
