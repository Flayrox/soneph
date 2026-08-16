import React, { useEffect, useRef, useState, useMemo } from "react";
import { X, Music, Play, Pause, RefreshCw, Copy, Check } from "lucide-react";
import type { DownloadedFile } from "@/types";
import { useI18n } from "@/i18n";
import { apiFetch } from "@/api";

interface LyricsDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  track: DownloadedFile | null;
  currentTime: number;
  isPlaying: boolean;
  currentPlayingPath: string | null;
  onPlayTrack: (relPath: string) => void;
  onSeekTrack?: (time: number) => void;
  getApiUrl: () => string;
  onLyricsUpdated?: () => void;
}

interface LyricLine {
  time: number | null;
  text: string;
}

export const LyricsDrawer: React.FC<LyricsDrawerProps> = ({
  isOpen,
  onClose,
  track,
  currentTime,
  isPlaying,
  currentPlayingPath,
  onPlayTrack,
  onSeekTrack,
  getApiUrl,
  onLyricsUpdated,
}) => {
  const { t } = useI18n();
  const [lyricsRaw, setLyricsRaw] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [isRetrying, setIsRetrying] = useState<boolean>(false);
  const [copied, setCopied] = useState<boolean>(false);
  const [showSyncButton, setShowSyncButton] = useState<boolean>(false);

  const containerRef = useRef<HTMLDivElement | null>(null);
  const activeLineRef = useRef<HTMLDivElement | null>(null);
  const isUserScrolledRef = useRef<boolean>(false);
  const isProgrammaticScrollingRef = useRef<boolean>(false);
  const lastActiveIndexRef = useRef<number>(-1);
  const lyricsCacheRef = useRef<Map<string, string>>(new Map());

  const isCurrentPlaying = track && currentPlayingPath === track.rel_path;

  // In-memory cached lyrics fetcher
  useEffect(() => {
    if (!track || !isOpen) {
      setLyricsRaw(null);
      return;
    }

    // Reset user scroll state on track change
    isUserScrolledRef.current = false;
    isProgrammaticScrollingRef.current = false;
    lastActiveIndexRef.current = -1;
    setShowSyncButton(false);

    if (lyricsCacheRef.current.has(track.rel_path)) {
      setLyricsRaw(lyricsCacheRef.current.get(track.rel_path)!);
      setIsLoading(false);
      return;
    }

    let isMounted = true;
    setIsLoading(true);

    apiFetch(`${getApiUrl()}/lyrics?path=${encodeURIComponent(track.rel_path)}`)
      .then((res) => (res.ok ? res.json() : { lyrics: null }))
      .then((data) => {
        if (isMounted) {
          const lyrics = data.lyrics || null;
          if (lyrics) {
            lyricsCacheRef.current.set(track.rel_path, lyrics);
          }
          setLyricsRaw(lyrics);
        }
      })
      .catch(() => {
        if (isMounted) setLyricsRaw(null);
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [track, isOpen, getApiUrl]);

  // Parse LRC / Text lines
  const parsedLines = useMemo<LyricLine[]>(() => {
    if (!lyricsRaw) return [];

    const lines = lyricsRaw.split("\n");
    const parsed: LyricLine[] = [];
    const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;

    lines.forEach((line) => {
      const match = timeRegex.exec(line);
      if (match) {
        const min = parseInt(match[1], 10);
        const sec = parseInt(match[2], 10);
        const ms = parseInt(match[3], 10);
        const totalTime = min * 60 + sec + (ms > 99 ? ms / 1000 : ms / 100);
        const text = line.replace(timeRegex, "").trim();
        if (text) {
          parsed.push({ time: totalTime, text });
        }
      } else {
        const text = line.trim();
        if (text && !text.startsWith("[")) {
          parsed.push({ time: null, text });
        }
      }
    });

    return parsed;
  }, [lyricsRaw]);

  // Compute current active line index
  const activeIndex = useMemo(() => {
    if (!isCurrentPlaying || parsedLines.length === 0) return -1;
    let idx = -1;
    for (let i = 0; i < parsedLines.length; i++) {
      if (parsedLines[i].time !== null && currentTime >= (parsedLines[i].time as number)) {
        idx = i;
      } else if (parsedLines[i].time !== null) {
        break;
      }
    }
    return idx;
  }, [isCurrentPlaying, parsedLines, currentTime]);

  // Hardware events (wheel, touch, mouse/pointer down) are 100% human physical input
  const handleUserHardwareInput = () => {
    if (!isCurrentPlaying) return;
    isUserScrolledRef.current = true;
    setShowSyncButton(true);
  };

  // Generic scroll event fallback
  const handleContainerScroll = () => {
    if (!isCurrentPlaying) return;
    if (!isProgrammaticScrollingRef.current) {
      isUserScrolledRef.current = true;
      setShowSyncButton(true);
    }
  };

  // Perform smooth container scroll ONLY if user hasn't manually scrolled away
  useEffect(() => {
    if (isUserScrolledRef.current || activeIndex < 0 || !isCurrentPlaying) return;

    if (activeIndex !== lastActiveIndexRef.current) {
      lastActiveIndexRef.current = activeIndex;
      const container = containerRef.current;
      const activeEl = activeLineRef.current;

      if (container && activeEl) {
        isProgrammaticScrollingRef.current = true;
        const targetScrollTop = activeEl.offsetTop - container.clientHeight / 3;

        container.scrollTo({
          top: Math.max(0, targetScrollTop),
          behavior: "smooth",
        });

        // Unlock after smooth scroll completes (~450ms)
        setTimeout(() => {
          isProgrammaticScrollingRef.current = false;
        }, 450);
      }
    }
  }, [activeIndex, isCurrentPlaying]);

  const handleResumeSync = () => {
    isUserScrolledRef.current = false;
    setShowSyncButton(false);
    lastActiveIndexRef.current = -1;

    const container = containerRef.current;
    const activeEl = activeLineRef.current;
    if (container && activeEl) {
      isProgrammaticScrollingRef.current = true;
      const targetScrollTop = activeEl.offsetTop - container.clientHeight / 3;
      container.scrollTo({
        top: Math.max(0, targetScrollTop),
        behavior: "smooth",
      });
      setTimeout(() => {
        isProgrammaticScrollingRef.current = false;
      }, 450);
    }
  };

  const handleLineClick = (lineTime: number | null) => {
    if (lineTime !== null && onSeekTrack) {
      if (!isCurrentPlaying && track) {
        onPlayTrack(track.rel_path);
      }
      onSeekTrack(lineTime);
      isUserScrolledRef.current = false;
      setShowSyncButton(false);
    }
  };

  const handleCopy = () => {
    if (!lyricsRaw) return;
    navigator.clipboard.writeText(lyricsRaw);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleRetryLyrics = async () => {
    if (!track) return;
    setIsRetrying(true);
    try {
      const res = await apiFetch(`${getApiUrl()}/lyrics/retry`, { method: "POST" });
      if (res.ok) {
        if (onLyricsUpdated) onLyricsUpdated();
        setTimeout(() => {
          apiFetch(`${getApiUrl()}/lyrics?path=${encodeURIComponent(track.rel_path)}`)
            .then((r) => (r.ok ? r.json() : null))
            .then((data) => {
              if (data?.lyrics) {
                lyricsCacheRef.current.set(track.rel_path, data.lyrics);
                setLyricsRaw(data.lyrics);
              }
            })
            .finally(() => setIsRetrying(false));
        }, 2000);
      } else {
        setIsRetrying(false);
      }
    } catch {
      setIsRetrying(false);
    }
  };

  if (!isOpen || !track) return null;

  const isSynced = track.lyrics_type === "synced" || parsedLines.some((l) => l.time !== null);
  const isUnsynced = !isSynced && (track.lyrics_type === "unsynced" || parsedLines.length > 0);
  const isMissing = !isSynced && !isUnsynced;

  return (
    <aside className="w-80 sm:w-96 h-full bg-[#1e1e20] border-l border-white/10 flex flex-col justify-between shrink-0 select-none pb-24 z-20 relative">
      {/* Header */}
      <div className="p-4 border-b border-white/10 flex items-center justify-between bg-[#18181a]">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-9 h-9 rounded bg-[#2a2a2d] border border-white/10 flex items-center justify-center text-apple-pink shrink-0 overflow-hidden relative">
            <Music className="w-4.5 h-4.5 absolute inset-0 m-auto opacity-60" />
            <img
              src={`${getApiUrl()}/cover?path=${encodeURIComponent(track.rel_path)}`}
              alt={track.title}
              className="w-full h-full object-cover relative z-10"
              onError={(e) => {
                e.currentTarget.style.display = "none";
              }}
            />
          </div>
          <div className="min-w-0">
            <h3 className="text-xs font-bold text-white truncate">{track.title}</h3>
            <p className="text-[11px] text-apple-subtext truncate">{track.artist}</p>
          </div>
        </div>

        <button
          onClick={onClose}
          className="w-7 h-7 rounded-full hover:bg-white/10 text-apple-subtext hover:text-white flex items-center justify-center transition-colors shrink-0 ml-2"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Status Bar */}
      <div className="px-4 py-2 border-b border-white/10 bg-[#161618] flex items-center justify-between text-xs">
        <span
          className={`text-[10px] font-semibold px-2 py-0.5 rounded ${
            isSynced
              ? "bg-emerald-500/15 text-emerald-400 border border-emerald-500/20"
              : isUnsynced
              ? "bg-amber-500/15 text-amber-400 border border-amber-500/20"
              : "bg-rose-500/15 text-rose-400 border border-rose-500/20"
          }`}
        >
          {isSynced ? t("Synced (LRC)") : isUnsynced ? t("Plain Text") : t("Missing")}
        </span>

        {lyricsRaw && (
          <button
            onClick={handleCopy}
            className="flex items-center gap-1 text-[11px] text-apple-subtext hover:text-white px-2 py-0.5 rounded bg-white/5 transition-colors"
          >
            {copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
            <span>{copied ? t("Copied") : t("Copy")}</span>
          </button>
        )}
      </div>

      {/* Lyrics Scroll Stream with Hardware Human Input Capture */}
      <div
        ref={containerRef}
        onWheel={handleUserHardwareInput}
        onTouchMove={handleUserHardwareInput}
        onMouseDown={handleUserHardwareInput}
        onPointerDown={handleUserHardwareInput}
        onScroll={handleContainerScroll}
        className="flex-1 overflow-y-auto p-5 space-y-4 scrollbar-thin scrollbar-thumb-white/10 relative"
      >
        {isLoading ? (
          <div className="h-full flex flex-col items-center justify-center text-apple-subtext space-y-2">
            <RefreshCw className="w-4 h-4 animate-spin text-apple-pink" />
            <p className="text-xs">{t("Loading lyrics...")}</p>
          </div>
        ) : isMissing ? (
          <div className="h-full flex flex-col items-center justify-center text-center space-y-3 py-12">
            <p className="text-xs font-semibold text-white">{t("No lyrics available")}</p>
            <p className="text-[11px] text-apple-subtext">{t("No .lrc file found for this track.")}</p>
            <button
              onClick={handleRetryLyrics}
              disabled={isRetrying}
              className="px-3 py-1.5 rounded-md bg-apple-pink hover:bg-apple-pinkHover text-white font-semibold text-xs transition-all disabled:opacity-50"
            >
              {isRetrying ? t("Searching...") : t("Fetch Lyrics")}
            </button>
          </div>
        ) : (
          <div className="space-y-3 pb-8">
            {parsedLines.map((line, idx) => {
              const isActive = idx === activeIndex;
              const isPast = activeIndex >= 0 && idx < activeIndex;

              return (
                <div
                  key={idx}
                  ref={isActive ? activeLineRef : null}
                  onClick={() => handleLineClick(line.time)}
                  className={`text-sm font-semibold transition-colors duration-200 leading-relaxed cursor-pointer ${
                    isActive
                      ? "text-white font-bold scale-[1.01] origin-left"
                      : isPast
                      ? "text-apple-subtext/40"
                      : "text-zinc-400 hover:text-white"
                  }`}
                >
                  {line.text}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Resume Sync Floating Button (Only when user explicitly scrolled away) */}
      {showSyncButton && isCurrentPlaying && isSynced && (
        <div className="absolute bottom-16 left-1/2 -translate-x-1/2 z-30">
          <button
            onClick={handleResumeSync}
            className="bg-apple-pink text-white text-[11px] font-semibold px-3 py-1.5 rounded-full shadow-lg hover:bg-apple-pinkHover transition-all"
          >
            <span>{t("Sync to Playback")}</span>
          </button>
        </div>
      )}

      {/* Bottom Controls */}
      <div className="p-3 border-t border-white/10 bg-[#161618] flex items-center gap-2">
        <button
          onClick={() => onPlayTrack(track.rel_path)}
          className={`flex-1 flex items-center justify-center gap-2 py-1.5 px-3 rounded-md font-semibold text-xs transition-all ${
            isCurrentPlaying && isPlaying
              ? "bg-white/10 text-white"
              : "bg-apple-pink text-white hover:bg-apple-pinkHover"
          }`}
        >
          {isCurrentPlaying && isPlaying ? (
            <>
              <Pause className="w-3.5 h-3.5 fill-white" />
              <span>{t("Pause")}</span>
            </>
          ) : (
            <>
              <Play className="w-3.5 h-3.5 fill-white ml-0.5" />
              <span>{t("Play")}</span>
            </>
          )}
        </button>

        {!isSynced && (
          <button
            onClick={handleRetryLyrics}
            disabled={isRetrying}
            className="flex items-center gap-1 py-1.5 px-3 rounded-md bg-[#242428] hover:bg-white/10 text-white text-xs font-semibold border border-white/10 transition-all disabled:opacity-50"
          >
            <RefreshCw className={`w-3 h-3 text-apple-pink ${isRetrying ? "animate-spin" : ""}`} />
            <span>{isRetrying ? t("Retrying...") : t("Upgrade")}</span>
          </button>
        )}
      </div>
    </aside>
  );
};
