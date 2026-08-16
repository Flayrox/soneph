import React, { useState, useEffect, useRef } from "react";
import { Glass } from "./Glass";
import {
  Play,
  Pause,
  SkipBack,
  SkipForward,
  Shuffle,
  Repeat,
  Repeat1,
  Volume2,
  VolumeX,
  MessageSquareQuote,
  ListMusic,
  Disc,
  X,
} from "lucide-react";
import { cleanTitle } from "@/format";
import type { DownloadedFile } from "@/types";
import { useI18n } from "@/i18n";

interface PlayerProps {
  currentTrack: DownloadedFile | null;
  isPlaying: boolean;
  onPlayToggle: () => void;
  onPrevTrack: () => void;
  onNextTrack: () => void;
  onOpenLyrics: () => void;
  audioRef: React.RefObject<HTMLAudioElement | null>;
  onTimeUpdate?: (time: number) => void;
  getApiUrl?: () => string;
  /** The resolved playback queue (for the visible queue panel). */
  queueTracks?: DownloadedFile[];
  /** Index of the current track inside queueTracks. */
  currentIndex?: number;
  shuffle?: boolean;
  onToggleShuffle?: () => void;
  repeatMode?: "off" | "all" | "one";
  onCycleRepeat?: () => void;
  /** Jump to a specific queue position. */
  onPlayIndex?: (index: number) => void;
  /** Move a queue entry to another position (drag & drop). */
  onReorderQueue?: (from: number, to: number) => void;
  /** Remove a track from the queue. */
  onRemoveFromQueue?: (index: number) => void;
}

export const Player: React.FC<PlayerProps> = ({
  currentTrack,
  isPlaying,
  onPlayToggle,
  onPrevTrack,
  onNextTrack,
  onOpenLyrics,
  audioRef,
  onTimeUpdate,
  getApiUrl,
  queueTracks = [],
  currentIndex = -1,
  shuffle = false,
  onToggleShuffle,
  repeatMode = "off",
  onCycleRepeat,
  onPlayIndex,
  onReorderQueue,
  onRemoveFromQueue,
}) => {
  const { t } = useI18n();
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.8);
  const [isMuted, setIsMuted] = useState(false);
  const [queueOpen, setQueueOpen] = useState(false);
  const queueRef = useRef<HTMLDivElement | null>(null);
  // Drag & drop state for the queue panel: the absolute index being dragged
  // and the absolute index currently hovered as the drop target.
  const [dragQueueIdx, setDragQueueIdx] = useState<number | null>(null);
  const [dropQueueIdx, setDropQueueIdx] = useState<number | null>(null);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    const handleTimeUpdate = () => {
      setCurrentTime(audio.currentTime);
      if (onTimeUpdate) onTimeUpdate(audio.currentTime);
    };
    const handleLoadedMetadata = () => setDuration(audio.duration);

    audio.addEventListener("timeupdate", handleTimeUpdate);
    audio.addEventListener("loadedmetadata", handleLoadedMetadata);

    return () => {
      audio.removeEventListener("timeupdate", handleTimeUpdate);
      audio.removeEventListener("loadedmetadata", handleLoadedMetadata);
    };
  }, [audioRef, onTimeUpdate]);

  // Close the queue panel when clicking outside it.
  useEffect(() => {
    if (!queueOpen) return;
    const onDown = (e: MouseEvent) => {
      if (queueRef.current && !queueRef.current.contains(e.target as Node)) {
        setQueueOpen(false);
      }
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [queueOpen]);

  const handleSeek = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = parseFloat(e.target.value);
    if (audioRef.current) {
      audioRef.current.currentTime = newTime;
      setCurrentTime(newTime);
    }
  };

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = parseFloat(e.target.value);
    setVolume(val);
    if (audioRef.current) {
      audioRef.current.volume = val;
    }
    setIsMuted(val === 0);
  };

  const toggleMute = () => {
    if (!audioRef.current) return;
    if (isMuted) {
      audioRef.current.volume = volume || 0.8;
      setIsMuted(false);
    } else {
      audioRef.current.volume = 0;
      setIsMuted(true);
    }
  };

  if (!currentTrack) {
    return null;
  }

  const progressPercent = duration > 0 ? (currentTime / duration) * 100 : 0;

  // Queue order for display: track immediately after the current one is "up next".
  const afterCurrent = currentIndex >= 0 ? queueTracks.slice(currentIndex + 1) : [];
  const upNext = afterCurrent[0] ?? null;

  const queueItemBtn = (
    track: DownloadedFile,
    idx: number,
    active: boolean,
    isNowPlaying: boolean
  ) => {
    const isDragTarget = dropQueueIdx === idx && dragQueueIdx !== idx;
    return (
      <div
        key={`${track.rel_path}_${idx}`}
        draggable={!!onReorderQueue}
        onDragStart={(e) => {
          e.dataTransfer.effectAllowed = "move";
          setDragQueueIdx(idx);
          setDropQueueIdx(null);
        }}
        onDragOver={(e) => {
          if (onReorderQueue && dragQueueIdx !== null && dragQueueIdx !== idx) {
            e.preventDefault();
            setDropQueueIdx(idx);
          }
        }}
        onDrop={(e) => {
          e.preventDefault();
          if (dragQueueIdx !== null && dragQueueIdx !== idx && onReorderQueue) {
            onReorderQueue(dragQueueIdx, idx);
          }
          setDragQueueIdx(null);
          setDropQueueIdx(null);
        }}
        onDragEnd={() => {
          setDragQueueIdx(null);
          setDropQueueIdx(null);
        }}
        className={`group relative rounded-lg transition-colors ${
          dragQueueIdx === idx ? "opacity-40" : ""
        } ${isDragTarget ? "shadow-[inset_0_2px_0_0_rgb(250,45,72)]" : ""}`}
      >
        <button
          onClick={() => {
            onPlayIndex?.(idx);
            setQueueOpen(false);
          }}
          className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-left transition-colors ${
            isNowPlaying
              ? "bg-apple-pink/15 text-apple-pink"
              : "text-zinc-200 hover:bg-white/5"
          }`}
        >
          <div className="w-7 h-7 rounded bg-[#2a2a2e] border border-white/10 flex items-center justify-center shrink-0 overflow-hidden relative">
            <Disc className="w-3.5 h-3.5 text-apple-subtext absolute inset-0 m-auto opacity-60" />
            {getApiUrl && (
              <img
                src={`${getApiUrl()}/cover?path=${encodeURIComponent(track.rel_path)}`}
                alt={track.title}
                className="w-full h-full object-cover relative z-10"
                onError={(e) => {
                  e.currentTarget.style.display = "none";
                }}
              />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[11px] font-semibold truncate">
          {isNowPlaying ? (
            <span className="flex items-center gap-1.5 text-apple-pink">
              <span className="w-1.5 h-1.5 rounded-full bg-apple-pink animate-pulse inline-block shrink-0" />
              {cleanTitle(track.title)}
            </span>
          ) : (
            cleanTitle(track.title)
          )}
            </p>
            <p className="text-[10px] text-apple-subtext truncate">{track.artist}</p>
          </div>
          {active && <Play className="w-3 h-3 text-apple-pink shrink-0" />}
        </button>
        {/* Remove from queue — appears on hover */}
        {onRemoveFromQueue && (
          <button
            onClick={(e) => {
              e.stopPropagation();
              onRemoveFromQueue(idx);
            }}
            title={t("Remove from queue")}
            className="absolute -right-1 -top-1 p-1 rounded-full bg-[#1e1e20] border border-white/10 text-apple-subtext hover:text-rose-400 hover:border-rose-500/40 transition-colors opacity-0 group-hover:opacity-100"
          >
            <X className="w-3 h-3" />
          </button>
        )}
      </div>
    );
  };

  return (
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 select-none w-max max-w-[calc(100vw-24px)]">
      {/* Floating glass capsule */}
      <Glass cornerRadius={16} className="w-auto">
        <div className="flex items-center gap-6 min-w-[min(560px,calc(100vw-48px))] px-5 py-3">
        {/* Controls */}
        <div className="flex items-center gap-3">
          <button
            onClick={onToggleShuffle}
            title={t("Shuffle")}
            className={`transition-colors ${
              shuffle
                ? "text-apple-pink"
                : "text-apple-subtext hover:text-white"
            }`}
          >
            <Shuffle className="w-3.5 h-3.5" />
          </button>

          <button onClick={onPrevTrack} className="text-white hover:opacity-80 transition-opacity">
            <SkipBack className="w-4 h-4 fill-current" />
          </button>

          <button
            onClick={onPlayToggle}
            className="w-9 h-9 rounded-full bg-white hover:scale-105 text-black flex items-center justify-center transition-all shadow-md active:scale-95"
          >
            {isPlaying ? (
              <Pause className="w-4 h-4 fill-black text-black" />
            ) : (
              <Play className="w-4 h-4 fill-black text-black ml-0.5" />
            )}
          </button>

          <button onClick={onNextTrack} className="text-white hover:opacity-80 transition-opacity">
            <SkipForward className="w-4 h-4 fill-current" />
          </button>

          <button
            onClick={onCycleRepeat}
            title={
              repeatMode === "off"
                ? t("Repeat Off")
                : repeatMode === "all"
                ? t("Repeat All")
                : t("Repeat One")
            }
            className={`transition-colors ${
              repeatMode === "off"
                ? "text-apple-subtext hover:text-white"
                : "text-apple-pink"
            }`}
          >
            {repeatMode === "one" ? (
              <Repeat1 className="w-3.5 h-3.5" />
            ) : (
              <Repeat className="w-3.5 h-3.5" />
            )}
          </button>
        </div>

        {/* Track Info Card Container matching Screenshot 1 */}
        <div className="flex-1 bg-[#1a1a1d]/90 border border-white/10 rounded-xl px-3 py-1.5 flex items-center gap-3">
          <div className="w-10 h-10 bg-[#2a2a2e] rounded-md flex items-center justify-center text-apple-subtext shrink-0 overflow-hidden shadow relative">
            <Disc className="w-5 h-5 text-apple-subtext absolute inset-0 m-auto opacity-60" />
            {getApiUrl && (
              <img
                src={`${getApiUrl()}/cover?path=${encodeURIComponent(currentTrack.rel_path)}`}
                alt={currentTrack.title}
                className="w-full h-full object-cover relative z-10"
                onError={(e) => {
                  e.currentTarget.style.display = "none";
                }}
              />
            )}
          </div>

          <div className="overflow-hidden flex-1">
            <p className="text-xs font-bold text-white truncate">{cleanTitle(currentTrack.title)}</p>
            <p className="text-[11px] text-apple-subtext truncate mt-0.5">
              {currentTrack.artist} — {currentTrack.album}
            </p>
            {/* Up next — the next track in the queue */}
            {upNext && (
              <p className="text-[10px] text-apple-pink/90 truncate mt-0.5 flex items-center gap-1">
                <span className="w-1 h-1 rounded-full bg-apple-pink inline-block shrink-0" />
                {t("Up next")}: {cleanTitle(upNext.title)}
              </p>
            )}
          </div>
        </div>

        {/* Right Tools: Lyrics 💬, Queue ≡, Volume */}
        <div className="flex items-center gap-3">
          <button
            onClick={onOpenLyrics}
            className="text-apple-pink hover:scale-110 transition-transform"
            title={t("Lyrics")}
          >
            <MessageSquareQuote className="w-5 h-5 fill-apple-pink/20" />
          </button>

          <button
            onClick={() => setQueueOpen((v) => !v)}
            className={`relative transition-colors ${
              queueOpen ? "text-apple-pink" : "text-apple-subtext hover:text-white"
            }`}
            title={t("Queue")}
          >
            <ListMusic className="w-4 h-4" />
            {queueTracks.length > 0 && (
              <span className="absolute -top-1.5 -right-1.5 min-w-3.5 h-3.5 px-0.5 rounded-full bg-apple-pink text-white text-[8px] font-bold flex items-center justify-center">
                {queueTracks.length}
              </span>
            )}
          </button>

          <div className="flex items-center gap-2">
            <button onClick={toggleMute} className="text-apple-subtext hover:text-white transition-colors">
              {isMuted || volume === 0 ? (
                <VolumeX className="w-4 h-4 text-rose-500" />
              ) : (
                <Volume2 className="w-4 h-4" />
              )}
            </button>

            <input
              type="range"
              min={0}
              max={1}
              step={0.01}
              value={isMuted ? 0 : volume}
              onChange={handleVolumeChange}
              className="w-20 h-1 bg-white/20 rounded-lg appearance-none cursor-pointer accent-apple-pink"
            />
          </div>
        </div>
        </div>

        {/* Progress bar — glued to the bottom edge of the floating capsule.
            The glass's overflow-hidden rounds its ends with the capsule. */}
        <div className="px-2.5 pt-1 pb-0 leading-none">
          <input
            type="range"
            min={0}
            max={duration || 100}
            value={currentTime}
            onChange={handleSeek}
            className="block w-full h-1 bg-white/10 rounded-full appearance-none cursor-pointer accent-apple-pink leading-none"
          />
        </div>
      </Glass>

      {/* Visible queue panel — the scroll container itself carries the rounded
          corners so they stay rounded while scrolling (the inner glass is taller
          than the viewport, so clipping must happen here). */}
      {queueOpen && (
        <div
          ref={queueRef}
          className="absolute bottom-full left-1/2 -translate-x-1/2 mb-3 w-80 max-h-[min(50vh,420px)] overflow-y-auto overflow-x-hidden rounded-[14px] scrollbar-none"
        >
          <Glass cornerRadius={14} className="w-full">
            <div className="flex items-center justify-between px-3 py-2 border-b border-white/10">
              <span className="text-[11px] font-bold text-white uppercase tracking-wider flex items-center gap-1.5">
                <ListMusic className="w-3.5 h-3.5 text-apple-pink" />
                {t("Queue")} ({queueTracks.length})
              </span>
              <button
                onClick={() => setQueueOpen(false)}
                className="p-1 rounded-full text-apple-subtext hover:text-white hover:bg-white/10 transition-colors"
                title={t("Close")}
              >
                <X className="w-3.5 h-3.5" />
              </button>
            </div>

            <div className="p-1.5 space-y-0.5">
              {/* Now playing */}
              <div className="px-2.5 pt-1.5 pb-1 text-[9px] font-semibold text-apple-subtext uppercase tracking-wider">
                {t("Now Playing")}
              </div>
              {currentIndex >= 0 && queueTracks[currentIndex] && (
                queueItemBtn(queueTracks[currentIndex], currentIndex, true, true)
              )}

              {/* Up next */}
              <div className="px-2.5 pt-2 pb-1 text-[9px] font-semibold text-apple-subtext uppercase tracking-wider">
                {t("Up Next")}
              </div>
              {afterCurrent.length === 0 ? (
                <div className="px-2.5 py-2 text-[11px] text-zinc-500">
                  {repeatMode === "all"
                    ? t("End of queue — will loop")
                    : t("End of queue")}
                </div>
              ) : (
                afterCurrent.map((track, i) =>
                  queueItemBtn(track, currentIndex + 1 + i, false, false)
                )
              )}
            </div>
          </Glass>
        </div>
      )}
    </div>
  );
};
