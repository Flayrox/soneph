import React, { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { Sidebar } from "@/components/Sidebar";
import { AppHeader } from "@/components/AppHeader";
import { TrackList } from "@/components/TrackList";
import { Player } from "@/components/Player";
import { LyricsModal } from "@/components/LyricsModal";
import { ToastContainer, ToastMessage } from "@/components/Toast";
import { LyricsDrawer } from "@/components/LyricsDrawer";
import { LyricsManagerView } from "@/components/LyricsManagerView";
import { SyncSettingsView } from "@/components/SyncSettingsView";
import { DownloadsView } from "@/components/DownloadsView";
import { PlaylistView } from "@/components/PlaylistView";
import { HomeView } from "@/components/HomeView";
import { useI18n } from "@/i18n";
import { apiFetch, wsUrl } from "@/api";
import type {
  DownloadedFile,
  DownloadTask,
  HistoryRecord,
  Playlist,
  PlaylistSummary,
  TopTrack,
} from "@/types";

// The frontend is served from the same origin as the Go backend
// (Vite dev proxy in development, go:embed in production), so all
// API + WebSocket URLs are relative.
const API_URL = "/api";
const WS_URL = "/api/ws";

const jsonHeaders = { "Content-Type": "application/json" };

export default function App() {
  const { t } = useI18n();
  const [tasks, setTasks] = useState<DownloadTask[]>([]);
  const [files, setFiles] = useState<DownloadedFile[]>([]);
  const [playlists, setPlaylists] = useState<PlaylistSummary[]>([]);
  const [playlistDetail, setPlaylistDetail] = useState<Playlist | null>(null);
  const [likes, setLikes] = useState<Set<string>>(new Set());
  const [recentHistory, setRecentHistory] = useState<HistoryRecord[]>([]);
  const [topTracks, setTopTracks] = useState<TopTrack[]>([]);
  const [activeFilter, setActiveFilter] = useState<string>("");
  const [activeNav, setActiveNav] = useState<string>("songs");
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

  // Synchronized Karaoke Lyrics State
  const [isLyricsOpen, setIsLyricsOpen] = useState<boolean>(false);
  const [lyricsRaw, setLyricsRaw] = useState<string | null>(null);

  const audioRef = useRef<HTMLAudioElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
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
      if (res.ok) {
        addToast("success", t("File Removed"), t('Removed "{name}" from storage.', { name: path }));
        fetchFiles();
        fetchPlaylists();
      } else {
        const data = await res.json();
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
  const filteredFiles = useMemo(() => {
    const list = files.filter(
      (f) =>
        f.title.toLowerCase().includes(activeFilter.toLowerCase()) ||
        f.artist.toLowerCase().includes(activeFilter.toLowerCase()) ||
        f.album.toLowerCase().includes(activeFilter.toLowerCase())
    );
    return list;
  }, [files, activeFilter]);

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
    apiFetch(`${getApiUrl()}/scrobble`, {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({ path }),
    })
      .catch(() => {})
      .then(() => fetchHistory());
  };

  const playFromList = (list: string[], idx: number) => {
    if (idx < 0 || idx >= list.length) return;
    const relPath = list[idx];
    queueRef.current = list;
    queueIndexRef.current = idx;
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
    const idx = (queueIndexRef.current - 1 + q.length) % q.length;
    playFromList(q, idx);
  };

  const handleNextTrack = () => {
    const q = queueRef.current;
    if (q.length === 0) return;
    const idx = (queueIndexRef.current + 1) % q.length;
    playFromList(q, idx);
  };

  const handleNavChange = (nav: string) => {
    if (nav.startsWith("pl:")) {
      openPlaylist(nav.slice(3));
      return;
    }
    setActiveNav(nav);
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

      {/* Sidebar */}
      <Sidebar
        totalFiles={files.length}
        syncedCount={syncedCount}
        activeFilter={activeFilter}
        onFilterChange={setActiveFilter}
        activeNav={activeNav}
        onNavChange={handleNavChange}
        activeTasksCount={activeTasksCount}
        playlists={playlists}
        onCreatePlaylist={createPlaylist}
      />

      {/* Main Content Panel */}
      <div className="flex-1 h-full flex flex-col overflow-hidden bg-[#161618] relative z-10">
        {/* Header */}
        <AppHeader
          onDownload={handleDownload}
          isLoading={isSubmitting}
          activeTasksCount={activeTasksCount}
          currentNav={activeNav}
          currentPlaylistName={playlistDetail?.name}
          onOpenQueue={() => setActiveNav("downloads")}
        />

        {/* Scrollable Main View */}
        <div className="flex-1 overflow-y-auto pb-32">
          {activeNav === "home" ? (
            <HomeView
              files={files}
              recent={recentResolved}
              top={topResolved}
              liked={likedFiles}
              likes={likes}
              totalPlays={recentHistory.length}
              currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
              isPlaying={isPlaying}
              onPlayList={playFromList}
              onToggleLike={toggleLike}
              onNavChange={setActiveNav}
              getApiUrl={getApiUrl}
            />
          ) : activeNav === "sync" ? (
            <SyncSettingsView getApiUrl={getApiUrl} onNotify={addToast} />
          ) : activeNav === "lyrics" ? (
            <LyricsManagerView
              files={files}
              currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
              isPlaying={isPlaying}
              onPlayTrack={handleTrackPlay}
              onSelectTrack={handleSelectTrackForDrawer}
              getApiUrl={getApiUrl}
              onRefreshFiles={fetchFiles}
            />
          ) : activeNav === "downloads" ? (
            <DownloadsView tasks={tasks} />
          ) : playlistId ? (
            <PlaylistView
              playlist={playlistDetail}
              currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
              isPlaying={isPlaying}
              onPlayTrack={handleTrackPlay}
              onSelectTrack={handleSelectTrackForDrawer}
              onRemoveTrack={(path) => removeTrackFromPlaylist(playlistId, path)}
              onDeletePlaylist={() => deletePlaylist(playlistId)}
              onPlayAll={() =>
                playlistDetail && playFromList(playlistDetail.tracks.map((f) => f.rel_path), 0)
              }
              getApiUrl={getApiUrl}
              playlists={playlists}
              onAddToPlaylist={addTrackToPlaylist}
              onCreatePlaylist={handleCreateAndAdd}
              likes={likes}
              onToggleLike={toggleLike}
            />
          ) : (
            <TrackList
              files={
                activeNav === "liked"
                  ? likedFiles.filter((f) => filteredFiles.includes(f))
                  : filteredFiles
              }
              activeTasks={tasks}
              currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
              isPlaying={isPlaying}
              onTrackPlay={handleTrackPlay}
              onSelectTrack={handleSelectTrackForDrawer}
              onDelete={handleDeleteFile}
              getApiUrl={getApiUrl}
              playlists={playlists}
              onAddToPlaylist={addTrackToPlaylist}
              onCreatePlaylist={handleCreateAndAdd}
              likes={likes}
              onToggleLike={toggleLike}
            />
          )}
        </div>
      </div>

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
