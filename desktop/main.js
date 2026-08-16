// ── soneph desktop (macOS) ──────────────────────────────────────────────
// Enveloppe native autour du serveur Go + du frontend Vite.
//
// Au lancement :
//   1. cherche le binaire du serveur (Resources/bin en build, backend/bin
//      en dev) et le démarre sur un port libre,
//   2. ouvre une fenêtre macOS native (traffic lights intégrés) pointant
//      vers http://localhost:<port>,
//   3. tue le serveur à la fermeture.
//
// Dev :  SONEPH_DEV_URL=http://localhost:5173 npm start  → charge le dev
//        server Vite au lieu de démarrer un serveur embarqué.

const { app, BrowserWindow, Menu, shell, dialog } = require("electron");
const { spawn } = require("child_process");
const net = require("net");
const http = require("http");
const path = require("path");
const fs = require("fs");
const os = require("os");

let serverProc = null;
let mainWindow = null;

const DEFAULT_PORT_START = 8080;

// ── Binaire du serveur Go ───────────────────────────────────────────────
function findServerBinary() {
  const candidates = [
    process.env.SONEPH_SERVER,
    process.resourcesPath && path.join(process.resourcesPath, "bin", "soneph-server"),
    path.join(__dirname, "..", "backend", "bin", "soneph-server"),
    path.join(__dirname, "..", "..", "backend", "bin", "soneph-server"),
    path.join(__dirname, "..", "backend", "soneph-server"),
  ];
  for (const c of candidates) {
    if (c && fs.existsSync(c)) return c;
  }
  return null;
}

// Un port est « libre » si on ne peut pas s'y connecter. (Tenter de s'y
// binder est trompeur : sur macOS, se lier à 127.0.0.1 peut réussir même
// quand un serveur écoute déjà sur *:port en double-stack.)
function isPortFree(port) {
  return new Promise((resolve) => {
    const sock = net.connect({ port, host: "127.0.0.1" });
    sock.once("connect", () => {
      sock.destroy();
      resolve(false);
    });
    sock.once("error", () => resolve(true));
  });
}

async function pickFreePort(start) {
  for (let p = start; p < start + 30; p++) {
    if (await isPortFree(p)) return p;
  }
  return null;
}

function waitForServer(url, timeoutMs) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const tryOnce = () => {
      const req = http.get(url, (res) => {
        res.resume();
        resolve();
      });
      req.on("error", () => {
        if (Date.now() - start > timeoutMs) reject(new Error("timeout"));
        else setTimeout(tryOnce, 300);
      });
      req.setTimeout(2000, () => req.destroy());
    };
    tryOnce();
  });
}

async function startServer(port) {
  const bin = findServerBinary();
  if (!bin) {
    dialog.showErrorBox(
      "soneph",
      "Binaire du serveur introuvable.\n\nLance d'abord :  make build  (ou ./desktop/build.sh)"
    );
    return null;
  }

  // Dossier de musique par défaut : ~/Music/soneph (créé si besoin). En
  // app packagée le cwd n'est pas fiable, on force toujours DOWNLOAD_DIR.
  const downloads = process.env.DOWNLOAD_DIR || path.join(os.homedir(), "Music", "soneph");
  fs.mkdirSync(downloads, { recursive: true });

  // Serveur local uniquement : on retire tout token hérité de l'environnement.
  const env = { ...process.env, DOWNLOAD_DIR: downloads, PORT: String(port) };
  delete env.SONEPH_TOKEN;

  serverProc = spawn(bin, [], { env, stdio: ["ignore", "pipe", "pipe"] });
  serverProc.stdout.on("data", (d) => console.log("[server]", String(d).trim()));
  serverProc.stderr.on("data", (d) => console.error("[server]", String(d).trim()));

  // Si le binaire meurt pendant le démarrage (port pris, etc.), échouer vite.
  const died = new Promise((_, reject) =>
    serverProc.once("exit", () => reject(new Error("server exited")))
  );

  try {
    await Promise.race([
      waitForServer(`http://127.0.0.1:${port}/api/settings`, 15000),
      died,
    ]);
    return port;
  } catch (err) {
    dialog.showErrorBox(
      "soneph",
      `Le serveur n'a pas démarré sur le port ${port}.\n\n${err.message}`
    );
    return null;
  }
}

function stopServer() {
  if (serverProc && !serverProc.killed) {
    serverProc.kill("SIGTERM");
    serverProc = null;
  }
}

// ── Fenêtre & menu ──────────────────────────────────────────────────────
function createWindow(url) {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 960,
    minHeight: 600,
    backgroundColor: "#161618",
    titleBarStyle: "hiddenInset", // traffic lights flottants, look natif moderne
    trafficLightPosition: { x: 16, y: 18 },
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  mainWindow.loadURL(url);

  // Les liens externes s'ouvrent dans le navigateur par défaut.
  mainWindow.webContents.setWindowOpenHandler(({ url: u }) => {
    if (u.startsWith("http")) shell.openExternal(u);
    return { action: "deny" };
  });

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

function buildMenu() {
  const isMac = process.platform === "darwin";
  const template = [
    ...(isMac
      ? [
          {
            label: app.name,
            submenu: [
              { role: "about" },
              { type: "separator" },
              { role: "hide" },
              { role: "hideOthers" },
              { role: "unhide" },
              { type: "separator" },
              { role: "quit" },
            ],
          },
        ]
      : []),
    {
      label: "Modifier",
      submenu: [
        { role: "undo" },
        { role: "redo" },
        { type: "separator" },
        { role: "cut" },
        { role: "copy" },
        { role: "paste" },
        { role: "selectAll" },
      ],
    },
    {
      label: "Affichage",
      submenu: [
        { role: "reload" },
        { role: "forceReload" },
        { role: "toggleDevTools" },
        { type: "separator" },
        { role: "togglefullscreen" },
      ],
    },
    {
      label: "Fenêtre",
      submenu: [{ role: "minimize" }, { role: "zoom" }, { role: "close" }],
    },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}

// ── Cycle de vie ────────────────────────────────────────────────────────
const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on("second-instance", () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
    }
  });

  app.whenReady().then(async () => {
    app.setName("Soneph");
    buildMenu();

    const devUrl = process.env.SONEPH_DEV_URL;
    let url = devUrl;
    if (!devUrl) {
      const port = await pickFreePort(DEFAULT_PORT_START);
      if (!port) {
        dialog.showErrorBox("soneph", "Aucun port libre trouvé.");
        app.quit();
        return;
      }
      const started = await startServer(port);
      if (!started) {
        app.quit();
        return;
      }
      url = `http://127.0.0.1:${port}`;
    }

    createWindow(url);

    app.on("activate", () => {
      if (BrowserWindow.getAllWindows().length === 0) createWindow(url);
    });
  });

  app.on("window-all-closed", () => {
    stopServer();
    if (process.platform !== "darwin") app.quit();
  });

  app.on("before-quit", stopServer);
  app.on("will-quit", stopServer);
}
