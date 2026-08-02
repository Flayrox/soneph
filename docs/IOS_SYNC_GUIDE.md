# 📱 Guide de Synchronisation iPhone — soneph

Ce guide récapitule toutes les méthodes gratuites et payantes pour transférer et écouter vos musiques téléchargeables via soneph avec pochettes, métadonnées 320kbps et paroles synchronisées (.LRC / ID3v2.3) sur iPhone.

---

## 🍏 Méthode 1 : Synchronisation Native macOS Finder (100% Gratuite — Sans Abonnement)

C'est la méthode recommandée sur Mac pour mettre vos musiques directement dans l'application **Musique** native d'iOS sans payer d'abonnement l'app Musique.

1. **Connexion** : Branchez votre iPhone à votre Mac via câble USB (ou activez *"Activer la synchronisation Wi-Fi"* lors du 1er branchement).
2. **Finder** : Ouvrez le **Finder** sur macOS et cliquez sur votre **iPhone** dans la barre latérale gauche.
3. **Onglet Musique** :
   - Allez dans l'onglet **Musique**.
   - Cochez **Synchroniser la musique sur l'iPhone**.
   - Sélectionnez vos playlists ou dossiers d'artistes téléchargeurs dans `downloads/`.
4. **Appliquer** : Cliquez sur **Appliquer**.

> 💡 **Résultat** : Les fichiers MP3 encodés en **ID3v2.3** avec paroles propres et synchronisées sont transférés dans l'app Musique native de votre iPhone.

---

## 🌐 Méthode 2 : Application Web Progressive (PWA) sur Safari iOS

Accédez à votre serveur soneph (en local ou sur votre VPS) depuis le navigateur Safari de votre iPhone :

1. Ouvrez **Safari** sur iPhone et entrez l'adresse de votre serveur (ex: `http://192.168.x.x:3000` ou `http://vps-ip:3000`).
2. Appuyez sur le bouton **Partager** (icône carré avec flèche vers le haut dans Safari).
3. Sélectionnez **Sur l'écran d'accueil** *(Add to Home Screen)*.

> 💡 **Résultat** : soneph devient une application native sur l'écran d'accueil de votre iPhone, vous permettant de lancer les téléchargements et d'utiliser le **Lecteur Karaoké avec paroles synchronisées** en direct.

---

## 📲 Méthode 3 : Applications iOS Gratuites sans câble (VLC / Documents)

### A. VLC for Mobile (100% Gratuit mondialement)
1. Téléchargez **[VLC for Mobile](https://apps.apple.com/app/vlc-for-mobile/id650377962)** sur l'App Store.
2. Dans VLC sur iPhone, ouvrez le menu et activez **Partage via Wi-Fi**.
3. Depuis votre ordinateur, ouvrez l'adresse Web affichée par VLC et glissez-déposez vos morceaux MP3.

### B. Documents by Readdle
1. Téléchargez **[Documents by Readdle](https://apps.apple.com/app/documents-file-reader-browser/id364901807)**.
2. Utilisez le transfert Wi-Fi intégré (`docstransfer.com`) pour envoyer vos MP3 et fichiers `.lrc` associés.

---

## ☁️ Méthode 4 : Synchronisation iCloud l'app Musique (Si abonnement l'app Musique)

Si vous possédez un abonnement l'app Musique ou iTunes Match :
1. Sur **Mac** : Ouvrez **Musique** > **Réglages** > **Général** > Cochez **Synchroniser la bibliothèque**.
2. Sur **iPhone** : Ouvrez **Réglages** > **Musique** > Cochez **Synchroniser la bibliothèque**.

> 💡 Dès qu'un morceau est importé sur macOS, iCloud l'envoie automatiquement sur votre iPhone en moins de 15 secondes.
