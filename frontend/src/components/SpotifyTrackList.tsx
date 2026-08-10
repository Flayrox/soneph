import React from "react";
import { Play, Pause, Trash2, CheckCircle2, Music, Clock, Sparkles, FileText, AlertCircle } from "lucide-react";

export interface DownloadedFile {
  rel_path: string;
  file_name?: string;
  title: string;
  artist: string;
  album: string;
  duration?: number;
  size_bytes?: number;
  has_lyrics?: boolean;
  lyrics_type?: "synced" | "unsynced" | "none";
  lrc_path?: string;
  mod_time: string;
}

export interface DownloadTask {
  id: string;
  url: string;
  bitrate?: string;
  order?: string;
  status: "queued" | "downloading" | "completed" | "failed";
  progress: string;
  current_track?: string;
  total_tracks?: number;
  completed_count?: number;
  recent_tracks?: string[];
  logs: string[];
  created_at: string;
  error?: string;
}

interface TrackListProps {
  files: DownloadedFile[];
  activeTasks: DownloadTask[];
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onTrackPlay: (relPath: string) => void;
  onSelectTrack?: (track: DownloadedFile) => void;
  onDelete: (path: string) => void;
  getApiUrl?: () => string;
}

export const TrackList: React.FC<TrackListProps> = ({
  files,
  activeTasks,
  currentPlayingPath,
  isPlaying,
  onTrackPlay,
  onSelectTrack,
  onDelete,
  getApiUrl,
}) => {
  const formatDuration = (seconds?: number) => {
    if (!seconds || isNaN(seconds)) return "3:30";
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs < 10 ? "0" : ""}${secs}`;
  };

  const formatModTime = (isoString: string) => {
    if (!isoString) return "—";
    const date = new Date(isoString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / (1000 * 60));

    if (diffMins < 1) return "Just now";
    if (diffMins < 60) return `${diffMins}m ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  };

  const downloadingTasks = activeTasks.filter(
    (t) => t.status === "downloading" || t.status === "queued"
  );

  return (
    <div className="w-full text-zinc-200 select-none font-sans">
      <table className="w-full text-left border-collapse text-xs">
        {/* Table Header */}
        <thead>
          <tr className="border-b border-white/10 text-apple-subtext font-semibold uppercase text-[10px] tracking-wider">
            <th className="py-2.5 px-4 w-12 text-center">#</th>
            <th className="py-2.5 px-4">Title</th>
            <th className="py-2.5 px-4 hidden md:table-cell">Album</th>
            <th className="py-2.5 px-4 hidden sm:table-cell">Added</th>
            <th className="py-2.5 px-4 text-center w-28">Lyrics Sync</th>
            <th className="py-2.5 px-4 text-right w-16">Time</th>
            <th className="py-2.5 px-4 text-right w-12"></th>
          </tr>
        </thead>

        <tbody>
          {/* Active Downloading Song Rows */}
          {downloadingTasks.map((task, idx) => {
            const currentSong = task.current_track || task.url;
            const total = task.total_tracks || 0;
            const done = task.completed_count || 0;
            const percent = total > 0 ? Math.round((done / total) * 100) : 0;

            return (
              <tr
                key={`task_${task.id}_${idx}`}
                className="border-b border-apple-pink/20 bg-apple-pink/10 animate-pulse text-white font-medium"
              >
                {/* Apple Spinning Ring Animation */}
                <td className="py-3 px-4 text-center">
                  <div className="w-4 h-4 border-2 border-apple-pink/30 border-t-apple-pink rounded-full animate-spin mx-auto flex items-center justify-center">
                    <div className="w-1 h-1 bg-apple-pink rounded-sm" />
                  </div>
                </td>

                {/* Track Info */}
                <td className="py-3 px-4">
                  <div className="flex items-center gap-3">
                    <div className="w-9 h-9 rounded-md bg-apple-pink/20 border border-apple-pink/40 flex items-center justify-center text-apple-pink shrink-0">
                      <Music className="w-4 h-4 animate-bounce" />
                    </div>
                    <div className="min-w-0">
                      <p className="font-semibold text-white truncate max-w-sm flex items-center gap-2">
                        <span>{currentSong}</span>
                        {total > 0 && (
                          <span className="text-[10px] bg-apple-pink text-white font-bold px-2 py-0.5 rounded-full">
                            {done}/{total} ({percent}%)
                          </span>
                        )}
                      </p>
                      <p className="text-[11px] text-apple-pink font-medium truncate">
                        {task.status === "queued" ? "Queued in download engine..." : task.progress}
                      </p>
                    </div>
                  </div>
                </td>

                <td className="py-3 px-4 text-apple-subtext hidden md:table-cell truncate">
                  {task.order === "reverse" ? "Newest Added First" : "Import Queue"}
                </td>

                <td className="py-3 px-4 text-apple-subtext hidden sm:table-cell">
                  <div className="flex items-center gap-1 text-[11px] text-apple-pink">
                    <Clock className="w-3 h-3" />
                    <span>Syncing now</span>
                  </div>
                </td>

                <td className="py-3 px-4 text-center">
                  <span className="text-[10px] font-bold text-apple-pink bg-apple-pink/20 px-2 py-0.5 rounded-full border border-apple-pink/30">
                    Downloading
                  </span>
                </td>

                <td className="py-3 px-4 text-right text-apple-pink font-semibold">
                  {percent}%
                </td>

                <td className="py-3 px-4"></td>
              </tr>
            );
          })}

          {/* Downloaded Songs Rows */}
          {files.map((file, idx) => {
            const isSelected = currentPlayingPath === file.rel_path;
            const isThisPlaying = isSelected && isPlaying;

            const isSynced = file.lyrics_type === "synced";
            const isUnsynced = file.lyrics_type === "unsynced";

            return (
              <tr
                key={file.rel_path}
                className={`border-b border-white/5 group hover:bg-white/5 transition-colors cursor-pointer ${
                  isSelected ? "bg-apple-pink/15 text-white" : "text-zinc-300"
                }`}
                onClick={() => {
                  if (onSelectTrack) {
                    onSelectTrack(file);
                  } else {
                    onTrackPlay(file.rel_path);
                  }
                }}
              >
                {/* Track Number / Play Button */}
                <td className="py-2.5 px-4 text-center text-apple-subtext font-medium group-hover:text-white">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onTrackPlay(file.rel_path);
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

                {/* Track Title & Artist */}
                <td className="py-2.5 px-4">
                  <div className="flex items-center gap-3">
                    <div className="w-9 h-9 rounded-md bg-[#28282c] border border-white/10 flex items-center justify-center text-apple-pink shrink-0 overflow-hidden shadow-sm relative">
                      <Music className="w-4 h-4 absolute inset-0 m-auto opacity-60" />
                      {getApiUrl && (
                        <img
                          src={`${getApiUrl()}/cover?path=${encodeURIComponent(file.rel_path)}`}
                          alt={file.title}
                          className="w-full h-full object-cover relative z-10"
                          onError={(e) => {
                            e.currentTarget.style.display = "none";
                          }}
                        />
                      )}
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
                  {file.album || "Single"}
                </td>

                {/* Date Added */}
                <td className="py-2.5 px-4 text-apple-subtext hidden sm:table-cell text-[11px] font-normal">
                  {formatModTime(file.mod_time)}
                </td>

                {/* Lyrics Sync Status Badge */}
                <td className="py-2.5 px-4 text-center">
                  {isSynced ? (
                    <span
                      title="Time-Synced Karaoke LRC lyrics"
                      className="inline-block text-[10px] font-semibold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded"
                    >
                      Synced
                    </span>
                  ) : isUnsynced ? (
                    <span
                      title="Plain text unsynced lyrics"
                      className="inline-block text-[10px] font-semibold text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded"
                    >
                      Text
                    </span>
                  ) : (
                    <span
                      title="No lyrics downloaded yet"
                      className="inline-block text-[10px] font-semibold text-rose-400 bg-rose-500/10 border border-rose-500/20 px-2 py-0.5 rounded"
                    >
                      Missing
                    </span>
                  )}
                </td>

                {/* Duration */}
                <td className="py-2.5 px-4 text-right text-apple-subtext text-[11px] font-normal">
                  {formatDuration(file.duration)}
                </td>

                {/* Actions */}
                <td className="py-2.5 px-4 text-right">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onDelete(file.rel_path);
                    }}
                    className="p-1 rounded text-zinc-500 hover:text-rose-400 hover:bg-white/10 opacity-0 group-hover:opacity-100 transition-all"
                    title="Delete track"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            );
          })}

          {files.length === 0 && downloadingTasks.length === 0 && (
            <tr>
              <td colSpan={7} className="text-center py-20 text-apple-subtext">
                <Music className="w-10 h-10 mx-auto mb-3 opacity-30" />
                <p className="text-sm font-semibold text-zinc-400">No tracks imported yet</p>
                <p className="text-xs text-zinc-500 mt-1">
                  Paste a  playlist or track URL above to start syncing
                </p>
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
};
