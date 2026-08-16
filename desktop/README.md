# soneph — App macOS

Enveloppe native (Electron) autour du serveur Go + du frontend Vite.

## Build

```bash
./build.sh          # ou : npm run build
```

Le script :
1. build le frontend Vite → `backend/web/dist` (embarqué dans le binaire Go),
2. compile le serveur Go → `backend/bin/soneph-server`,
3. génère l'icône (`resources/icon.icns`),
4. package l'app → `dist/Soneph-darwin-arm64/Soneph.app` avec le binaire Go
   dans `Contents/Resources/bin/` et l'icône appliquée.

Lance ensuite : `open "dist/Soneph-darwin-arm64/Soneph.app"`

## Comportement

- Au lancement, l'app démarre le serveur Go embarqué sur un **port libre**
  (8080 si dispo, sinon 8081, 8082…) et ouvre la fenêtre native dessus.
- **Musique** : `~/Music/soneph` par défaut (créé au premier lancement).
  Pour utiliser un autre dossier : `DOWNLOAD_DIR=/chemin/vers/musique open Soneph.app`
  (ou lance le binaire avec la variable d'env).
- **Port** : `SONEPH_SERVER=/chemin/vers/soneph-server` force un binaire précis.
- **Dev** : `SONEPH_DEV_URL=http://localhost:5173 npm start` charge le dev
  server Vite au lieu du serveur embarqué (lance d'abord `./scripts/dev.sh`).
- Le serveur est arrêté proprement à la fermeture de l'app.

## Notes

- Signature ad-hoc appliquée au build (pas de Gatekeeper pour un build local).
- Pour distribuer : il faudra une signature Developer ID + notarisation
  (`codesign` + `notarytool`) — voir docs quand on y arrive.
- Le futur remplacement : une app Swift native (WKWebView ou AppKit) — le
  backend Go + l'API REST restent identiques.
