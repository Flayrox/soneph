import React, { useState, useRef, useEffect } from "react";
import LiquidGlass from "liquid-glass-react";
import { Download, Loader2, Search, SlidersHorizontal, Check } from "lucide-react";
import { useI18n, LangToggle } from "@/i18n";

interface AppHeaderProps {
  onDownload: (url: string, bitrate: string, order: string) => Promise<void>;
  isLoading: boolean;
  activeTasksCount: number;
  currentNav: string;
  currentPlaylistName?: string | null;
  onOpenQueue?: () => void;
}

export const AppHeader: React.FC<AppHeaderProps> = ({
  onDownload,
  isLoading,
  activeTasksCount,
  currentNav,
  currentPlaylistName,
  onOpenQueue,
}) => {
  const { t } = useI18n();
  const [url, setUrl] = useState("");
  const [bitrate, setBitrate] = useState("320k");
  const [order, setOrder] = useState("reverse");
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const popoverRef = useRef<HTMLDivElement | null>(null);

  // Click outside to close settings popover
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setIsSettingsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim() || isLoading) return;
    await onDownload(url.trim(), bitrate, order);
    setUrl("");
  };

  const getNavTitle = () => {
    if (currentNav.startsWith("pl:")) {
      return currentPlaylistName || t("Playlist");
    }
    switch (currentNav) {
      case "home":
        return t("Home");
      case "downloads":
        return t("Downloads");
      case "lyrics":
        return t("Lyrics");
      case "sync":
        return t("Sync & Settings");
      case "liked":
        return t("Liked tracks");
      default:
        return t("All Music");
    }
  };

  return (
    <header className="h-14 bg-[#161618]/90 backdrop-blur-2xl border-b border-white/10 px-6 flex items-center justify-between sticky top-0 z-30 select-none">
      {/* Page Title */}
      <div className="flex items-center gap-3">
        <h1 className="text-lg font-bold text-white tracking-tight">{getNavTitle()}</h1>
      </div>

      {/* Center Download Input & Clean Options Popover */}
      <div className="flex items-center gap-2">
        {/* Import Link Form */}
        <LiquidGlass cornerRadius={999} padding="0 6px" blurAmount={0.015} displacementScale={25}>
        <form onSubmit={handleSubmit} className="relative flex items-center w-64 sm:w-80">
          <Search className="w-4 h-4 absolute left-3.5 text-apple-subtext" />
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t("Paste Link...")}
            className="w-full bg-[#242428]/50 border border-white/10 focus:border-apple-pink rounded-full py-1.5 pl-10 pr-24 text-xs text-white placeholder-apple-subtext focus:outline-none transition-all shadow-inner"
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
            <span>{t("Import")}</span>
          </button>
        </form>
        </LiquidGlass>

        <LangToggle />

        {/* Clean Apple macOS Options Menu Trigger */}
        <div className="relative" ref={popoverRef}>
          <button
            type="button"
            onClick={() => setIsSettingsOpen(!isSettingsOpen)}
            className={`w-8 h-8 rounded-full flex items-center justify-center border transition-all ${
              isSettingsOpen
                ? "bg-apple-pink text-white border-apple-pink shadow-md"
                : "bg-[#242428] text-apple-subtext hover:text-white border-white/10 hover:border-white/20"
            }`}
            title={t("Download Preferences")}
          >
            <SlidersHorizontal className="w-3.5 h-3.5" />
          </button>

          {/* Native macOS Style Popover Menu */}
          {isSettingsOpen && (
            <div className="absolute right-0 top-10 w-56 bg-[#1e1e22]/95 backdrop-blur-2xl border border-white/10 rounded-xl p-1.5 shadow-2xl z-50 text-xs select-none space-y-1">
              <div className="px-2.5 py-1 text-[10px] font-semibold text-apple-subtext/60 uppercase tracking-wider">
                {t("Audio Quality")}
              </div>
              <button
                type="button"
                onClick={() => {
                  setBitrate("320k");
                  setIsSettingsOpen(false);
                }}
                className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-xs font-medium transition-colors ${
                  bitrate === "320k" ? "bg-apple-pink text-white" : "text-zinc-200 hover:bg-white/10"
                }`}
              >
                <span>{t("320 kbps (High Quality)")}</span>
                {bitrate === "320k" && <Check className="w-3.5 h-3.5" />}
              </button>
              <button
                type="button"
                onClick={() => {
                  setBitrate("192k");
                  setIsSettingsOpen(false);
                }}
                className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-xs font-medium transition-colors ${
                  bitrate === "192k" ? "bg-apple-pink text-white" : "text-zinc-200 hover:bg-white/10"
                }`}
              >
                <span>{t("192 kbps (Standard)")}</span>
                {bitrate === "192k" && <Check className="w-3.5 h-3.5" />}
              </button>
              <button
                type="button"
                onClick={() => {
                  setBitrate("128k");
                  setIsSettingsOpen(false);
                }}
                className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-xs font-medium transition-colors ${
                  bitrate === "128k" ? "bg-apple-pink text-white" : "text-zinc-200 hover:bg-white/10"
                }`}
              >
                <span>{t("128 kbps (Compact)")}</span>
                {bitrate === "128k" && <Check className="w-3.5 h-3.5" />}
              </button>

              <div className="my-1 border-t border-white/10" />

              <div className="px-2.5 py-1 text-[10px] font-semibold text-apple-subtext/60 uppercase tracking-wider">
                {t("Import Order")}
              </div>
              <button
                type="button"
                onClick={() => {
                  setOrder("reverse");
                  setIsSettingsOpen(false);
                }}
                className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-xs font-medium transition-colors ${
                  order === "reverse" ? "bg-apple-pink text-white" : "text-zinc-200 hover:bg-white/10"
                }`}
              >
                <span>{t("Newest Added First")}</span>
                {order === "reverse" && <Check className="w-3.5 h-3.5" />}
              </button>
              <button
                type="button"
                onClick={() => {
                  setOrder("normal");
                  setIsSettingsOpen(false);
                }}
                className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left text-xs font-medium transition-colors ${
                  order === "normal" ? "bg-apple-pink text-white" : "text-zinc-200 hover:bg-white/10"
                }`}
              >
                <span>{t("Original Playlist Order")}</span>
                {order === "normal" && <Check className="w-3.5 h-3.5" />}
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Active Tasks Pill */}
      <div>
        {activeTasksCount > 0 && (
          <button
            onClick={onOpenQueue}
            className="flex items-center gap-2 bg-apple-pink/20 text-apple-pink hover:bg-apple-pink/30 border border-apple-pink/30 px-3 py-1 rounded-full text-xs font-semibold transition-all cursor-pointer shadow-md"
            title={t("Click to view active download queue details")}
          >
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
            <span>{activeTasksCount} {t("Syncing...")}</span>
          </button>
        )}
      </div>
    </header>
  );
};
