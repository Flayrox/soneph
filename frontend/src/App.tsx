import React, { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { Sidebar } from "@/components/Sidebar";
import { AppHeader } from "@/components/AppHeader";
import { TrackList } from "@/components/TrackList";
import { Player } from "@/components/Player";
import { LyricsModal } from "@/components/LyricsModal";
import { ToastContainer, ToastMessage } from "@/components/Toast";
import { LyricsDrawer } from "@/components/LyricsDrawer";
import { LyricsManagerView } from "@/components/LyricsManagerView";
import { PlaylistView } from "@/components/PlaylistView";
import { CollectionGrid } from "@/components/CollectionGrid";
import { CollectionDetail } from "@/components/CollectionDetail";
import { HomeView } from "@/components/HomeView";
import { usePins } from "@/pins";
import { MarketplaceView } from "@/components/MarketplaceView";
import { OnboardingView } from "@/components/OnboardingView";
import { PluginHostView } from "@/framework/PluginHostView";
import { usePlugins } from "@/framework/PluginProvider";
import type { PluginApp } from "@/framework/plugin.types";
import { useI18n } from "@/i18n";
import { apiFetch, wsUrl } from "@/api";
import type {
  DownloadedFile,
  DownloadTask,
  HistoryRecord,
  Playlist,
  PlaylistSummary,
  SearchTrack,
  TopTrack,
} from "@/types";

// The frontend is served from the same origin as the Go backend
// (Vite dev proxy in development, go:embed in production), so all
// API + WebSocket URLs are relative.
const API_URL = "/api";
const WS_URL = "/api/ws";

const jsonHeaders = { "Content-Type": "application/json" };

// Convertit un résultat de recherche FTS (ligne de la table tracks) en
// DownloadedFile pour les vues existantes de l'app.
function trackToFile(t: SearchTrack): DownloadedFile {
  const lyricsSynced = !!t.lyrics_synced;
  return {
    rel_path: t.path,
    file_name: t.path.split("/").pop(),
    title: t.title,
    artist: t.artist || "Unknown Artist",
    album: t.album || "Unknown Album",
    size_bytes: t.size_bytes,
    lyrics_type: lyricsSynced ? "synced" : t.lyrics_path ? "unsynced" : "none",
    has_lyrics: !!t.lyrics_path || lyricsSynced,
    mod_time: t.updated_at || t.added_at,
  };
}

export default function App() {
  const { t } = useI18n();
  const { isEnabled, configured, finishOnboarding } = usePlugins();
  const importEnabled = isEnabled("import");
  const statsEnabled = isEnabled("stats");
  const { pins, togglePin, isPinned } = usePins();
  const [tasks, setTasks] = useState<DownloadTask[]>([]);
  const [files, setFiles] = useState<DownloadedFile[]>([]);
  const [playlists, setPlaylists] = useState<PlaylistSummary[]>([]);
  const [playlistDetail, setPlaylistDetail] = useState<Playlist | null>(null);
  const [likes, setLikes] = useState<Set<string>>(new Set());
  const [recentHistory, setRecentHistory] = useState<HistoryRecord[]>([]);
  const [topTracks, setTopTracks] = useState<TopTrack[]>([]);
  const [activeFilter, setActiveFilter] = useState<string>("");
  const [activeNav, setActiveNav] = useState<string>("songs");
  const [sidebarRight, setSidebarRight] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.localStorage.getItem("soneph_sidebar_pos") === "right";
  });
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  // Right Side Lyrics Drawer State
  const [selectedTrackForDrawer, setSelectedTrackForDrawer] = useState<DownloadedFile | null>(null);
  const [isLyricsDrawerOpen, setIsLyricsDrawerOpen] = useState<boolean>(false);

  // Audio Playback State
  const [currentTrackPath, setCurrentTrackPath] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [currentTime, setCurrentTime] = useState<number>(0);

  // Playback queue: the ordered list of rel_paths currently being played.
  const queueRef = useRef<string[]>([]);
  const queueIndexRef = useRef<number>(-1);
  // Shuffle order (indices into queueRef) — regenerated on toggle / queue change.
  const shuffleOrderRef = useRef<number[]>([]);
  const [shuffle, setShuffle] = useState<boolean>(false);
  const [repeatMode, setRepeatMode] = useState<"off" | "all" | "one">("off");
  const [isQueueOpen, setIsQueueOpen] = useState<boolean>(false);	// Bumped on every queue mutation so the Player re-renders with the new queue.
	const [queueTick, setQueueTick] = useState<number>(0);
	// Ancienne clé localStorage (pré-M3) — utilisée pour la migration one-shot.
	const QUEUE_STORAGE_KEY = "soneph_queue_v1";

  // Display queue: resolve paths to files (recomputed each render).
  const queueTracks = useMemo(() => {
    void queueTick;
    return queueRef.current
      .map((p) => files.find((f) => f.rel_path === p))
      .filter((f): f is DownloadedFile => !!f);
  }, [files, queueTick]);

  // Synchronized Karaoke Lyrics State
  const [isLyricsOpen, setIsLyricsOpen] = useState<boolean>(false);
  const [lyricsRaw, setLyricsRaw] = useState<string | null>(null);

  const audioRef = useRef<HTMLAudioElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const jobResyncRef = useRef<number | null>(null);
  const lastScrobbledRef = useRef<string | null>(null);

  const getApiUrl = () => API_URL;

  const getWsUrl = () => WS_URL;

  const getStreamUrl = (relPath: string) => {
    return `${getApiUrl()}/stream?path=${encodeURIComponent(relPath)}`;
  };

  const fetchLyrics = async (relPath: string) => {
    try {
      const res = await apiFetch(`${getApiUrl()}/lyrics?path=${encodeURIComponent(relPath)}`);
      if (res.ok) {
        const data = await res.json();
        setLyricsRaw(data.lyrics || null);
      } else {
        setLyricsRaw(null);
      }
    } catch {
      setLyricsRaw(null);
    }
  };

  const addToast = (type: "success" | "error" | "info", title: string, message: string) => {
    const id = Math.random().toString(36).substring(2, 9);
    setToasts((prev) => [...prev, { id, type, title, message }]);
  };

  const dismissToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  };

  const fetchTasks = useCallback(async () => {
    try {
      const res = await apiFetch(`${getApiUrl()}/tasks`);
      if (res.ok) {
        const data = await res.json();
        setTasks(data.tasks || []);
      }
    } catch (err) {
      console.error("Error fetching tasks:", err);
    }
  }, []);

  const fetchFiles = useCallback(async () => {
    try {
      const res = await apiFetch(`${getApiUrl()}/downloads`);
      if (res.ok) {
        const data = await res.json();
        setFiles(data.files || []);
      }
    } catch (err) {
      console.error("Error fetching downloads:", err);
    }
  }, []);

  const fetchLikes = useCallback(async () => {
    try {
      const res = await apiFetch(`${getApiUrl()}/likes`);
      if (res.ok) {
        const data = await res.json();
        setLikes(new Set(data.likes || []));
      }
    } catch (err) {
      console.error("Error fetching likes:", err);
    }
  }, []);

  const fetchHistory = useCallback(async () => {
    try {
      const res = await apiFetch(`${getApiUrl()}/history/recent?limit=50`);
      if (res.ok) {
        const data = await res.json();
        setRecentHistory(data.history || []);
      }
    } catch (err) {
      console.error("Error fetching history:", err);
    }
    try {
      const res = await apiFetch(`${getApiUrl()}/history/top?limit=10`);
      if (res.ok) {
        const data = await res.json();
        setTopTracks(data.top || []);
      }
    } catch (err) {
      console.error("Error fetching top tracks:", err);
    }
  }, []);

  const fetchPlaylists = useCallback(async () => {
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists`);
      if (res.ok) {
        const data = await res.json();
        setPlaylists(data.playlists || []);
      }
    } catch (err) {
      console.error("Error fetching playlists:", err);
    }
  }, []);

  // Load a playlist's resolved tracks (used to open + refresh the current view).
  const loadPlaylist = useCallback(async (id: string) => {
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists/${id}`);
      if (res.ok) {
        const data = await res.json();
        setPlaylistDetail(data.playlist);
      }
    } catch {
      setPlaylistDetail(null);
    }
  }, []);

  // True once the initial restore has run — until then, never persist an
  // (empty) queue, or it would clobber the saved one on first mount.
  const restoredQueueRef = useRef(false);	// Restore the saved queue once the library has loaded. Depuis M3, la
	// source de vérité est le serveur (GET /api/queue) ; le localStorage
	// pré-M3 est migré une seule fois s'il reste des chemins valides.
	useEffect(() => {
		if (restoredQueueRef.current || files.length === 0) return;
		restoredQueueRef.current = true;
		const validPaths = new Set(files.map((f) => f.rel_path));
		void (async () => {
			let queue: string[] = [];
			let index = 0;
			try {
				const res = await apiFetch(`${getApiUrl()}/queue`);
				if (res.ok) {
					const parsed = await res.json();
					if (Array.isArray(parsed.queue)) {
						queue = parsed.queue.filter(
							(p: unknown) => typeof p === "string" && validPaths.has(p)
						);
						index =
							typeof parsed.index === "number" &&
							parsed.index >= 0 &&
							parsed.index < queue.length
								? parsed.index
								: 0;
					}
				}
			} catch {
				// hors-ligne : on retombe sur la copie locale pré-M3
			}

			// Migration one-shot : serveur vide, localStorage plein.
			let legacy: { queue: unknown; index: unknown } | null = null;
			try {
				const raw = window.localStorage.getItem(QUEUE_STORAGE_KEY);
				if (raw) legacy = JSON.parse(raw);
			} catch {
				legacy = null;
			}
			const legacyQueue = Array.isArray(legacy?.queue)
				? (legacy.queue as unknown[]).filter(
						(p: unknown) => typeof p === "string" && validPaths.has(p)
					)
				: [];
			if (queue.length === 0 && legacyQueue.length > 0) {
				queue = legacyQueue as string[];
				index =
					typeof legacy?.index === "number" &&
					legacy.index >= 0 &&
					legacy.index < queue.length
						? legacy.index
						: 0;
				try {
					window.localStorage.removeItem(QUEUE_STORAGE_KEY);
				} catch {
					// storage indisponible — sans effet
				}
			}

			if (queue.length > 0) {
				queueRef.current = queue;
				queueIndexRef.current = index;
			}
			// Toujours bump pour que la file (éventuellement restaurée) soit
			// repersistée côté serveur correctement.
			setQueueTick((v) => v + 1);
		})();
	}, [files]);

	// Persist the playback queue to the server on every mutation — débouncé
	// (500 ms) pour ne pas envoyer un PUT par avance de piste. La file reste
	// en mémoire comme cache optimiste ; le serveur est la source de vérité.
	useEffect(() => {
		if (!restoredQueueRef.current) return;
		const timer = window.setTimeout(() => {
			void (async () => {
				try {
					await apiFetch(`${getApiUrl()}/queue`, {
						method: "PUT",
						headers: jsonHeaders,
						body: JSON.stringify({
							queue: queueRef.current,
							index: queueIndexRef.current,
						}),
					});
				} catch {
					// hors-ligne : la file reste en mémoire, resync au prochain tick
				}
			})();
		}, 500);
		return () => window.clearTimeout(timer);
	}, [queueTick]);

  // Connect WebSockets
  useEffect(() => {
    fetchTasks();
    fetchFiles();
    fetchPlaylists();
    fetchLikes();
    fetchHistory();

    const connectWS = () => {
      const ws = new WebSocket(wsUrl(getWsUrl()));
      wsRef.current = ws;

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.event === "task_update") {
            const updatedTask: DownloadTask = msg.data;
            setTasks((prev) => {
              const idx = prev.findIndex((t) => t.id === updatedTask.id);
              if (idx >= 0) {
                const next = [...prev];
                next[idx] = updatedTask;
                return next;
              } else {
                return [updatedTask, ...prev];
              }
            });

            if (updatedTask.status === "completed") {
              addToast("success", t("Download Complete"), t("Audio ready — lyrics syncing in background."));
              fetchFiles();
            } else if (updatedTask.status === "failed") {
              addToast("error", t("Import Error"), updatedTask.error || t("Execution error"));
            }
          } else if (msg.event === "downloads_changed") {
            fetchFiles();
            fetchPlaylists();
          } else if (msg.event === "playlist_updated") {
            // Playlist créée en même temps qu'un téléchargement : les
            // morceaux manquants viennent d'être ajoutés.
            fetchPlaylists();
            const added = Number((msg.data as any)?.added ?? 0);
            if (added > 0) {
              addToast("success", t("Playlist updated"), `${added} ${t("tracks added")}`);
            }
          } else if (msg.event === "job_update") {
            // M4 : la file jobs est la vérité. Chaque transition d'état
            // (enfilé → running → done/failed/retry) resynchronise la liste
            // des téléchargements — sans polling ; un petit debounce absorbe
            // les rafales (ex. import de playlist).
            if (jobResyncRef.current) window.clearTimeout(jobResyncRef.current);
            jobResyncRef.current = window.setTimeout(() => {
              fetchTasks();
            }, 200);
          }
        } catch (err) {
          console.error("Failed to parse WS message:", err);
        }
      };

      ws.onclose = () => {
        setTimeout(connectWS, 3000);
      };

      ws.onerror = (err) => {
        console.error("WS error:", err);
        ws.close();
      };
    };

    connectWS();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [fetchTasks, fetchFiles, fetchPlaylists, fetchLikes, fetchHistory]);

  const handleDownload = async (url: string, bitrate: string = "320k", order: string = "reverse") => {
    setIsSubmitting(true);
    try {
      const res = await apiFetch(`${getApiUrl()}/download`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ url, bitrate, order }),
      });

      if (res.ok) {
        const data = await res.json().catch(() => ({}));
        // Lien playlist : le backend a créé la playlist en même temps que le
        // téléchargement — morceaux déjà sur disque ajoutés, manquants en
        // cours de téléchargement.
        const pl = data?.playlist;
        if (pl?.name) {
          addToast(
            "success",
            t("Playlist created"),
            `${pl.name} — ${pl.added_now ?? 0} ${t("tracks added")} · ${pl.to_download ?? 0} ${t("to download")}`
          );
          fetchPlaylists();
        } else {
          let label = t("Track / Playlist Import Started");
          if (url.includes("/artist/")) {
            label = t("Artist Discography Import Started");
          }
          const orderText = order === "reverse" ? t("Newest Added First") : t("Original Order");
          addToast(
            "info",
            label,
            `${t("Downloading")} (${bitrate}, ${orderText}) ${t("MP3 + Metadata + Clean Lyrics...")}`
          );
        }
        fetchTasks();
      } else {
        const data = await res.json();
        addToast("error", t("Error"), data.error || t("Failed to dispatch import"));
      }
    } catch (err) {
      addToast("error", t("Network Error"), t("Unable to connect to Go backend"));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteFile = async (path: string) => {
    try {
      const res = await apiFetch(`${getApiUrl()}/downloads?path=${encodeURIComponent(path)}`, {
        method: "DELETE",
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        addToast("success", t("File Removed"), t('Removed "{name}" from storage.', { name: path }));
        // Une autre copie du même morceau existe (ex. on supprime le fichier
        // album, il reste le single) : les stats ont été recollées dessus.
        if (data?.stats_migrated?.to) {
          addToast(
            "info",
            t("Stats kept"),
            t("Stats transferred to \"{to}\"", { to: data.stats_migrated.to })
          );
        }
        fetchFiles();
        fetchPlaylists();
      } else {
        addToast("error", t("Delete Error"), data.error || t("Error"));
      }
    } catch (err) {
      addToast("error", t("Network Error"), t("Action could not be completed"));
    }
  };

  // ── Playlists ───────────────────────────────────────────────────────────
  const createPlaylist = async (name: string): Promise<PlaylistSummary | null> => {
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ name }),
      });
      if (res.ok) {
        const data = await res.json();
        fetchPlaylists();
        addToast("success", t("Playlist created"), data.playlist?.name || name);
        return data.playlist;
      }
    } catch {
      // ignore — toast below
    }
    addToast("error", t("Error"), t("Failed to dispatch import"));
    return null;
  };

  // Create a playlist and, if a track is given, add it right away.
  const handleCreateAndAdd = async (name: string, path?: string) => {
    const pl = await createPlaylist(name);
    if (pl && path) await addTrackToPlaylist(pl.id, path);
  };

  const addTrackToPlaylist = async (playlistId: string, path: string) => {
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists/${playlistId}/tracks`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ path }),
      });
      if (res.ok) {
        addToast("success", t("Added to playlist"), path);
        fetchPlaylists();
        if (activeNav === `pl:${playlistId}`) loadPlaylist(playlistId);
      }
    } catch {
      addToast("error", t("Error"), t("Failed to dispatch import"));
    }
  };

  const removeTrackFromPlaylist = async (playlistId: string, path: string) => {
    try {
      const res = await apiFetch(
        `${getApiUrl()}/playlists/${playlistId}/tracks?path=${encodeURIComponent(path)}`,
        { method: "DELETE" }
      );
      if (res.ok) {
        addToast("success", t("Removed from playlist"), path);
        fetchPlaylists();
        if (activeNav === `pl:${playlistId}`) loadPlaylist(playlistId);
      }
    } catch {
      addToast("error", t("Error"), t("Action could not be completed"));
    }
  };

  const reorderPlaylistTrack = async (id: string, path: string, toIndex: number) => {
    const tracks = playlistDetail?.tracks ?? [];
    const paths = tracks.map((f) => f.rel_path);
    const from = paths.indexOf(path);
    if (from < 0) return;
    const [moved] = paths.splice(from, 1);
    paths.splice(Math.min(toIndex, paths.length), 0, moved);
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists/${id}/order`, {
        method: "POST",
        headers: jsonHeaders,
        body: JSON.stringify({ paths }),
      });
      if (res.ok) {
        fetchPlaylists();
        loadPlaylist(id);
      }
    } catch {
      // ignore — playlist stays in its previous order
    }
  };

  const deletePlaylist = async (id: string) => {
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists/${id}`, { method: "DELETE" });
      if (res.ok) {
        addToast("success", t("Playlist deleted"), "");
        if (activeNav === `pl:${id}`) {
          setActiveNav("songs");
          setPlaylistDetail(null);
        }
        fetchPlaylists();
      }
    } catch {
      addToast("error", t("Error"), t("Action could not be completed"));
    }
  };

  const openPlaylist = (id: string) => {
    setActiveNav(`pl:${id}`);
    setPlaylistDetail(null);
    loadPlaylist(id);
  };

  // ── Likes ──────────────────────────────────────────────────────────────
  const toggleLike = async (path: string) => {
    const isLiked = likes.has(path);
    // Optimistic update.
    setLikes((prev) => {
      const next = new Set(prev);
      if (isLiked) next.delete(path);
      else next.add(path);
      return next;
    });
    try {
      const res = isLiked
        ? await apiFetch(`${getApiUrl()}/likes?path=${encodeURIComponent(path)}`, { method: "DELETE" })
        : await apiFetch(`${getApiUrl()}/likes`, {
            method: "POST",
            headers: jsonHeaders,
            body: JSON.stringify({ path }),
          });
      if (res.ok) {
        addToast(
          "success",
          isLiked ? t("Track removed from likes") : t("Track added to likes"),
          path
        );
      }
    } catch {
      addToast("error", t("Network Error"), t("Action could not be completed"));
    }
  };

  // ── Playback ────────────────────────────────────────────────────────────

  // Recherche serveur (FTS5) : résultats débouncés via /api/search. Le filtre
  // client reste le repli instantané pendant la frappe ou hors-ligne.
  const [searchResults, setSearchResults] = useState<DownloadedFile[] | null>(null);
  const searchSeqRef = useRef(0);

  useEffect(() => {
    const q = activeFilter.trim();
    if (!q) {
      searchSeqRef.current++;
      setSearchResults(null);
      return;
    }
    const seq = ++searchSeqRef.current;
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const res = await apiFetch(`${getApiUrl()}/search?q=${encodeURIComponent(q)}&limit=100`);
          if (res.ok) {
            const data = await res.json();
            const mapped = (data.tracks || []).map(trackToFile);
            // Ne garde que la réponse la plus récente (ignore les retards).
            if (seq === searchSeqRef.current) setSearchResults(mapped);
          }
        } catch {
          // Hors-ligne / erreur serveur : le filtre client prend le relais.
        }
      })();
    }, 200);
    return () => window.clearTimeout(timer);
  }, [activeFilter]);

  const filteredFiles = useMemo(() => {
    // Résultats FTS du serveur quand disponibles, sinon filtre client.
    if (searchResults !== null) return searchResults;
    const q = activeFilter.toLowerCase();
    return files.filter(
      (f) =>
        f.title.toLowerCase().includes(q) ||
        f.artist.toLowerCase().includes(q) ||
        f.album.toLowerCase().includes(q)
    );
  }, [files, activeFilter, searchResults]);

  // Resolve history/likes paths against the scanned library (missing files skipped).
  const byPath = useMemo(() => {
    const m = new Map<string, DownloadedFile>();
    for (const f of files) m.set(f.rel_path, f);
    return m;
  }, [files]);

  const likedFiles = useMemo(() => files.filter((f) => likes.has(f.rel_path)), [files, likes]);

  const recentResolved = useMemo(() => {
    const out: DownloadedFile[] = [];
    for (const r of recentHistory) {
      const f = byPath.get(r.path);
      if (f && !out.some((o) => o.rel_path === f.rel_path)) out.push(f);
    }
    return out;
  }, [recentHistory, byPath]);

  const topResolved = useMemo(() => {
    const out: { file: DownloadedFile; plays: number }[] = [];
    for (const r of topTracks) {
      const f = byPath.get(r.path);
      if (f) out.push({ file: f, plays: r.plays });
    }
    return out;
  }, [topTracks, byPath]);

  // Artists & albums grouped from the library (name → tracks).
  const groupBy = (key: (f: DownloadedFile) => string) => {
    const m = new Map<string, DownloadedFile[]>();
    for (const f of files) {
      const k = key(f);
      const arr = m.get(k) ?? [];
      arr.push(f);
      m.set(k, arr);
    }
    return [...m.entries()]
      .map(([name, list]) => ({ name, files: list }))
      .sort((a, b) => b.files.length - a.files.length || a.name.localeCompare(b.name));
  };

  const artists = useMemo(
    () => groupBy((f) => f.artist || t("Unknown")),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [files]
  );
  const albums = useMemo(
    () => groupBy((f) => f.album || t("Unknown")),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [files]
  );

  const pinnedEntries = useMemo(() => {
    const out: {
      kind: "artist" | "album" | "playlist";
      name: string;
      files: DownloadedFile[];
      id?: string;
      trackCount?: number;
    }[] = [];
    for (const pin of pins) {
      if (pin.kind === "playlist") {
        const pl = playlists.find((p) => p.id === pin.value);
        if (pl) {
          out.push({
            kind: "playlist",
            name: pl.name,
            files: [],
            id: pl.id,
            trackCount: pl.track_count,
          });
        }
        continue;
      }
      const list = pin.kind === "artist" ? artists : albums;
      const entry = list.find((e) => e.name === pin.value);
      if (entry) {
        out.push({
          kind: pin.kind,
          name: entry.name,
          files: entry.files,
          trackCount: entry.files.length,
        });
      }
    }
    return out;
  }, [pins, artists, albums, playlists]);

  const artistName = activeNav.startsWith("artist:")
    ? decodeURIComponent(activeNav.slice(7))
    : null;
  const albumName = activeNav.startsWith("album:")
    ? decodeURIComponent(activeNav.slice(6))
    : null;
  const artistFiles = artistName
    ? files.filter((f) => (f.artist || t("Unknown")) === artistName)
    : [];
  const albumFiles = albumName
    ? files.filter((f) => (f.album || t("Unknown")) === albumName)
    : [];

  // The ordered list of rel_paths for the current view — becomes the queue.
  const currentListPaths = (): string[] => {
    if (activeNav.startsWith("pl:")) {
      return playlistDetail?.tracks.map((f) => f.rel_path) ?? [];
    }
    if (activeNav === "lyrics") {
      return files.map((f) => f.rel_path);
    }
    if (activeNav === "liked") {
      return likedFiles.filter((f) => filteredFiles.includes(f)).map((f) => f.rel_path);
    }
    return filteredFiles.map((f) => f.rel_path);
  };

  const scrobble = (path: string) => {
    if (lastScrobbledRef.current === path) return;
    lastScrobbledRef.current = path;
    const track = files.find((f) => f.rel_path === path);
    apiFetch(`${getApiUrl()}/scrobble`, {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({ path, duration: Math.round(track?.duration || 0) }),
    })
      .catch(() => {})
      .then(() => fetchHistory());
  };

  // Builds the shuffle order (indices into the queue) starting at the current
  // index, so shuffle never restarts from a random track.
  const buildShuffleOrder = (list: string[], idx: number) => {
    const others = list.map((_, i) => i).filter((i) => i !== idx);
    for (let i = others.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [others[i], others[j]] = [others[j], others[i]];
    }
    shuffleOrderRef.current = [idx, ...others];
  };

  const playFromList = (list: string[], idx: number) => {
    if (idx < 0 || idx >= list.length) return;
    const relPath = list[idx];
    queueRef.current = list;
    queueIndexRef.current = idx;
    if (shuffle) buildShuffleOrder(list, idx);
    setQueueTick((v) => v + 1);
    setCurrentTrackPath(relPath);
    fetchLyrics(relPath);
    setIsPlaying(true);
    if (audioRef.current) {
      audioRef.current.src = getStreamUrl(relPath);
      audioRef.current.play().catch((err) => console.error("Playback error:", err));
    }
  };

  const handleTrackPlay = (relPath: string) => {
    const list = currentListPaths();
    const idx = list.indexOf(relPath);
    if (idx < 0) return;

    if (currentTrackPath === relPath) {
      if (audioRef.current) {
        if (isPlaying) {
          audioRef.current.pause();
          setIsPlaying(false);
        } else {
          audioRef.current.play();
          setIsPlaying(true);
        }
      }
      return;
    }
    playFromList(list, idx);
  };

  const handleSelectTrackForDrawer = (track: DownloadedFile) => {
    setSelectedTrackForDrawer(track);
    setIsLyricsDrawerOpen(true);
  };

  const handlePrevTrack = () => {
    const q = queueRef.current;
    if (q.length === 0) return;
    // In shuffle mode, walk the shuffled order backwards; otherwise wrap.
    if (shuffle && shuffleOrderRef.current.length > 0) {
      const order = shuffleOrderRef.current;
      const pos = order.indexOf(queueIndexRef.current);
      const idx = pos <= 0 ? order[order.length - 1] : order[pos - 1];
      playFromList(q, idx);
      return;
    }
    const idx = (queueIndexRef.current - 1 + q.length) % q.length;
    playFromList(q, idx);
  };

  const handleNextTrack = () => {
    const q = queueRef.current;
    if (q.length === 0) return;
    // Repeat-one: replay the current track from the start.
    if (repeatMode === "one") {
      if (audioRef.current) {
        audioRef.current.currentTime = 0;
        audioRef.current.play().catch((err) => console.error("Playback error:", err));
      }
      return;
    }
    if (shuffle && shuffleOrderRef.current.length > 0) {
      const order = shuffleOrderRef.current;
      const pos = order.indexOf(queueIndexRef.current);
      if (pos < order.length - 1) {
        playFromList(q, order[pos + 1]);
      } else if (repeatMode === "all") {
        playFromList(q, order[0]);
      } else {
        // Repeat off: stop at the end of the shuffled queue.
        setIsPlaying(false);
        if (audioRef.current) audioRef.current.pause();
      }
      return;
    }
    const idx = queueIndexRef.current + 1;
    if (idx < q.length) {
      playFromList(q, idx);
    } else if (repeatMode === "all") {
      playFromList(q, 0);
    } else {
      // Repeat off: stop at the end.
      setIsPlaying(false);
      if (audioRef.current) audioRef.current.pause();
    }
  };

  const toggleShuffle = () => {
    setShuffle((prev) => {
      const next = !prev;
      if (next && queueRef.current.length > 0) {
        buildShuffleOrder(queueRef.current, queueIndexRef.current);
      }
      return next;
    });
  };

  const cycleRepeat = () => {
    setRepeatMode((prev) => (prev === "off" ? "all" : prev === "all" ? "one" : "off"));
  };

  // Move a queue entry to another position (drag & drop in the panel).
  // The current index is adjusted so playback stays on the same track.
  const reorderQueue = (from: number, to: number) => {
    const q = [...queueRef.current];
    if (from < 0 || from >= q.length || to < 0 || to >= q.length || from === to) return;
    const cur = queueIndexRef.current;
    const [moved] = q.splice(from, 1);
    q.splice(to, 0, moved);
    let newCur = cur;
    if (from === cur) newCur = to;
    else if (from < cur && to >= cur) newCur = cur - 1;
    else if (from > cur && to <= cur) newCur = cur + 1;
    queueRef.current = q;
    queueIndexRef.current = newCur;
    if (shuffle) buildShuffleOrder(q, newCur);
    setQueueTick((v) => v + 1);
  };

  // Remove a track from the queue. If it was the current one, playback
  // keeps running and the index moves to the next track.
  const removeFromQueue = (idx: number) => {
    const q = [...queueRef.current];
    if (idx < 0 || idx >= q.length) return;
    const cur = queueIndexRef.current;
    q.splice(idx, 1);
    if (q.length === 0) {
      queueRef.current = [];
      queueIndexRef.current = -1;
      setCurrentTrackPath(null);
      setIsPlaying(false);
      if (audioRef.current) audioRef.current.pause();
      setQueueTick((v) => v + 1);
      return;
    }
    let newCur = cur;
    if (idx < cur) newCur = cur - 1;
    else if (idx === cur) newCur = Math.min(cur, q.length - 1);
    queueRef.current = q;
    queueIndexRef.current = newCur;
    if (shuffle) buildShuffleOrder(q, newCur);
    setQueueTick((v) => v + 1);
  };

  // Insert tracks right after the current one — used by the right-click
  // context menu ("Play next") and the selection action bar.
  const playNext = (paths: string[]) => {
    if (paths.length === 0) return;
    const q = queueRef.current;
    if (q.length === 0) {
      playFromList(paths, 0);
      return;
    }
    const at = queueIndexRef.current + 1;
    const next = [...q];
    next.splice(at, 0, ...paths);
    queueRef.current = next;
    if (shuffle) buildShuffleOrder(next, queueIndexRef.current);
    setQueueTick((v) => v + 1);
    const added = paths.length;
    addToast("info", t("Added to queue"), `${added} ${added > 1 ? t("tracks") : t("track")} ${t("will play next")}`);
  };

  const handleNavChange = (nav: string) => {
    if (nav.startsWith("pl:")) {
      openPlaylist(nav.slice(3));
      return;
    }
    setActiveNav(nav);
  };

  // If a module gets disabled while its view is open, fall back to the library.
  useEffect(() => {
    if (activeNav === "downloads" && !importEnabled) setActiveNav("songs");
    if (activeNav === "stats" && !statsEnabled) setActiveNav("songs");
  }, [importEnabled, statsEnabled, activeNav]);

  const toggleSidebarSide = () => {
    setSidebarRight((prev) => {
      const next = !prev;
      try {
        window.localStorage.setItem("soneph_sidebar_pos", next ? "right" : "left");
      } catch {
        // storage unavailable — position stays in-memory
      }
      return next;
    });
  };

  const activeTasksCount = tasks.filter(
    (t) => t.status === "downloading" || t.status === "queued"
  ).length;
  const syncedCount = files.filter((f) => f.lyrics_type === "synced").length;
  const currentTrack =
    files.find((f) => f.rel_path === currentTrackPath) ?? null;

  // Auto-switch drawer track when currently playing song changes
  useEffect(() => {
    if (currentTrack && isLyricsDrawerOpen) {
      setSelectedTrackForDrawer(currentTrack);
    }
  }, [currentTrack, isLyricsDrawerOpen]);

  const handleSeekTrack = (time: number) => {
    if (audioRef.current) {
      audioRef.current.currentTime = time;
    }
  };

  const playlistId = activeNav.startsWith("pl:") ? activeNav.slice(3) : null;

  // Shared context handed to every plugin-contributed view.
  const app: PluginApp = {
    nav: activeNav,
    setNav: setActiveNav,
    files,
    tasks,
    playlists,
    playlistDetail,
    likes,
    toggleLike,
    playTrack: handleTrackPlay,
    playList: playFromList,
    playNext,
    getApiUrl,
    notify: addToast,
    refreshFiles: fetchFiles,
    currentPlayingPath: currentTrack ? currentTrack.rel_path : null,
    isPlaying,

    // Derived library data
    filteredFiles,
    likedFiles,
    recent: recentResolved,
    top: topResolved,
    totalPlays: recentHistory.length,
    pinned: pinnedEntries,
    artists,
    albums,

    // Pins
    isPinned,
    togglePin,

    // Lyrics drawer
    openLyricsDrawer: handleSelectTrackForDrawer,

    // File & playlist operations
    deleteFile: handleDeleteFile,
    addToPlaylist: addTrackToPlaylist,
    createPlaylist: handleCreateAndAdd,
    removeFromPlaylist: removeTrackFromPlaylist,
    reorderPlaylist: reorderPlaylistTrack,
    deletePlaylist,
    openPlaylist,
  };

  // Resolve the active nav to a registry view id: static views map 1:1,
  // dynamic routes (playlist / artist / album details) match by prefix.
  const routeViewId = (() => {
    if (activeNav.startsWith("pl:")) return "playlist";
    if (activeNav.startsWith("artist:") || activeNav.startsWith("album:")) return "collection";
    return activeNav;
  })();

  return (
    <main className="h-screen w-screen bg-[#161618] flex overflow-hidden font-sans relative">
      {/* Aurora background: soft colored blobs that the liquid-glass surfaces refract */}
      <div className="pointer-events-none absolute inset-0 z-0" aria-hidden="true">
        <div className="absolute -top-32 -left-24 w-[480px] h-[480px] rounded-full bg-apple-pink/20 blur-[120px]" />
        <div className="absolute top-1/3 -right-32 w-[520px] h-[520px] rounded-full bg-indigo-500/15 blur-[130px]" />
        <div className="absolute -bottom-40 left-1/3 w-[560px] h-[560px] rounded-full bg-sky-500/15 blur-[140px]" />
      </div>

      {/* Hidden Audio Player Element */}
      <audio
        ref={audioRef}
        onEnded={handleNextTrack}
        onPlay={() => {
          setIsPlaying(true);
          if (currentTrackPath) scrobble(currentTrackPath);
        }}
        onPause={() => setIsPlaying(false)}
      />

      {/* Sidebar — flippable left/right */}
      {!sidebarRight && (
        <Sidebar
          side="left"
          app={app}
          pins={pins}
          onTogglePin={togglePin}
          activeFilter={activeFilter}
          onFilterChange={setActiveFilter}
          activeNav={activeNav}
          onNavChange={handleNavChange}
          playlists={playlists}
          onCreatePlaylist={createPlaylist}
          queueTracks={queueTracks}
          currentIndex={queueRef.current.indexOf(currentTrackPath ?? "")}
          onPlayQueueIndex={(i) => {
            const q = queueRef.current;
            if (i >= 0 && i < q.length) playFromList(q, i);
          }}
          onRemoveFromQueue={removeFromQueue}
        />
      )}

      {/* Main Content Panel */}
      <div className="flex-1 h-full flex flex-col overflow-hidden bg-[#161618] relative z-10">
        {/* Header */}
        <AppHeader
          onDownload={handleDownload}
          isLoading={isSubmitting}
          activeTasksCount={importEnabled ? activeTasksCount : 0}
          importEnabled={importEnabled}
          currentNav={activeNav}
          currentPlaylistName={playlistDetail?.name}
          onOpenQueue={() => setActiveNav("downloads")}
          sidebarRight={sidebarRight}
          onToggleSidebar={toggleSidebarSide}
        />

        {/* Scrollable Main View */}
        <div className="flex-1 overflow-y-auto pb-32">
          {!configured ? (
            <OnboardingView onFinish={finishOnboarding} />
          ) : (
            <PluginHostView viewId={routeViewId} app={app} />
          )}
        </div>
      </div>

      {/* Sidebar on the right side of the window */}
      {sidebarRight && (
        <Sidebar
          side="right"
          app={app}
          pins={pins}
          onTogglePin={togglePin}
          activeFilter={activeFilter}
          onFilterChange={setActiveFilter}
          activeNav={activeNav}
          onNavChange={handleNavChange}
          playlists={playlists}
          onCreatePlaylist={createPlaylist}
          queueTracks={queueTracks}
          currentIndex={queueRef.current.indexOf(currentTrackPath ?? "")}
          onPlayQueueIndex={(i) => {
            const q = queueRef.current;
            if (i >= 0 && i < q.length) playFromList(q, i);
          }}
          onRemoveFromQueue={removeFromQueue}
        />
      )}

      {/* Right Side Lyrics & Details Column (Non-blocking window panel) */}
      <LyricsDrawer
        isOpen={isLyricsDrawerOpen}
        onClose={() => setIsLyricsDrawerOpen(false)}
        track={selectedTrackForDrawer}
        currentTime={currentTime}
        isPlaying={isPlaying}
        currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
        onPlayTrack={handleTrackPlay}
        onSeekTrack={handleSeekTrack}
        getApiUrl={getApiUrl}
        onLyricsUpdated={fetchFiles}
      />

      {/* Floating Player */}
      <Player
        currentTrack={currentTrack}
        isPlaying={isPlaying}
        onPlayToggle={() => currentTrack && handleTrackPlay(currentTrack.rel_path)}
        onPrevTrack={handlePrevTrack}
        onNextTrack={handleNextTrack}
        onOpenLyrics={() => setIsLyricsOpen(true)}
        audioRef={audioRef}
        onTimeUpdate={(t) => setCurrentTime(t)}
        getApiUrl={getApiUrl}
        queueTracks={queueTracks}
        currentIndex={queueRef.current.indexOf(currentTrackPath ?? "")}
        shuffle={shuffle}
        onToggleShuffle={toggleShuffle}
        repeatMode={repeatMode}
        onCycleRepeat={cycleRepeat}
        onPlayIndex={(i) => {
          const q = queueRef.current;
          if (i >= 0 && i < q.length) playFromList(q, i);
        }}
        onReorderQueue={reorderQueue}
        onRemoveFromQueue={removeFromQueue}
      />

      {/* Karaoke Style Synchronized Lyrics Modal */}
      <LyricsModal
        isOpen={isLyricsOpen}
        onClose={() => setIsLyricsOpen(false)}
        currentTrack={currentTrack}
        currentTime={currentTime}
        lyricsRaw={lyricsRaw}
      />

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </main>
  );
}
