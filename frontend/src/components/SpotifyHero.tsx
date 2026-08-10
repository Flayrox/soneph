"use client";

import React from "react";
import { Play, Pause, Shuffle, Download, MoreHorizontal, Search, CheckCircle2 } from "lucide-react";

interface HeroProps {
  totalFiles: number;
  totalSizeBytes: number;
  isPlaying: boolean;
  onPlayToggle: () => void;
  searchTerm: string;
  onSearchChange: (term: string) => void;
}

export const Hero: React.FC<HeroProps> = ({
  totalFiles,
  totalSizeBytes,
  isPlaying,
  onPlayToggle,
  searchTerm,
  onSearchChange,
}) => {
  const formatSize = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  return (
    <div className="select-none">
      {/* Gradient Header Banner */}
      <div className="bg-gradient-to-b from-[#1e3a8a] via-[#1e1b4b] to--base p-6 sm:p-8 pt-4 flex flex-col sm:flex-row items-start sm:items-end gap-6 shadow-2xl">
        {/* Playlist Cover Art Quad / Image */}
        <div className="w-44 h-44 sm:w-52 sm:h-52 bg--elevated rounded-md shadow-2xl overflow-hidden shrink-0 flex items-center justify-center border border-white/10 group relative">
          <div className="grid grid-cols-2 grid-rows-2 w-full h-full">
            <div className="bg-rose-600/40 flex items-center justify-center text-white font-black text-xl">S</div>
            <div className="bg-pink-600/40 flex items-center justify-center text-white font-black text-xl">O</div>
            <div className="bg-red-600/40 flex items-center justify-center text-rose-300 font-black text-xl">N</div>
            <div className="bg-rose-900/60 flex items-center justify-center text-rose-400 font-black text-lg font-mono">ephe</div>
          </div>
        </div>

        {/* Info Metadata */}
        <div className="flex flex-col gap-2">
          <span className="text-xs font-bold uppercase tracking-wider text-white">Public Playlist</span>
          <h1 className="text-4xl sm:text-6xl font-black text-white tracking-tight">
            sonephe Library
          </h1>

          <div className="flex items-center gap-2 text-xs sm:text-sm text--subtext mt-2">
            <span className="font-bold text-white">son<span className="text-rose-500 font-extrabold">ephe</span></span>
            <span>•</span>
            <span>{totalFiles} songs, {formatSize(totalSizeBytes)}</span>
            <span>•</span>
            <span className="text--green font-medium flex items-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5 inline" /> l'app Musique iCloud Synced
            </span>
          </div>
        </div>
      </div>

      {/* Control Bar Actions */}
      <div className="bg--base px-6 sm:px-8 py-5 flex items-center justify-between gap-4">
        <div className="flex items-center gap-5">
          {/* Big  Green Play Button */}
          <button
            onClick={onPlayToggle}
            className="w-14 h-14 rounded-full bg--green hover:bg--greenHover text-black flex items-center justify-center shadow-xl hover:scale-105 transition-all active:scale-95"
            title={isPlaying ? "Pause" : "Play All"}
          >
            {isPlaying ? (
              <Pause className="w-6 h-6 fill-black" />
            ) : (
              <Play className="w-6 h-6 fill-black ml-1" />
            )}
          </button>

          <button className="text--subtext hover:text-white transition-colors">
            <Shuffle className="w-6 h-6" />
          </button>

          <button className="text--subtext hover:text-white transition-colors">
            <Download className="w-6 h-6 text--green" />
          </button>

          <button className="text--subtext hover:text-white transition-colors">
            <MoreHorizontal className="w-6 h-6" />
          </button>
        </div>

        {/* Filter Input */}
        <div className="relative flex items-center w-56">
          <Search className="w-4 h-4 absolute left-3 text--subtext" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search in playlist"
            className="w-full bg--elevated hover:bg--highlight border border-transparent focus:border--border rounded-full py-1.5 pl-9 pr-4 text-xs text-white placeholder--subtext focus:outline-none transition-all"
          />
        </div>
      </div>
    </div>
  );
};
