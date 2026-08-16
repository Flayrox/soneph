import React, { useState, useEffect } from "react";
import {
  Play,
  Pause,
  SkipBack,
  SkipForward,
  Shuffle,
  Repeat,
  Volume2,
  VolumeX,
  MessageSquareQuote,
  ListMusic,
  Disc,
} from "lucide-react";
import { DownloadedFile } from "./TrackList";

interface PlayerProps {
  currentTrack: DownloadedFile | null;
  isPlaying: boolean;
  onPlayToggle: () => void;
  onPrevTrack: () => void;
  onNextTrack: () => void;
  onOpenLyrics: () => void;
  audioRef: React.RefObject<HTMLAudioElement>;
  onTimeUpdate?: (time: number) => void;
  getApiUrl?: () => string;
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
}) => {
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.8);
  const [isMuted, setIsMuted] = useState(false);

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

  return (
    <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 select-none">
      {/* l'app Musique Floating Liquid Glass Capsule */}
      <div className="bg-[#2a2a2e]/85 backdrop-blur-3xl border border-white/15 rounded-2xl px-5 py-3 shadow-[0_20px_50px_rgba(0,0,0,0.8)] flex items-center gap-6 min-w-[560px]">
        {/* Controls */}
        <div className="flex items-center gap-3">
          <button className="text-apple-subtext hover:text-white transition-colors">
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

          <button className="text-apple-subtext hover:text-white transition-colors">
            <Repeat className="w-3.5 h-3.5" />
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
            <p className="text-xs font-bold text-white truncate">{currentTrack.title}</p>
            <p className="text-[11px] text-apple-subtext truncate mt-0.5">
              {currentTrack.artist} — {currentTrack.album}
            </p>
          </div>
        </div>

        {/* Right Tools: Lyrics 💬, Queue ≡, Volume */}
        <div className="flex items-center gap-3">
          <button
            onClick={onOpenLyrics}
            className="text-apple-pink hover:scale-110 transition-transform"
            title="l'app Musique Lyrics"
          >
            <MessageSquareQuote className="w-5 h-5 fill-apple-pink/20" />
          </button>

          <button className="text-apple-subtext hover:text-white transition-colors">
            <ListMusic className="w-4 h-4" />
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

      {/* Embedded Progress Scrub Line under capsule */}
      <div className="mt-1 px-4">
        <input
          type="range"
          min={0}
          max={duration || 100}
          value={currentTime}
          onChange={handleSeek}
          className="w-full h-1 bg-white/10 rounded-full appearance-none cursor-pointer accent-apple-pink"
        />
      </div>
    </div>
  );
};
