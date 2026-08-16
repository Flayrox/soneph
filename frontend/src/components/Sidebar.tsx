import React from "react";
import {
  Search,
  Home,
  Radio,
  Clock,
  User,
  Disc,
  Music,
  Video,
  ListMusic,
  Heart,
  Pin,
  FolderCheck,
  Loader2,
  FileText,
  Settings2,
} from "lucide-react";
import { LyricsRetryPanel } from "./LyricsRetryPanel";
import { useI18n } from "@/i18n";

interface SidebarProps {
  totalFiles: number;
  syncedCount?: number;
  activeFilter: string;
  onFilterChange: (filter: string) => void;
  activeNav: string;
  onNavChange: (nav: string) => void;
  activeTasksCount?: number;
}

export const Sidebar: React.FC<SidebarProps> = ({
  totalFiles,
  syncedCount = 0,
  activeFilter,
  onFilterChange,
  activeNav,
  onNavChange,
  activeTasksCount = 0,
}) => {
  const { t } = useI18n();
  const libraryItems = [
    { id: "songs", label: t("Songs"), icon: Music, badge: totalFiles },
    { id: "lyrics", label: t("Lyrics"), icon: FileText, badge: `${syncedCount}/${totalFiles}` },
    { id: "recently_added", label: t("Recently Added"), icon: Clock },
    { id: "artists", label: t("Artists"), icon: User },
    { id: "albums", label: t("Albums"), icon: Disc },
    { id: "pins", label: t("Pins"), icon: Pin },
    { id: "sync", label: t("Sync & Settings"), icon: Settings2 },
  ];

  if (activeTasksCount > 0) {
    libraryItems.unshift({
      id: "downloading",
      label: t("Active Syncs"),
      icon: Loader2,
      badge: activeTasksCount,
    });
  }

  const playlistItems = [
    { id: "all_playlists", label: t("All Playlists"), icon: ListMusic },
    { id: "favorite_songs", label: t("Favorite Songs"), icon: Heart },
  ];

  return (
    <aside className="w-64 bg-[#1e1e20]/80 backdrop-blur-2xl border-r border-white/10 h-full flex flex-col justify-between p-3 select-none">
      <div className="space-y-4 overflow-y-auto scrollbar-none pr-1">
        {/* Search Input Bar */}
        <div className="relative px-1 pt-1">
          <Search className="w-4 h-4 absolute left-3.5 top-3 text-apple-subtext" />
          <input
            type="text"
            value={activeFilter}
            onChange={(e) => onFilterChange(e.target.value)}
            placeholder={t("Search")}
            className="w-full bg-[#2a2a2d] border border-white/5 rounded-lg py-1.5 pl-9 pr-3 text-xs text-white placeholder-apple-subtext focus:outline-none focus:border-apple-pink/50 transition-all"
          />
        </div>

        {/* Main Nav */}
        <div className="space-y-0.5 text-xs font-medium">
          <button
            onClick={() => onNavChange("home")}
            className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md transition-colors ${
              activeNav === "home"
                ? "bg-apple-pink text-white font-semibold"
                : "text-zinc-300 hover:bg-white/5"
            }`}
          >
            <Home className="w-4 h-4" />
            <span>{t("Home")}</span>
          </button>
          <button
            onClick={() => onNavChange("radio")}
            className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md transition-colors ${
              activeNav === "radio"
                ? "bg-apple-pink text-white font-semibold"
                : "text-zinc-300 hover:bg-white/5"
            }`}
          >
            <Radio className="w-4 h-4" />
            <span>{t("Radio")}</span>
          </button>
        </div>

        {/* Library Section */}
        <div className="space-y-1">
          <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
            {t("Library")}
          </span>
          <div className="space-y-0.5 text-xs font-medium">
            {libraryItems.map((item) => {
              const Icon = item.icon;
              const isActive = activeNav === item.id;
              const isSyncing = item.id === "downloading";

              return (
                <button
                  key={item.id}
                  onClick={() => onNavChange(item.id)}
                  className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                    isActive
                      ? "bg-apple-pink text-white font-semibold shadow-sm"
                      : isSyncing
                      ? "bg-apple-pink/15 text-apple-pink font-semibold border border-apple-pink/30 animate-pulse"
                      : "text-zinc-300 hover:bg-white/5"
                  }`}
                >
                  <div className="flex items-center gap-2.5">
                    <Icon className={`w-4 h-4 ${isSyncing ? "animate-spin text-apple-pink" : ""}`} />
                    <span>{item.label}</span>
                  </div>
                  {item.badge !== undefined && (
                    <span
                      className={`text-[10px] px-2 py-0.5 rounded-full font-semibold ${
                        isSyncing
                          ? "bg-apple-pink text-white font-bold"
                          : "bg-white/10 text-zinc-300"
                      }`}
                    >
                      {item.badge}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </div>

        {/* Playlists Section */}
        <div className="space-y-1">
          <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
            {t("Playlists")}
          </span>
          <div className="space-y-0.5 text-xs font-medium">
            {playlistItems.map((item) => {
              const Icon = item.icon;
              const isActive = activeNav === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => onNavChange(item.id)}
                  className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-md transition-colors ${
                    isActive
                      ? "bg-apple-pink text-white font-semibold"
                      : "text-zinc-300 hover:bg-white/5"
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  <span>{item.label}</span>
                </button>
              );
            })}
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
        <span className="text-[10px] bg-apple-pink/20 text-apple-pink px-2 py-0.5 rounded-full font-semibold">
          {t("Synced")}
        </span>
      </div>
    </aside>
  );
};
