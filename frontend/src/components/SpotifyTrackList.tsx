"use client";

import React, { useState } from "react";
import { Play, Pause, Cloud, Star, Trash2, Copy, Check, Disc, Loader2 } from "lucide-react";

export interface DownloadedFile {
  id: string;
  file_name: string;
  title: string;
  artist: string;
  album: string;
  path: string;
  rel_path: string;
  size: number;
  has_lyrics: boolean;
  mod_time: string;
}

export interface DownloadTask {
  id: string;
  url: string;
  bitrate?: string;
  status: "queued" | "downloading" | "completed" | "failed";
  progress: string;
  logs: string[];
  created_at: string;
  error?: string;
}

interface TrackListProps {
  files: DownloadedFile[];
  activeTasks?: DownloadTask[];
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onTrackPlay: (relPath: string) => void;
  onDelete: (relPath: string) => Promise<void>;
}

export const TrackList: React.FC<TrackListProps> = ({
  files,
  activeTasks = [],
  currentPlayingPath,
  isPlaying,
  onTrackPlay,
  onDelete,
}) => {
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  const [copiedPath, setCopiedPath] = useState<string | null>(null);
  const [favorites, setFavorites] = useState<Record<string, boolean>>({});

  const formatDuration = () => {
    return "3:45";
  };

  const toggleFavorite = (id: string) => {
    setFavorites((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const copyPath = (path: string) => {
    navigator.clipboard.writeText(path);
    setCopiedPath(path);
    setTimeout(() => setCopiedPath(null), 2000);
  };

  const parseTaskInfo = (progressStr: string) => {
    if (!progressStr) return { title: "Importing track...", percent: "" };
    const match = progressStr.match(/(?:Downloading|Downloaded|Processing|Skipping)\s+["']?([^"'\n:]+)["']?:?\s*(\d+%)?/i);
    if (match) {
      return { title: match[1].trim(), percent: match[2] || "" };
    }
    return { title: progressStr, percent: "" };
  };

  const activeDownloadingTasks = activeTasks.filter(
    (t) => t.status === "downloading" || t.status === "queued"
  );

  if (files.length === 0 && activeDownloadingTasks.length === 0) {
    return (
      <div className="px-8 py-20 text-center text-apple-subtext text-sm select-none">
        <Disc className="w-12 h-12 mx-auto mb-3 opacity-30 text-apple-subtext" />
        <p className="font-semibold text-white">No songs in your library</p>
        <p className="text-xs text-apple-subtext mt-1">
          Paste a  link in the top bar to import and sync your first track.
        </p>
      </div>
    );
  }

  return (
    <div className="px-6 py-4 select-none">
      <table className="w-full text-left border-collapse text-xs sm:text-sm">
        {/* Table Header matching l'app Musique macOS */}
        <thead>
          <tr className="text-apple-subtext border-b border-white/10 text-xs font-semibold">
            <th className="py-2.5 px-3 w-8">#</th>
            <th className="py-2.5 px-3 font-semibold">Title</th>
            <th className="py-2.5 px-3 w-12 text-center">
              <span title="Cloud & Download Status"><Cloud className="w-4 h-4 inline text-apple-subtext" /></span>
            </th>
            <th className="py-2.5 px-3 font-semibold">Time</th>
            <th className="py-2.5 px-3 font-semibold">Artist</th>
            <th className="py-2.5 px-3 font-semibold hidden md:table-cell">Album</th>
            <th className="py-2.5 px-3 font-semibold hidden lg:table-cell">Genre</th>
            <th className="py-2.5 px-3 w-10 text-center">
              <Star className="w-3.5 h-3.5 inline text-apple-subtext" />
            </th>
            <th className="py-2.5 px-3 text-right pr-4">Actions</th>
          </tr>
        </thead>

        {/* Table Body */}
        <tbody className="divide-y divide-white/5">
          {/* Active Downloading & Queued Tasks with Apple Downloading Ring */}
          {activeDownloadingTasks.map((task) => {
            const { title, percent } = parseTaskInfo(task.progress);
            const isDownloading = task.status === "downloading";

            return (
              <tr
                key={task.id}
                className="bg-apple-pink/5 border-b border-apple-pink/20 animate-pulse-subtle"
              >
                {/* # Column: Apple Download Progress Ring */}
                <td className="py-3 px-3 text-center">
                  <div className="relative w-5 h-5 mx-auto flex items-center justify-center">
                    <div className="w-5 h-5 rounded-full border-2 border-apple-pink/30 border-t-apple-pink animate-spin" />
                    <div className="w-1.5 h-1.5 rounded-sm bg-apple-pink absolute" />
                  </div>
                </td>

                {/* Title + Thumbnail + Downloading Badge */}
                <td className="py-3 px-3">
                  <div className="flex items-center gap-3">
                    <div className="w-9 h-9 rounded bg-apple-pink/10 border border-apple-pink/30 flex items-center justify-center text-apple-pink shrink-0 overflow-hidden shadow-inner">
                      <Loader2 className="w-5 h-5 animate-spin" />
                    </div>

                    <div className="overflow-hidden">
                      <p className="font-semibold text-white truncate max-w-xs flex items-center gap-2">
                        <span>{title}</span>
                      </p>
                      <div className="flex items-center gap-2 mt-0.5">
                        <span className="text-[10px] text-apple-pink font-bold bg-apple-pink/20 px-2 py-0.5 rounded-full border border-apple-pink/30 animate-pulse">
                          {isDownloading ? `Downloading ${percent}` : "Queued..."}
                        </span>
                        {task.bitrate && (
                          <span className="text-[10px] text-apple-subtext font-mono">
                            {task.bitrate}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </td>

                {/* Cloud Downloading Status Icon */}
                <td className="py-3 px-3 text-center">
                  <span title="Downloading to Cloud Library">
                    <div className="w-4 h-4 rounded-full border-2 border-apple-pink border-t-transparent animate-spin inline-block" />
                  </span>
                </td>

                {/* Time */}
                <td className="py-3 px-3 text-apple-pink font-mono text-xs whitespace-nowrap">
                  {percent || "In progress"}
                </td>

                {/* Artist */}
                <td className="py-3 px-3 text-apple-subtext text-xs italic truncate max-w-xs">
                  Downloading via 8 Threads...
                </td>

                {/* Album */}
                <td className="py-3 px-3 hidden md:table-cell text-apple-subtext text-xs italic truncate max-w-xs">
                  soneph Engine
                </td>

                {/* Genre */}
                <td className="py-3 px-3 hidden lg:table-cell text-apple-subtext text-xs">
                  Auto-Sync
                </td>

                {/* Star */}
                <td className="py-3 px-3 text-center">
                  <Star className="w-3.5 h-3.5 inline text-white/20" />
                </td>

                {/* Actions */}
                <td className="py-3 px-3 text-right pr-4 text-xs text-apple-pink font-semibold">
                  Syncing...
                </td>
              </tr>
            );
          })}

          {/* Already Downloaded Songs */}
          {files.map((file, idx) => {
            const isThisPlaying = currentPlayingPath === file.rel_path && isPlaying;
            const isHovered = hoveredIndex === idx;
            const isFav = favorites[file.id];

            return (
              <tr
                key={file.id || idx}
                onMouseEnter={() => setHoveredIndex(idx)}
                onMouseLeave={() => setHoveredIndex(null)}
                onClick={() => onTrackPlay(file.rel_path)}
                className={`group hover:bg-white/5 transition-colors cursor-pointer ${
                  isThisPlaying ? "bg-white/10" : ""
                }`}
              >
                {/* Play / Index Button */}
                <td className="py-2.5 px-3 text-center">
                  {isHovered || isThisPlaying ? (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onTrackPlay(file.rel_path);
                      }}
                      className="text-white hover:scale-110 transition-transform"
                    >
                      {isThisPlaying ? (
                        <Pause className="w-3.5 h-3.5 fill-apple-pink text-apple-pink" />
                      ) : (
                        <Play className="w-3.5 h-3.5 fill-white text-white ml-0.5" />
                      )}
                    </button>
                  ) : (
                    <span className="text-xs text-apple-subtext">{activeDownloadingTasks.length + idx + 1}</span>
                  )}
                </td>

                {/* Title + Thumbnail */}
                <td className="py-2.5 px-3">
                  <div className="flex items-center gap-3">
                    <div className="w-9 h-9 rounded bg-[#2a2a2d] flex items-center justify-center text-apple-subtext shrink-0 overflow-hidden shadow-sm">
                      <Disc className="w-5 h-5 text-apple-subtext" />
                    </div>

                    <div className="overflow-hidden">
                      <p
                        className={`font-semibold truncate max-w-xs ${
                          isThisPlaying ? "text-apple-pink" : "text-white"
                        }`}
                      >
                        {file.title}
                      </p>
                      {file.has_lyrics && (
                        <span className="text-[10px] text-apple-pink font-semibold bg-apple-pink/10 px-1.5 py-0.2 rounded border border-apple-pink/20 inline-block mt-0.5">
                          Synced Lyrics
                        </span>
                      )}
                    </div>
                  </div>
                </td>

                {/* iCloud Synced Icon */}
                <td className="py-2.5 px-3 text-center">
                  <span title="Synced with iCloud">
                    <Cloud className="w-4 h-4 text-emerald-400 inline" />
                  </span>
                </td>

                {/* Time */}
                <td className="py-2.5 px-3 text-apple-subtext text-xs whitespace-nowrap">
                  {formatDuration()}
                </td>

                {/* Artist */}
                <td className="py-2.5 px-3 text-white text-xs truncate max-w-xs">
                  {file.artist}
                </td>

                {/* Album */}
                <td className="py-2.5 px-3 hidden md:table-cell text-apple-subtext text-xs truncate max-w-xs">
                  {file.album}
                </td>

                {/* Genre */}
                <td className="py-2.5 px-3 hidden lg:table-cell text-apple-subtext text-xs">
                  Hip-Hop/Rap
                </td>

                {/* Favorite Star */}
                <td className="py-2.5 px-3 text-center">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleFavorite(file.id);
                    }}
                    className="text-apple-subtext hover:text-apple-pink transition-colors"
                  >
                    <Star
                      className={`w-3.5 h-3.5 inline ${
                        isFav ? "fill-apple-pink text-apple-pink" : "text-apple-subtext"
                      }`}
                    />
                  </button>
                </td>

                {/* Actions */}
                <td className="py-2.5 px-3 text-right pr-4">
                  <div className="flex items-center justify-end gap-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copyPath(file.rel_path);
                      }}
                      className="text-apple-subtext hover:text-white opacity-0 group-hover:opacity-100 transition-opacity p-1"
                      title="Copy Relative Path"
                    >
                      {copiedPath === file.rel_path ? (
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                      ) : (
                        <Copy className="w-3.5 h-3.5" />
                      )}
                    </button>

                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        if (confirm(`Remove "${file.title}" from library?`)) {
                          onDelete(file.rel_path);
                        }
                      }}
                      className="text-apple-subtext hover:text-rose-400 opacity-0 group-hover:opacity-100 transition-opacity p-1"
                      title="Delete Track"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};
