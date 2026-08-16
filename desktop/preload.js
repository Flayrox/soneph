// Pont sécurisé entre la page web et le main process Electron.
// Expose uniquement ce dont l'UI a besoin (pas de Node dans le renderer).
const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("soneph", {
  /** Ouvre le sélecteur de dossier macOS. Renvoie le chemin choisi, ou null si annulé. */
  pickDownloadDir: () => ipcRenderer.invoke("pick-download-dir"),
});
