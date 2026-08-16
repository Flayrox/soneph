import React, { useState } from "react";
import { Search, Music, Play, Pause, RefreshCw, FileText } from "lucide-react";
import type { DownloadedFile } from "@/types";
import { useI18n } from "@/i18n";

interface LyricsManagerViewProps {
  files: DownloadedFile[];
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onPlayTrack: (relPath: string) => void;
  onSelectTrack: (track: DownloadedFile) => void;
  getApiUrl: () => string;
  onRefreshFiles: () => void;
}

export const LyricsManagerView: React.FC<LyricsManagerViewProps> = ({
  files,
  currentPlayingPath,
  isPlaying,
  onPlayTrack,
  onSelectTrack,
  getApiUrl,
  onRefreshFiles,
}) => {
  const { t } = useI18n();
  const [filterType, setFilterType] = useState<"all" | "synced" | "unsynced" | "missing">("all");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [isUpgrading, setIsUpgrading] = useState<boolean>(false);

  const syncedCount = files.filter((f) => f.lyrics_type === "synced").length;
  const unsyncedCount = files.filter((f) => f.lyrics_type === "unsynced").length;
  const missingCount = files.filter((f) => f.lyrics_type === "none" || !f.has_lyrics).length;
  const totalCount = files.length;

  const filteredFiles = files.filter((f) => {
    if (filterType === "synced" && f.lyrics_type !== "synced") return false;
    if (filterType === "unsynced" && f.lyrics_type !== "unsynced") return false;
    if (filterType === "missing" && f.lyrics_type !== "none" && f.has_lyrics) return false;

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      return (
        f.title.toLowerCase().includes(q) ||
        f.artist.toLowerCase().includes(q) ||
        f.album.toLowerCase().includes(q)
      );
    }
    return true;
  });

  const handleUpgradeAllLyrics = async () => {
    setIsUpgrading(true);
    try {
      const res = await fetch(`${getApiUrl()}/lyrics/retry`, { method: "POST" });
      if (res.ok) {
        setTimeout(() => {
          onRefreshFiles();
          setIsUpgrading(false);
        }, 3000);
      } else {
        setIsUpgrading(false);
      }
    } catch {
      setIsUpgrading(false);
    }
  };

  return (
    <div className="w-full text-zinc-200 select-none font-sans">
      {/* Sub-Header Controls */}
      <div className="px-6 py-4 border-b border-white/10 flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-[#161618]">
        {/* Segmented Filter Pills */}
        <div className="flex items-center gap-1 bg-[#242428] p-1 rounded-lg border border-white/5 text-xs font-medium">
          <button
            onClick={() => setFilterType("all")}
            className={`px-3 py-1 rounded-md transition-all ${
              filterType === "all"
                ? "bg-apple-pink text-white font-semibold shadow-sm"
                : "text-apple-subtext hover:text-white"
            }`}
          >
            {t("All")} ({totalCount})
          </button>
          <button
            onClick={() => setFilterType("synced")}
            className={`px-3 py-1 rounded-md transition-all ${
              filterType === "synced"
                ? "bg-apple-pink text-white font-semibold shadow-sm"
                : "text-apple-subtext hover:text-white"
            }`}
          >
            {t("Synced")} ({syncedCount})
          </button>
          <button
            onClick={() => setFilterType("unsynced")}
            className={`px-3 py-1 rounded-md transition-all ${
              filterType === "unsynced"
                ? "bg-apple-pink text-white font-semibold shadow-sm"
                : "text-apple-subtext hover:text-white"
            }`}
          >
            {t("Plain")} ({unsyncedCount})
          </button>
          <button
            onClick={() => setFilterType("missing")}
            className={`px-3 py-1 rounded-md transition-all ${
              filterType === "missing"
                ? "bg-apple-pink text-white font-semibold shadow-sm"
                : "text-apple-subtext hover:text-white"
            }`}
          >
            {t("Missing")} ({missingCount})
          </button>
        </div>

        {/* Action Button & Search */}
        <div className="flex items-center gap-3">
          <div className="relative w-48 sm:w-60">
            <Search className="w-3.5 h-3.5 absolute left-3 top-2.5 text-apple-subtext" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={t("Search...")}
              className="w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg py-1.5 pl-8 pr-3 text-xs text-white placeholder-apple-subtext focus:outline-none transition-all"
            />
          </div>

          <button
            onClick={handleUpgradeAllLyrics}
            disabled={isUpgrading}
            className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-apple-pink hover:bg-apple-pinkHover text-white font-semibold text-xs transition-all shadow-md disabled:opacity-50 shrink-0"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isUpgrading ? "animate-spin" : ""}`} />
            <span>{isUpgrading ? t("Upgrading...") : t("Sync Lyrics")}</span>
          </button>
        </div>
      </div>

      {/* Track Table */}
      <table className="w-full text-left border-collapse text-xs">
        <thead>
          <tr className="border-b border-white/10 text-apple-subtext font-semibold uppercase text-[10px] tracking-wider">
            <th className="py-2.5 px-4 w-12 text-center">#</th>
            <th className="py-2.5 px-4">{t("Title")}</th>
            <th className="py-2.5 px-4 hidden md:table-cell">{t("Album")}</th>
            <th className="py-2.5 px-4 text-center w-32">{t("Status")}</th>
            <th className="py-2.5 px-4 text-right w-24"></th>
          </tr>
        </thead>

        <tbody>
          {filteredFiles.map((file, idx) => {
            const isSelected = currentPlayingPath === file.rel_path;
            const isThisPlaying = isSelected && isPlaying;

            const isSynced = file.lyrics_type === "synced";
            const isUnsynced = file.lyrics_type === "unsynced";

            return (
              <tr
                key={file.rel_path}
                onClick={() => onSelectTrack(file)}
                className={`border-b border-white/5 group hover:bg-white/5 transition-colors cursor-pointer ${
                  isSelected ? "bg-apple-pink/15 text-white" : "text-zinc-300"
                }`}
              >
                {/* Play Button */}
                <td className="py-2.5 px-4 text-center text-apple-subtext font-medium group-hover:text-white">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onPlayTrack(file.rel_path);
                    }}
                    className="flex items-center justify-center w-6 h-6 mx-auto rounded-full hover:bg-apple-pink/20 transition-colors"
                  >
                    {isThisPlaying ? (
                      <div className="flex items-end justify-center gap-0.5 h-3.5">
                        <span className="w-0.5 h-full bg-apple-pink animate-bounce" />
                        <span className="w-0.5 h-2/3 bg-apple-pink animate-bounce delay-75" />
                        <span className="w-0.5 h-4/5 bg-apple-pink animate-bounce delay-150" />
                      </div>
                    ) : isSelected ? (
                      <Pause className="w-4 h-4 text-apple-pink fill-apple-pink" />
                    ) : (
                      <>
                        <span className="group-hover:hidden">{idx + 1}</span>
                        <Play className="w-3.5 h-3.5 text-white fill-white hidden group-hover:block ml-0.5" />
                      </>
                    )}
                  </button>
                </td>

                {/* Title & Artist */}
                <td className="py-2.5 px-4">
                  <div className="flex items-center gap-3">
                    <div className="w-9 h-9 rounded-md bg-[#28282c] border border-white/10 flex items-center justify-center text-apple-pink shrink-0 overflow-hidden shadow-sm relative">
                      <Music className="w-4 h-4 absolute inset-0 m-auto opacity-60" />
                      <img
                        src={`${getApiUrl()}/cover?path=${encodeURIComponent(file.rel_path)}`}
                        alt={file.title}
                        className="w-full h-full object-cover relative z-10"
                        onError={(e) => {
                          e.currentTarget.style.display = "none";
                        }}
                      />
                    </div>
                    <div className="min-w-0">
                      <p
                        className={`font-semibold text-xs truncate max-w-sm ${
                          isSelected ? "text-apple-pink" : "text-white"
                        }`}
                      >
                        {file.title}
                      </p>
                      <p className="text-[11px] text-apple-subtext truncate font-normal">
                        {file.artist}
                      </p>
                    </div>
                  </div>
                </td>

                {/* Album */}
                <td className="py-2.5 px-4 text-apple-subtext hidden md:table-cell truncate max-w-[180px] font-normal">
                  {file.album || t("Single")}
                </td>

                {/* Status Badge */}
                <td className="py-2.5 px-4 text-center">
                  {isSynced ? (
                    <span className="inline-block text-[10px] font-semibold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2.5 py-0.5 rounded">
                      {t("Synced")}
                    </span>
                  ) : isUnsynced ? (
                    <span className="inline-block text-[10px] font-semibold text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2.5 py-0.5 rounded">
                      {t("Plain Text")}
                    </span>
                  ) : (
                    <span className="inline-block text-[10px] font-semibold text-rose-400 bg-rose-500/10 border border-rose-500/20 px-2.5 py-0.5 rounded">
                      {t("Missing")}
                    </span>
                  )}
                </td>

                {/* Action */}
                <td className="py-2.5 px-4 text-right">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onSelectTrack(file);
                    }}
                    className="text-[11px] text-apple-subtext hover:text-white font-medium transition-colors"
                  >
                    {t("View Lyrics →")}
                  </button>
                </td>
              </tr>
            );
          })}

          {filteredFiles.length === 0 && (
            <tr>
              <td colSpan={5} className="text-center py-20 text-apple-subtext">
                <FileText className="w-8 h-8 mx-auto mb-2 opacity-30" />
                <p className="text-xs font-semibold text-zinc-400">{t("No tracks match filter")}</p>
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
};
