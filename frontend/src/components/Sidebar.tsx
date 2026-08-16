import React, { useState } from "react";
import LiquidGlass from "liquid-glass-react";
import {
  Search,
  Music,
  FileText,
  FolderCheck,
  Settings2,
  ListMusic,
  Plus,
  DownloadCloud,
  X,
  Check,
  Home as HomeIcon,
  Heart,
  BarChart3,
  Puzzle,
} from "lucide-react";
import { LyricsRetryPanel } from "./LyricsRetryPanel";
import type { PlaylistSummary } from "@/types";
import { useI18n } from "@/i18n";
import { useModules } from "@/modules";

interface SidebarProps {
  totalFiles: number;
  syncedCount?: number;
  activeFilter: string;
  onFilterChange: (filter: string) => void;
  activeNav: string;
  onNavChange: (nav: string) => void;
  activeTasksCount?: number;
  playlists?: PlaylistSummary[];
  onCreatePlaylist?: (name: string) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  totalFiles,
  syncedCount = 0,
  activeFilter,
  onFilterChange,
  activeNav,
  onNavChange,
  activeTasksCount = 0,
  playlists = [],
  onCreatePlaylist,
}) => {
  const { t } = useI18n();
  const { isEnabled } = useModules();
  const importEnabled = isEnabled("import");
  const statsEnabled = isEnabled("stats");
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");

  const submitNewPlaylist = (e: React.FormEvent) => {
    e.preventDefault();
    if (newName.trim() && onCreatePlaylist) {
      onCreatePlaylist(newName.trim());
      setNewName("");
      setCreating(false);
    }
  };

  return (
    <aside className="w-64 bg-[#1e1e20]/80 backdrop-blur-2xl border-r border-white/10 h-full flex flex-col justify-between p-3 select-none relative z-10">
      <div className="space-y-4 overflow-y-auto scrollbar-none pr-1">
        {/* Search Input Bar */}
        <div className="relative px-1 pt-1">
          <Search className="w-4 h-4 absolute left-3.5 top-3 text-apple-subtext" />
          <LiquidGlass cornerRadius={999} padding="0px" blurAmount={0.015} displacementScale={20}>
            <input
              type="text"
              value={activeFilter}
              onChange={(e) => onFilterChange(e.target.value)}
              placeholder={t("Search")}
              className="w-full bg-[#2a2a2d]/60 border border-white/5 rounded-full py-1.5 pl-9 pr-3 text-xs text-white placeholder-apple-subtext focus:outline-none focus:border-apple-pink/50 transition-all"
            />
          </LiquidGlass>
        </div>

        {/* Music Section */}
        <div className="space-y-1">
          <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
            {t("Music")}
          </span>
          <div className="space-y-0.5 text-xs font-medium">
            <button
              onClick={() => onNavChange("home")}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "home"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <HomeIcon className="w-4 h-4" />
                <span>{t("Home")}</span>
              </div>
            </button>
            <button
              onClick={() => onNavChange("songs")}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "songs"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <Music className="w-4 h-4" />
                <span>{t("All Music")}</span>
              </div>
              <span
                className={`text-[10px] px-2 py-0.5 rounded-full font-semibold ${
                  activeNav === "songs" ? "bg-white/20 text-white" : "bg-white/10 text-zinc-300"
                }`}
              >
                {totalFiles}
              </span>
            </button>
            <button
              onClick={() => onNavChange("liked")}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "liked"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <Heart className="w-4 h-4" />
                <span>{t("Liked tracks")}</span>
              </div>
            </button>
            {statsEnabled && (
              <button
                onClick={() => onNavChange("stats")}
                className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md transition-colors ${
                  activeNav === "stats"
                    ? "bg-apple-pink text-white font-semibold shadow-sm"
                    : "text-zinc-300 hover:bg-white/5"
                }`}
              >
                <BarChart3 className="w-4 h-4" />
                <span>{t("Stats")}</span>
              </button>
            )}
          </div>
        </div>

        {/* Playlists Section */}
        <div className="space-y-1">
          <div className="flex items-center justify-between px-3">
            <span className="text-[11px] font-semibold text-apple-subtext uppercase tracking-wider">
              {t("Playlists")}
            </span>
            <button
              onClick={() => {
                setCreating((c) => !c);
                setNewName("");
              }}
              className="p-0.5 rounded text-apple-subtext hover:text-white transition-colors"
              title={t("New Playlist")}
            >
              {creating ? <X className="w-3.5 h-3.5" /> : <Plus className="w-3.5 h-3.5" />}
            </button>
          </div>

          {creating && (
            <form onSubmit={submitNewPlaylist} className="px-2">
              <input
                autoFocus
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder={t("Playlist Name")}
                className="w-full bg-[#2a2a2d] border border-white/10 focus:border-apple-pink rounded-lg px-2 py-1.5 text-xs text-white placeholder-apple-subtext focus:outline-none"
              />
            </form>
          )}

          <div className="space-y-0.5 text-xs font-medium">
            {playlists.map((p) => {
              const isActive = activeNav === `pl:${p.id}`;
              return (
                <button
                  key={p.id}
                  onClick={() => onNavChange(`pl:${p.id}`)}
                  className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                    isActive
                      ? "bg-apple-pink text-white font-semibold shadow-sm"
                      : "text-zinc-300 hover:bg-white/5"
                  }`}
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <ListMusic className="w-4 h-4 shrink-0" />
                    <span className="truncate">{p.name}</span>
                  </div>
                  <span
                    className={`text-[10px] px-2 py-0.5 rounded-full font-semibold shrink-0 ${
                      isActive ? "bg-white/20 text-white" : "bg-white/10 text-zinc-300"
                    }`}
                  >
                    {p.track_count}
                  </span>
                </button>
              );
            })}
            {playlists.length === 0 && !creating && (
              <div className="px-3 py-1 text-[11px] text-zinc-500">{t("No playlists yet")}</div>
            )}
          </div>
        </div>

        {/* Downloads Section — part of the Import module */}
        {importEnabled && (
          <div className="space-y-1">
            <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
              {t("Downloads")}
            </span>
            <button
              onClick={() => onNavChange("downloads")}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "downloads"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : activeTasksCount > 0
                  ? "bg-apple-pink/15 text-apple-pink font-semibold border border-apple-pink/30 animate-pulse"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <DownloadCloud className={`w-4 h-4 ${activeTasksCount > 0 ? "animate-bounce" : ""}`} />
                <span>{t("Downloads")}</span>
              </div>
              {activeTasksCount > 0 && (
                <span className="text-[10px] bg-apple-pink text-white font-bold px-2 py-0.5 rounded-full">
                  {activeTasksCount}
                </span>
              )}
            </button>
          </div>
        )}

        {/* Library Section */}
        <div className="space-y-1">
          <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
            {t("Library")}
          </span>
          <div className="space-y-0.5 text-xs font-medium">
            <button
              onClick={() => onNavChange("lyrics")}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "lyrics"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <div className="flex items-center gap-2.5">
                <FileText className="w-4 h-4" />
                <span>{t("Lyrics")}</span>
              </div>
              <span
                className={`text-[10px] px-2 py-0.5 rounded-full font-semibold ${
                  activeNav === "lyrics" ? "bg-white/20 text-white" : "bg-white/10 text-zinc-300"
                }`}
              >
                {syncedCount}/{totalFiles}
              </span>
            </button>
            <button
              onClick={() => onNavChange("sync")}
              className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "sync"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <Settings2 className="w-4 h-4" />
              <span>{t("Sync & Settings")}</span>
            </button>
            <button
              onClick={() => onNavChange("marketplace")}
              className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md transition-colors ${
                activeNav === "marketplace"
                  ? "bg-apple-pink text-white font-semibold shadow-sm"
                  : "text-zinc-300 hover:bg-white/5"
              }`}
            >
              <Puzzle className="w-4 h-4" />
              <span>{t("Marketplace")}</span>
            </button>
          </div>
        </div>
      </div>

      {/* Lyrics Retry Panel */}
      <div className="px-1 pb-2">
        <LyricsRetryPanel />
      </div>

      {/* Syncthing P2P Status Footer */}
      <div className="pt-3 border-t border-white/10 px-2 flex items-center justify-between text-xs text-apple-subtext">
        <div className="flex items-center gap-2">
          <FolderCheck className="w-4 h-4 text-apple-pink" />
          <span>{t("Auto-Sync")}</span>
        </div>
        <LiquidGlass cornerRadius={999} padding="0px" blurAmount={0.015} displacementScale={20}>
          <span className="flex items-center gap-1 text-[10px] bg-apple-pink/20 text-apple-pink px-2 py-0.5 rounded-full font-semibold">
            <Check className="w-3 h-3 inline" /> {t("Synced")}
          </span>
        </LiquidGlass>
      </div>

    </aside>
  );
};
