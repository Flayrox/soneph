"use client";

import React, { useState } from "react";
import { Download, Loader2, Search, Zap, ArrowDownUp } from "lucide-react";

interface HeaderProps {
  onDownload: (url: string, bitrate: string, order: string) => Promise<void>;
  isLoading: boolean;
  activeTasksCount: number;
  currentNav: string;
  onOpenQueue?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  onDownload,
  isLoading,
  activeTasksCount,
  currentNav,
  onOpenQueue,
}) => {
  const [url, setUrl] = useState("");
  const [bitrate, setBitrate] = useState("320k");
  const [order, setOrder] = useState("reverse");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || isLoading) return;
    await onDownload(url.trim(), bitrate, order);
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

      {/* Center Download, Priority Order & Quality Selector */}
      <div className="flex items-center gap-2 sm:gap-3">
        {/* Download Order Priority Dropdown */}
        <div className="relative flex items-center bg-[#242428] border border-white/10 rounded-full px-2.5 py-1 text-xs text-white">
          <ArrowDownUp className="w-3.5 h-3.5 text-apple-pink mr-1.5" />
          <select
            value={order}
            onChange={(e) => setOrder(e.target.value)}
            className="bg-transparent text-xs font-semibold text-white focus:outline-none cursor-pointer pr-1"
          >
            <option value="reverse" className="bg-[#242428] text-white">Newest Added First</option>
            <option value="normal" className="bg-[#242428] text-white">Original Playlist Order</option>
          </select>
        </div>

        {/* Bitrate Selector Dropdown */}
        <div className="relative flex items-center bg-[#242428] border border-white/10 rounded-full px-2.5 py-1 text-xs text-white">
          <Zap className="w-3.5 h-3.5 text-apple-pink mr-1.5" />
          <select
            value={bitrate}
            onChange={(e) => setBitrate(e.target.value)}
            className="bg-transparent text-xs font-semibold text-white focus:outline-none cursor-pointer pr-1"
          >
            <option value="320k" className="bg-[#242428] text-white">320 kbps (HQ • 10MB)</option>
            <option value="192k" className="bg-[#242428] text-white">192 kbps (Balanced • 5MB)</option>
            <option value="128k" className="bg-[#242428] text-white">128 kbps (Fast • 2.5MB)</option>
          </select>
        </div>

        {/* Import Input */}
        <form onSubmit={handleSubmit} className="relative flex items-center w-64 sm:w-72">
          <Search className="w-4 h-4 absolute left-3.5 text-apple-subtext" />
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="Paste  Link..."
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
      </div>

      {/* Active Tasks Pill */}
      <div>
        {activeTasksCount > 0 && (
          <button
            onClick={onOpenQueue}
            className="flex items-center gap-2 bg-apple-pink/20 text-apple-pink hover:bg-apple-pink/30 border border-apple-pink/30 px-3 py-1 rounded-full text-xs font-semibold animate-pulse transition-all cursor-pointer shadow-md"
            title="Click to view active download queue details"
          >
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
            <span>{activeTasksCount} Syncing...</span>
          </button>
        )}
      </div>
    </header>
  );
};
