"use client";

import React, { useState } from "react";
import { Download, Loader2, Search, SlidersHorizontal } from "lucide-react";

interface HeaderProps {
  onDownload: (url: string) => Promise<void>;
  isLoading: boolean;
  activeTasksCount: number;
  currentNav: string;
}

export const Header: React.FC<HeaderProps> = ({
  onDownload,
  isLoading,
  activeTasksCount,
  currentNav,
}) => {
  const [url, setUrl] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || isLoading) return;
    await onDownload(url.trim());
    setUrl("");
  };

  const getNavTitle = () => {
    switch (currentNav) {
      case "recently_added":
        return "Recently Added";
      case "artists":
        return "Artists";
      case "albums":
        return "Albums";
      case "pins":
        return "Pins";
      default:
        return "Songs";
    }
  };

  return (
    <header className="h-14 bg-[#161618]/90 backdrop-blur-2xl border-b border-white/10 px-6 flex items-center justify-between sticky top-0 z-30 select-none">
      {/* Title */}
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-bold text-white tracking-tight">{getNavTitle()}</h1>
      </div>

      {/* Center Download & Search Pill */}
      <div className="flex items-center gap-3">
        <form onSubmit={handleSubmit} className="relative flex items-center w-72 sm:w-96">
          <Search className="w-4 h-4 absolute left-3.5 text-apple-subtext" />
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="Paste  Track, Playlist or Artist URL..."
            className="w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-full py-1.5 pl-10 pr-24 text-xs text-white placeholder-apple-subtext focus:outline-none transition-all shadow-inner"
          />
          <button
            type="submit"
            disabled={isLoading || !url.trim()}
            className="absolute right-1 px-3 py-1 bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-50 text-white font-semibold rounded-full text-xs flex items-center gap-1 transition-all shadow-md active:scale-95"
          >
            {isLoading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Download className="w-3.5 h-3.5" />
            )}
            <span>Import</span>
          </button>
        </form>

        <button className="w-8 h-8 rounded-full bg-[#242428] hover:bg-white/10 text-apple-subtext hover:text-white flex items-center justify-center transition-colors border border-white/5">
          <SlidersHorizontal className="w-4 h-4" />
        </button>
      </div>

      {/* Active Tasks Pill */}
      <div>
        {activeTasksCount > 0 && (
          <div className="flex items-center gap-2 bg-apple-pink/20 text-apple-pink border border-apple-pink/30 px-3 py-1 rounded-full text-xs font-semibold animate-pulse">
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
            <span>{activeTasksCount} Syncing...</span>
          </div>
        )}
      </div>
    </header>
  );
};
