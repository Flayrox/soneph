import React, { createContext, useContext, useEffect, useState } from "react";

export type Lang = "fr" | "en";

const STORAGE_KEY = "soneph_lang";

// Clés = texte anglais (défaut). fr = traduction. Si une clé manque,
// t() renvoie la clé elle-même → l'anglais par défaut, jamais de trou.
const dict: Record<Lang, Record<string, string>> = {
  en: {},
  fr: {
    // ── Navigation ──
    Songs: "Sons",
    "All Music": "Toutes les musiques",
    Music: "Musique",
    Lyrics: "Paroles",
    "Recently Added": "Ajouts récents",
    Artists: "Artistes",
    Albums: "Albums",
    Pins: "Épinglés",
    Downloads: "Téléchargements",
    "Sync & Settings": "Sync & Réglages",
    Home: "Accueil",
    Radio: "Radio",
    Library: "Bibliothèque",
    Playlists: "Playlists",
    "All Playlists": "Toutes les playlists",
    "Favorite Songs": "Morceaux favoris",
    "Active Syncs": "Synchronisations actives",
    "Auto-Sync": "Synchro auto",
    Search: "Rechercher",

    // ── Header ──
    "Paste Link...": "Colle un lien…",
    Import: "Importer",
    "Download Preferences": "Préférences de téléchargement",
    "Audio Quality": "Qualité audio",
    "320 kbps (High Quality)": "320 kbps (Haute qualité)",
    "192 kbps (Standard)": "192 kbps (Standard)",
    "128 kbps (Compact)": "128 kbps (Compact)",
    "Import Order": "Ordre d'import",
    "Newest Added First": "Les plus récents d'abord",
    "Original Playlist Order": "Ordre original de la playlist",
    "Click to view active download queue details": "Voir la file de téléchargement",
    "Syncing...": "Synchronisation…",
    Language: "Langue",

    // ── File d'attente ──
    "Playlist Download Queue": "File de téléchargement",
    "active worker(s)": "worker(s) actif(s)",
    queued: "en attente",
    "tracks done": "morceaux terminés",
    "Downloading Now": "Téléchargement en cours",
    "Downloaded Songs": "Morceaux téléchargés",
    "Up Next in Queue": "File d'attente",
    Queued: "En attente",
    "Failed Imports": "Imports échoués",
    "All tracks auto-sync into the macOS Music app & Finder":
      "Tous les morceaux se synchronisent automatiquement avec l'app Musique (macOS)",
    "Now Downloading": "Téléchargement en cours",
    "of": "sur",
    songs: "morceaux",
    "In queue...": "En attente…",
    Downloaded: "Téléchargé",
    Synced: "Synchronisé",

    // ── File M4 (jobs) ──
    "Job Queue": "File des tâches",
    "Fast Filter": "Filtre rapide",
    Download: "Téléchargement",
    "Scans the link against your library": "Analyse le lien face à ta bibliothèque",
    "Downloads tracks from the link": "Télécharge les morceaux du lien",
    Running: "En cours",
    Done: "Terminé",
    "Retry in": "Nouvelle tentative dans",
    attempts: "tentatives",

    // ── Liste des morceaux ──
    Title: "Titre",
    Album: "Album",
    Added: "Ajouté",
    "Lyrics Sync": "Paroles",
    Time: "Durée",
    Downloading: "Téléchargement",
    "Syncing now": "Synchronisation…",
    "Import Queue": "File d'import",
    Text: "Texte",
    Missing: "Manquantes",
    "No tracks imported yet": "Aucun morceau importé",
    "Paste a playlist or track URL above to start syncing":
      "Colle un lien (playlist ou morceau) ci-dessus pour commencer",
    "Queued in download engine...": "En attente dans le moteur de téléchargement…",
    Single: "Single",
    "Just now": "À l'instant",
    "m ago": "min",
    "h ago": "h",

    // ── Gestion des paroles ──
    All: "Tous",
    "Plain": "Texte",
    "Plain Text": "Texte brut",
    "Status": "État",
    "View Lyrics →": "Voir les paroles →",
    "No tracks match filter": "Aucun morceau ne correspond",
    "Sync Lyrics": "Synchroniser les paroles",
    "Upgrading...": "Mise à jour…",
    "Search...": "Rechercher…",

    // ── Panneau paroles ──
    "Synced Lyrics": "Paroles synchronisées",
    "Scanning library...": "Analyse de la bibliothèque…",
    "Total songs": "Morceaux au total",
    "Without synced lyrics": "Sans paroles synchronisées",
    "Plain text to upgrade": "Texte brut à synchroniser",
    "lyrics added": "paroles ajoutées",
    "not found": "introuvables",
    Hide: "Masquer",
    Show: "Afficher",
    details: "les détails",
    Scan: "Analyser",
    "Retry All": "Réessayer tout",
    "Running…": "En cours…",
    "Starting…": "Démarrage…",

    // ── Drawer paroles ──
    "Loading lyrics...": "Chargement des paroles…",
    "No lyrics available": "Pas de paroles disponibles",
    "No .lrc file found for this track.": "Aucun fichier .lrc pour ce morceau.",
    "Fetch Lyrics": "Récupérer les paroles",
    "Searching...": "Recherche…",
    "Sync to Playback": "Resynchroniser",
    Pause: "Pause",
    Play: "Lecture",
    Copy: "Copier",
    Copied: "Copié",
    "Retrying...": "Nouvelle tentative…",
    Upgrade: "Mettre à niveau",
    "Synced (LRC)": "Synchronisées (LRC)",

    // ── Modal paroles ──
    "No Synced Lyrics File Available": "Aucune paroles synchronisées",
    "This song was downloaded without a .LRC synced lyrics file.":
      "Ce morceau a été téléchargé sans fichier .LRC synchronisé.",
    "Press ESC or click close to return to your library":
      "Appuie sur Échap ou ferme pour revenir à ta bibliothèque",

    // ── Toasts ──
    "Track / Playlist Import Started": "Import de morceau / playlist lancé",
    "Artist Discography Import Started": "Import de la discographie lancé",
    "Original Order": "Ordre original",
    "MP3 + Metadata + Clean Lyrics...": "MP3 + métadonnées + paroles…",
    "Download Complete": "Téléchargement terminé",
    "Audio ready — lyrics syncing in background.": "Audio prêt — paroles en cours de synchronisation.",
    "Import Error": "Erreur d'import",
    "Execution error": "Erreur d'exécution",
    "File Removed": "Fichier supprimé",
    "Removed \"{name}\" from storage.": "« {name} » supprimé du stockage.",
    "Delete Error": "Erreur de suppression",
    "Network Error": "Erreur réseau",
    "Unable to connect to Go backend": "Impossible de joindre le backend",
    "Action could not be completed": "Action impossible",
    "Failed to dispatch import": "Échec du lancement de l'import",
    "Error": "Erreur",

    // ── Sync & Réglages ──
    "Auto-Import into Music": "Auto-Import dans Musique",
    "Copies new files automatically into the Music app — no duplicates":
      "Copie automatiquement les nouveaux fichiers dans l'app Musique — sans doublon",
    "In progress": "En cours",
    Stopped: "Arrêté",
    Unavailable: "Indisponible",
    "Auto-import requires macOS with the Music app installed. On a server (VPS), distribution is handled by Syncthing — install the watcher on your Mac with the script":
      "L'auto-import nécessite macOS avec l'app Musique installée. Sur un serveur (VPS), la distribution est assurée par Syncthing — installe le watcher sur ton Mac avec le script",
    "Watched folder": "Dossier surveillé",
    "Music folder": "Dossier Musique",
    "Files imported": "Fichiers importés",
    "Watcher script": "Script du watcher",
    "Change": "Changer",
    "Library folder changed": "Dossier de la bibliothèque changé",
    Start: "Démarrer",
    Stop: "Arrêter",
    Refresh: "Actualiser",
    "Download settings": "Réglages de téléchargement",
    "Speed vs stability: too much parallelism triggers platform rate limiting":
      "Vitesse vs stabilité : trop de parallélisme déclenche le rate limiting des plateformes",
    "Parallel songs (threads)": "Chansons en parallèle (threads)",
    "Applies to next downloads (default 6)": "Appliqué aux prochains téléchargements (défaut 6)",
    "Parallel playlists (workers)": "Playlists en parallèle (workers)",
    "Applies at next start (default 4)": "Appliqué au prochain démarrage (défaut 4)",
    Save: "Enregistrer",
    "Loading…": "Chargement…",
    "Watcher started": "Watcher démarré",
    "Watcher stopped": "Watcher arrêté",
    "New files will be automatically imported into Music.":
      "Les nouveaux fichiers seront automatiquement importés dans Musique.",
    "Auto-import is disabled.": "L'auto-import est désactivé.",
    Failed: "Impossible",
    "Action denied": "Action refusée",
    "Action failed": "Action impossible",
    "Cannot reach the backend": "Impossible de contacter le backend",
    "Settings saved": "Réglages enregistrés",
    "Threads apply to next downloads; workers at next start.":
      "Threads appliqués aux prochains téléchargements ; workers au prochain démarrage.",
    "Save failed": "Échec de l'enregistrement",
    "Cannot save": "Impossible d'enregistrer",
    "Cannot join the backend": "Impossible de joindre le backend",

    // ── Clés supplémentaires ──
    "Processing playlist...": "Traitement de la playlist…",
    "Delete track": "Supprimer le morceau",
    "Time-Synced Karaoke LRC lyrics": "Paroles LRC synchronisées (karaoké)",
    "Plain text unsynced lyrics": "Paroles en texte non synchronisées",
    "No lyrics downloaded yet": "Pas encore de paroles",

    // ── Playlists ──
    Playlist: "Playlist",
    "New Playlist": "Nouvelle playlist",
    "Playlist Name": "Nom de la playlist",
    "Add to Playlist": "Ajouter à la playlist",
    "Remove from Playlist": "Retirer de la playlist",
    "No playlists yet": "Aucune playlist pour l'instant",
    "Play All": "Tout écouter",
    "Delete Playlist": "Supprimer la playlist",
    "Playlist created": "Playlist créée",
    "Playlist updated": "Playlist mise à jour",
    "to download": "à télécharger",
    "Playlist deleted": "Playlist supprimée",
    "Added to playlist": "Ajouté à la playlist",
    "Removed from playlist": "Retiré de la playlist",
    "This playlist is empty": "Cette playlist est vide",
    "Add tracks from the library with the + button":
      "Ajoute des morceaux depuis la bibliothèque avec le bouton +",
    "No downloads yet": "Aucun téléchargement pour l'instant",
    "Import a link above to start downloading": "Colle un lien ci-dessus pour commencer à télécharger",

    // ── Accueil & Likes ──
    "Recent listens": "Dernières écoutes",
    "Top tracks": "Morceaux les plus écoutés",
    "Liked tracks": "Morceaux aimés",
    "Recently played": "Récemment écoutés",
    "times": "fois",
    "plays": "lectures",
    "Liked": "Aimé",
    "Like": "Aimer",
    "Play something to start building your history":
      "Écoute un morceau pour commencer à remplir ton historique",
    "No likes yet — tap the heart on a track": "Aucun morceau aimé — clique sur le cœur d'un morceau",
    "Welcome back": "Bon retour",
    "Your library at a glance": "Ta bibliothèque en un coup d'œil",
    "Total tracks": "Morceaux au total",
    "Tracks liked": "Morceaux aimés",
    "Track added to likes": "Morceau ajouté aux favoris",
    "Track removed from likes": "Morceau retiré des favoris",

    // ── Sélection de morceaux (desktop) ──
    selected: "sélectionnés",
    Close: "Fermer",
    "Play Next": "Lire ensuite",
    "Added to queue": "Ajouté à la file",
    track: "morceau",
    tracks: "morceaux",
    "will play next": "sera joué ensuite",
    will: "seront joués ensuite",

    // ── Lecteur : file d'attente, shuffle, repeat ──
    Shuffle: "Aléatoire",
    "Repeat Off": "Répétition désactivée",
    "Repeat All": "Répéter tout",
    "Repeat One": "Répéter un morceau",
    Queue: "File d'attente",
    "Now Playing": "En cours de lecture",
    "Up Next": "À suivre",
    "Up next": "À suivre",
    "End of queue": "Fin de la file",
    "End of queue — will loop": "Fin de la file — boucle active",

    // ── Recherche dans les playlists ──
    "Search tracks to add…": "Chercher des morceaux à ajouter…",
    "No results": "Aucun résultat",
    "More from": "Plus de",
    "More from album": "Plus de l'album",
    "Search your library to add tracks to this playlist":
      "Cherche dans ta bibliothèque pour ajouter des morceaux à cette playlist",
    "View Artist": "Voir l'artiste",
    "View Album": "Voir l'album",
    "Remove from queue": "Retirer de la file",
    "Drag to resize — double-click to reset": "Tire pour redimensionner — double-clic pour réinitialiser",
    "Move sidebar to the right": "Déplacer la sidebar à droite",
    "Move sidebar to the left": "Déplacer la sidebar à gauche",

    // ── Artistes, albums & pins ──
    Artist: "Artiste",
    Pinned: "Épinglés",
    Pin: "Épingler",
    Unpin: "Désépingler",
    Unknown: "Inconnu",
    "No artists yet": "Aucun artiste pour l'instant",
    "No albums yet": "Aucun album pour l'instant",
    "Import music to build your library": "Importe de la musique pour construire ta bibliothèque",
    "No songs by this artist": "Aucun morceau de cet artiste",
    "No songs on this album": "Aucun morceau sur cet album",

    // ── Modules & Marketplace ──
    Marketplace: "Marketplace",
    Modules: "Modules",
    Plugins: "Plugins",
    "Choose your plugins": "Choisis tes plugins",
    "Enable the plugins you want. You can change this anytime in the Marketplace.":
      "Active les plugins que tu veux. Tu pourras les changer à tout moment dans la Marketplace.",
    "All plugins are optional — the player, library and lyrics stay always on.":
      "Tous les plugins sont optionnels — le lecteur, la bibliothèque et les paroles restent toujours actifs.",
    "Choose your modules": "Choisis tes modules",
    "Welcome to soneph": "Bienvenue sur soneph",
    "Enable the features you want. You can change this anytime in the Marketplace.":
      "Active les fonctionnalités que tu veux. Tu pourras les changer à tout moment dans la Marketplace.",
    Continue: "Continuer",
    Enabled: "Activé",
    Disabled: "Désactivé",
    "Import Module": "Module d'import",
    "Import Module Desc": "Télécharge musique, playlists et discographies d'artistes depuis un lien.",
    "Stats Module": "Module Stats",
    "Stats Module Desc": "Statistiques façon Wrapped : temps d'écoute, top artistes, écoutes par jour.",
    "Listening time": "Temps d'écoute",
    "Total plays": "Écoutes au total",
    "Top artists": "Top artistes",
    "Plays per day": "Écoutes par jour",
    hours: "heures",
    minutes: "minutes",
    "No stats yet": "Pas encore de stats",
    "Play something to fill in your stats": "Écoute un morceau pour remplir tes stats",
    "This module is disabled": "Ce module est désactivé",
    "Re-enable it from the Marketplace": "Réactive-le depuis la Marketplace",
    "Top artist": "Top artiste",
    "Changes apply immediately — no restart needed":
      "Les changements s'appliquent immédiatement — pas besoin de redémarrer",
    "All modules are optional — the player, library and lyrics stay always on.":
      "Tous les modules sont optionnels — le lecteur, la bibliothèque et les paroles restent toujours actifs.",

    // ── Token API ──
    "API Token (optional)": "Token API (optionnel)",
    "Protect the API with a token. Must match the server SONEPH_TOKEN env var.":
      "Protège l'API avec un token. Doit correspondre à la variable SONEPH_TOKEN du serveur.",
    "Token saved": "Token enregistré",
    "Token stored in this browser only.": "Token stocké dans ce navigateur uniquement.",

    // ── Plugin d'exemple ──
    "Example Plugin": "Plugin d'exemple",
    "Example Plugin Desc": "Le plus petit plugin complet : vue, badge et action. Copie-le pour créer le tien (docs/plugins.md).",
    Example: "Exemple",
    "Everything below reads from the { app } prop": "Chaque ligne ci-dessous lit le contexte « app »",
    "Read from app": "Lire depuis app",
    "Call app actions": "Appeler les actions de l'app",
    "Play first track": "Lire le premier morceau",
    "Like second track": "Aimer le second morceau",
    "Jump to library": "Aller à la bibliothèque",
    "Show a toast": "Afficher une notification",
    "First track": "Premier morceau",
    "How this works": "Comment ça marche",
    "This view is declared in src/plugins/example.ts with its component and a badge. Disable it anytime from the Marketplace.":
      "Cette vue est déclarée dans src/plugins/example.ts avec son composant et son badge. Désactive-la à tout moment depuis la Marketplace.",
    "Open the Marketplace": "Ouvrir la Marketplace",
    "This toast came from a plugin action": "Ce toast vient d'une action de plugin",
    "Current nav": "Navigation actuelle",

    // ── Dédoublonnage ──
    "Deduplicate library": "Dédoublonner la bibliothèque",
    "Apple Music rips, (1) copies and same-title tracks are grouped — keep one, delete the rest.":
      "Les rip Apple Music, copies « (1) » et morceaux au même titre sont regroupés — garde-en un, supprime le reste.",
    "Scan duplicates": "Scanner les doublons",
    "Scanning…": "Analyse en cours…",
    // ── Détails d'un morceau (clic droit) ──
    "More details": "Plus de détails",
    Details: "Détails",
    Quality: "Qualité",
    Duration: "Durée",
    "Album artist": "Artiste album",
    Year: "Année",
    Genre: "Genre",
    Track: "Piste",
    Disc: "Disque",
    Writers: "Auteurs",
    Producers: "Producteurs",
    Musicians: "Musiciens",
    Publisher: "Éditeur",
    Copyright: "Copyright",
    Source: "Source",
    "Open in Spotify": "Ouvrir sur Spotify",
    "Create playlist from link": "Créer la playlist depuis le lien (sons déjà sur disque)",
    "tracks added": "morceaux ajoutés",
    "No track from this link is on disk yet — download it first.":
      "Aucun morceau de ce lien n'est déjà sur disque — télécharge-le d'abord.",
    "Stats kept": "Stats conservées",
    "Stats transferred to": "Stats transférées vers",
    "stats kept on the kept copy": "stats conservées sur la copie gardée",

    "No duplicates found": "Aucun doublon trouvé 🎉",
    "duplicate group(s) found": "groupe(s) de doublons trouvé(s)",
    "Keep": "Garder",
    "Delete selected": "Supprimer la sélection",
    "Keep best": "Garder le meilleur",
    "duplicates": "doublons",
    "Duplicate": "Doublon",

    // ── Export playlists (iPhone / Syncthing) ──
    "Export playlists": "Exporter les playlists",
    "Writes one .m3u8 per playlist with relative paths — drop them in a Syncthing folder or on the iPhone via USB.":
      "Écrit un fichier .m3u8 par playlist avec des chemins relatifs — dépose-les dans un dossier Syncthing ou sur l'iPhone via USB.",
    "Export folder": "Dossier d'export",
    "Playlists exported": "Playlists exportées",
    "Playlist files written to": "Fichiers écrits dans",
    "Export": "Exporter",
    "Exporting…": "Export en cours…",
  },
};

interface I18nContextValue {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nContextValue>({
  lang: "en",
  setLang: () => {},
  t: (k: string) => k,
});

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => {
    if (typeof window !== "undefined") {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (stored === "fr" || stored === "en") return stored;
      // Défaut : suivre la langue du navigateur
      if (window.navigator.language?.toLowerCase().startsWith("fr")) return "fr";
    }
    return "en";
  });

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, lang);
    document.documentElement.lang = lang;
  }, [lang]);

  const setLang = (l: Lang) => setLangState(l);

  const t = (key: string, vars?: Record<string, string | number>) => {
    let s = dict[lang][key] ?? key;
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        s = s.split(`{${k}}`).join(String(v));
      }
    }
    return s;
  };

  return (
    <I18nContext.Provider value={{ lang, setLang, t }}>{children}</I18nContext.Provider>
  );
}

export function useI18n() {
  return useContext(I18nContext);
}

// Sélecteur de langue compact (FR | EN), placé dans le header.
export function LangToggle() {
  const { lang, setLang, t } = useI18n();
  return (
    <div className="flex items-center gap-0.5 bg-[#242428] border border-white/10 rounded-full p-0.5 text-[10px] font-bold select-none">
      <button
        onClick={() => setLang("fr")}
        title="Français"
        className={`px-2 py-1 rounded-full transition-colors ${
          lang === "fr" ? "bg-apple-pink text-white" : "text-apple-subtext hover:text-white"
        }`}
      >
        FR
      </button>
      <button
        onClick={() => setLang("en")}
        title="English"
        className={`px-2 py-1 rounded-full transition-colors ${
          lang === "en" ? "bg-apple-pink text-white" : "text-apple-subtext hover:text-white"
        }`}
      >
        EN
      </button>
    </div>
  );
}
