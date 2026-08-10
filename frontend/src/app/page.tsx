"use client";

import React, { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { Sidebar } from "@/components/Sidebar";
import { Header } from "@/components/Header";
import { TrackList, DownloadedFile } from "@/components/TrackList";
import { Player } from "@/components/Player";
import { LyricsModal } from "@/components/LyricsModal";
import { QueueDrawer } from "@/components/QueueDrawer";
import { ToastContainer, ToastMessage } from "@/components/Toast";
import { LyricsDrawer } from "@/components/LyricsDrawer";
import { LyricsManagerView } from "@/components/LyricsManagerView";

export interface DownloadTask {
  id: string;
  url: string;
  bitrate?: string;
  order?: string;
  status: "queued" | "downloading" | "completed" | "failed";
  progress: string;
  logs: string[];
  created_at: string;
  error?: string;
}

export default function Home() {
  const [tasks, setTasks] = useState<DownloadTask[]>([]);
  const [files, setFiles] = useState<DownloadedFile[]>([]);
  const [activeFilter, setActiveFilter] = useState<string>("");
  const [activeNav, setActiveNav] = useState<string>("songs");
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);

  // Queue Drawer State
  const [isQueueOpen, setIsQueueOpen] = useState<boolean>(false);

  // Right Side Lyrics Drawer State
  const [selectedTrackForDrawer, setSelectedTrackForDrawer] = useState<DownloadedFile | null>(null);
  const [isLyricsDrawerOpen, setIsLyricsDrawerOpen] = useState<boolean>(false);

  // Audio Playback State
  const [currentTrackIndex, setCurrentTrackIndex] = useState<number | null>(null);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [currentTime, setCurrentTime] = useState<number>(0);

  // Synchronized Karaoke Lyrics State
  const [isLyricsOpen, setIsLyricsOpen] = useState<boolean>(false);
  const [lyricsRaw, setLyricsRaw] = useState<string | null>(null);

  const audioRef = useRef<HTMLAudioElement | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const getApiUrl = () => {
    if (typeof window !== "undefined") {
      const hostname = window.location.hostname;
      return `http://${hostname}:8080/api`;
    }
    return "http://localhost:8080/api";
  };

  const getWsUrl = () => {
    if (typeof window !== "undefined") {
      const hostname = window.location.hostname;
      return `ws://${hostname}:8080/api/ws`;
    }
    return "ws://localhost:8080/api/ws";
  };

  const getStreamUrl = (relPath: string) => {
    return `${getApiUrl()}/stream?path=${encodeURIComponent(relPath)}`;
  };

  const fetchLyrics = async (relPath: string) => {
    try {
      const res = await fetch(`${getApiUrl()}/lyrics?path=${encodeURIComponent(relPath)}`);
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
      const res = await fetch(`${getApiUrl()}/tasks`);
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
      const res = await fetch(`${getApiUrl()}/downloads`);
      if (res.ok) {
        const data = await res.json();
        setFiles(data.files || []);
      }
    } catch (err) {
      console.error("Error fetching downloads:", err);
    }
  }, []);

  // Connect WebSockets
  useEffect(() => {
    fetchTasks();
    fetchFiles();

    const connectWS = () => {
      const ws = new WebSocket(getWsUrl());
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
              addToast(
                "success",
                "Synced to l'app Musique",
                `Track and clean lyrics ready in Music.app & iCloud!`
              );
              fetchFiles();
            } else if (updatedTask.status === "failed") {
              addToast("error", "Import Error", updatedTask.error || "Execution error");
            }
          } else if (msg.event === "downloads_changed") {
            fetchFiles();
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
  }, [fetchTasks, fetchFiles]);

  const handleDownload = async (url: string, bitrate: string = "320k", order: string = "reverse") => {
    setIsSubmitting(true);
    try {
      const res = await fetch(`${getApiUrl()}/download`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url, bitrate, order }),
      });

      if (res.ok) {
        let label = "Track / Playlist Import Started";
        if (url.includes("/artist/")) {
          label = "Artist Discography Import Started";
        }
        const orderText = order === "reverse" ? "Newest Added First" : "Original Order";
        addToast("info", label, `Downloading (${bitrate}, ${orderText}) MP3 + Metadata + Clean Lyrics...`);
        fetchTasks();
      } else {
        const data = await res.json();
        addToast("error", "Error", data.error || "Failed to dispatch import");
      }
    } catch (err) {
      addToast("error", "Network Error", "Unable to connect to Go backend");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteFile = async (path: string) => {
    try {
      const res = await fetch(`${getApiUrl()}/downloads?path=${encodeURIComponent(path)}`, {
        method: "DELETE",
      });
      if (res.ok) {
        addToast("success", "File Removed", `Removed "${path}" from storage.`);
        fetchFiles();
      } else {
        const data = await res.json();
        addToast("error", "Delete Error", data.error || "Unknown error");
      }
    } catch (err) {
      addToast("error", "Network Error", "Action could not be completed");
    }
  };

  // Playback Control Handlers
  const handleTrackPlay = (relPath: string) => {
    const idx = files.findIndex((f) => f.rel_path === relPath);
    if (idx < 0) return;

    fetchLyrics(relPath);

    if (currentTrackIndex === idx) {
      if (audioRef.current) {
        if (isPlaying) {
          audioRef.current.pause();
          setIsPlaying(false);
        } else {
          audioRef.current.play();
          setIsPlaying(true);
        }
      }
    } else {
      setCurrentTrackIndex(idx);
      setIsPlaying(true);
      if (audioRef.current) {
        audioRef.current.src = getStreamUrl(relPath);
        audioRef.current.play().catch((err) => console.error("Playback error:", err));
      }
    }
  };

  const handleSelectTrackForDrawer = (track: DownloadedFile) => {
    setSelectedTrackForDrawer(track);
    setIsLyricsDrawerOpen(true);
  };

  const handlePrevTrack = () => {
    if (currentTrackIndex === null || files.length === 0) return;
    const prevIdx = (currentTrackIndex - 1 + files.length) % files.length;
    handleTrackPlay(files[prevIdx].rel_path);
  };

  const handleNextTrack = () => {
    if (currentTrackIndex === null || files.length === 0) return;
    const nextIdx = (currentTrackIndex + 1) % files.length;
    handleTrackPlay(files[nextIdx].rel_path);
  };

  const handleNavChange = (nav: string) => {
    if (nav === "downloading") {
      setIsQueueOpen(true);
    } else {
      setActiveNav(nav);
    }
  };

  const filteredFiles = useMemo(() => {
    let list = files.filter(
      (f) =>
        f.title.toLowerCase().includes(activeFilter.toLowerCase()) ||
        f.artist.toLowerCase().includes(activeFilter.toLowerCase()) ||
        f.album.toLowerCase().includes(activeFilter.toLowerCase())
    );

    if (activeNav === "recently_added") {
      list = [...list].sort((a, b) => {
        const timeA = a.mod_time ? new Date(a.mod_time).getTime() : 0;
        const timeB = b.mod_time ? new Date(b.mod_time).getTime() : 0;
        return timeB - timeA;
      });
    }

    return list;
  }, [files, activeFilter, activeNav]);

  const activeTasksCount = tasks.filter(
    (t) => t.status === "downloading" || t.status === "queued"
  ).length;
  const syncedCount = files.filter((f) => f.lyrics_type === "synced").length;
  const currentTrack = currentTrackIndex !== null ? files[currentTrackIndex] : null;

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

  return (
    <main className="h-screen w-screen bg-[#161618] flex overflow-hidden font-sans">
      {/* Hidden Audio Player Element */}
      <audio
        ref={audioRef}
        onEnded={handleNextTrack}
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
      />

      {/* l'app Musique macOS Sidebar */}
      <Sidebar
        totalFiles={files.length}
        syncedCount={syncedCount}
        activeFilter={activeFilter}
        onFilterChange={setActiveFilter}
        activeNav={activeNav}
        onNavChange={handleNavChange}
        activeTasksCount={activeTasksCount}
      />

      {/* Main l'app Musique Content Panel */}
      <div className="flex-1 h-full flex flex-col overflow-hidden bg-[#161618]">
        {/* l'app Musique Header */}
        <Header
          onDownload={handleDownload}
          isLoading={isSubmitting}
          activeTasksCount={activeTasksCount}
          currentNav={activeNav}
          onOpenQueue={() => setIsQueueOpen(true)}
        />

        {/* Scrollable Main View (Songs vs Lyrics Management Center) */}
        <div className="flex-1 overflow-y-auto pb-32">
          {activeNav === "lyrics" ? (
            <LyricsManagerView
              files={files}
              currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
              isPlaying={isPlaying}
              onPlayTrack={handleTrackPlay}
              onSelectTrack={handleSelectTrackForDrawer}
              getApiUrl={getApiUrl}
              onRefreshFiles={fetchFiles}
            />
          ) : (
            <TrackList
              files={filteredFiles}
              activeTasks={tasks}
              currentPlayingPath={currentTrack ? currentTrack.rel_path : null}
              isPlaying={isPlaying}
              onTrackPlay={handleTrackPlay}
              onSelectTrack={handleSelectTrackForDrawer}
              onDelete={handleDeleteFile}
              getApiUrl={getApiUrl}
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

      {/* Floating l'app Musique Player */}
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

      {/* l'app Musique Style Active Queue & Download Manager Drawer */}
      <QueueDrawer
        isOpen={isQueueOpen}
        onClose={() => setIsQueueOpen(false)}
        tasks={tasks}
      />

      {/* l'app Musique Karaoke Style Synchronized Lyrics Modal */}
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
