import React from "react";
import { X, Loader2, CheckCircle2, AlertCircle, Clock, Zap, Music, Disc } from "lucide-react";
import { DownloadTask } from "./TrackList";

interface QueueDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  tasks: DownloadTask[];
}

export const QueueDrawer: React.FC<QueueDrawerProps> = ({
  isOpen,
  onClose,
  tasks,
}) => {
  if (!isOpen) return null;

  const activeDownloading = tasks.filter((t) => t.status === "downloading");
  const queued = tasks.filter((t) => t.status === "queued");
  const completed = tasks.filter((t) => t.status === "completed");
  const failed = tasks.filter((t) => t.status === "failed");

  // Collect all recent tracks downloaded across tasks
  const allRecentTracks = tasks.flatMap((t) => t.recent_tracks || []);

  return (
    <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex justify-end animate-fade-in select-none font-sans">
      <div className="w-full max-w-md bg-[#1c1c1f] border-l border-white/10 h-full flex flex-col justify-between p-6 shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between pb-4 border-b border-white/10">
          <div>
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Loader2 className="w-4 h-4 text-apple-pink animate-spin" />
              <span>Playlist Download Queue</span>
            </h2>
            <p className="text-xs text-apple-subtext mt-0.5 font-medium">
              {activeDownloading.length} active worker(s) • {queued.length} queued • {allRecentTracks.length} tracks done
            </p>
          </div>

          <button
            onClick={onClose}
            className="w-8 h-8 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto my-4 space-y-6 pr-1 scrollbar-none">
          {/* Active Downloading Tasks */}
          {activeDownloading.length > 0 && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-apple-pink uppercase tracking-wider flex items-center gap-1.5">
                <Zap className="w-3.5 h-3.5" />
                <span>Downloading Now</span>
              </h3>

              <div className="space-y-3">
                {activeDownloading.map((task) => {
                  const total = task.total_tracks || 0;
                  const done = task.completed_count || 0;
                  const percent = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
                  const currentSong = task.current_track || "Processing playlist...";

                  return (
                    <div
                      key={task.id}
                      className="bg-[#26262a] border border-apple-pink/30 rounded-xl p-3.5 space-y-3 shadow-lg"
                    >
                      {/* Current Song Title Badge */}
                      <div className="flex items-start gap-3">
                        <div className="w-9 h-9 rounded-lg bg-apple-pink/20 border border-apple-pink/40 flex items-center justify-center text-apple-pink shrink-0 mt-0.5">
                          <Music className="w-4.5 h-4.5 animate-bounce" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <span className="text-[10px] uppercase font-bold tracking-wider text-apple-pink">
                            Now Downloading
                          </span>
                          <p className="text-xs font-bold text-white truncate leading-snug">
                            {currentSong}
                          </p>
                        </div>
                      </div>

                      {/* Global Playlist Progress Bar */}
                      <div className="space-y-1.5 pt-1 border-t border-white/5">
                        <div className="flex justify-between text-xs text-apple-subtext font-medium">
                          <span>
                            {total > 0 ? `${Math.min(done, total)} of ${total} songs` : task.progress}
                          </span>
                          <span className="font-bold text-white">{percent}%</span>
                        </div>
                        <div className="w-full h-2 bg-black/40 rounded-full overflow-hidden p-0.5 border border-white/10">
                          <div
                            className="h-full bg-apple-pink rounded-full transition-all duration-300 shadow-sm"
                            style={{ width: `${Math.max(percent, 5)}%` }}
                          />
                        </div>
                      </div>

                      {/* Log snippet */}
                      <p className="text-[11px] text-zinc-400 truncate pt-0.5">
                        {task.progress}
                      </p>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Recently Completed Song-by-Song List */}
          {allRecentTracks.length > 0 && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-emerald-400 uppercase tracking-wider flex items-center gap-1.5">
                <CheckCircle2 className="w-3.5 h-3.5" />
                <span>Downloaded Songs ({allRecentTracks.length})</span>
              </h3>

              <div className="space-y-2">
                {allRecentTracks.slice(0, 20).map((song, idx) => (
                  <div
                    key={`recent_${idx}`}
                    className="bg-[#242428]/60 border border-emerald-500/20 rounded-xl p-2.5 flex items-center justify-between text-xs text-zinc-200"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <Disc className="w-4 h-4 text-emerald-400 shrink-0" />
                      <span className="truncate font-semibold text-white">{song}</span>
                    </div>
                    <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full font-medium shrink-0">
                      Downloaded
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Queued Playlist Tasks */}
          {queued.length > 0 && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-apple-subtext uppercase tracking-wider flex items-center gap-1.5">
                <Clock className="w-3.5 h-3.5" />
                <span>Up Next in Queue ({queued.length})</span>
              </h3>

              <div className="space-y-2">
                {queued.map((task) => (
                  <div
                    key={task.id}
                    className="bg-[#242428]/40 border border-white/5 rounded-xl p-3 flex items-center justify-between text-xs text-apple-subtext"
                  >
                    <span className="truncate max-w-[260px] text-zinc-300 font-medium">
                      {task.url}
                    </span>
                    <span className="text-[10px] bg-white/10 text-zinc-300 px-2 py-0.5 rounded-full font-medium">
                      Queued
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Failed Items */}
          {failed.length > 0 && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-rose-400 uppercase tracking-wider flex items-center gap-1.5">
                <AlertCircle className="w-3.5 h-3.5" />
                <span>Failed Imports ({failed.length})</span>
              </h3>

              <div className="space-y-2">
                {failed.map((task) => (
                  <div
                    key={task.id}
                    className="bg-rose-500/10 border border-rose-500/30 rounded-xl p-3 text-xs text-rose-300 space-y-1"
                  >
                    <p className="font-semibold truncate">{task.url}</p>
                    <p className="text-[11px] text-rose-400/80">{task.error || "Execution error"}</p>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="pt-4 border-t border-white/10 text-center text-xs text-apple-subtext font-medium">
          <span>All tracks auto-sync into macOS l'app Musique & Finder</span>
        </div>
      </div>
    </div>
  );
};
