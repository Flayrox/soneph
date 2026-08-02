"use client";

import React from "react";
import { X, Loader2, CheckCircle2, AlertCircle, Clock, Zap, ArrowDownUp } from "lucide-react";
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

  const parseLogInfo = (task: DownloadTask) => {
    const lastLog = task.logs && task.logs.length > 0 ? task.logs[task.logs.length - 1] : task.progress;
    return lastLog || task.progress || "Processing...";
  };

  return (
    <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex justify-end animate-fade-in select-none">
      <div className="w-full max-w-lg bg-[#1a1a1d] border-l border-white/10 h-full flex flex-col justify-between p-6 shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between pb-4 border-b border-white/10">
          <div>
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Loader2 className="w-4 h-4 text-apple-pink animate-spin" />
              <span>Download & Sync Manager</span>
            </h2>
            <p className="text-xs text-apple-subtext mt-0.5">
              {activeDownloading.length} active • {queued.length} queued • {completed.length} completed
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
          {/* Active Downloading Section */}
          {activeDownloading.length > 0 && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-apple-pink uppercase tracking-wider flex items-center gap-1.5">
                <Zap className="w-3.5 h-3.5" />
                <span>Currently Downloading (8 Threads)</span>
              </h3>

              <div className="space-y-2">
                {activeDownloading.map((task) => (
                  <div
                    key={task.id}
                    className="bg-[#242428] border border-apple-pink/30 rounded-xl p-3 space-y-2"
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-bold text-white truncate max-w-[280px]">
                        {task.url}
                      </span>
                      <span className="text-[10px] bg-apple-pink/20 text-apple-pink px-2 py-0.5 rounded-full font-mono font-semibold">
                        {task.bitrate || "320k"}
                      </span>
                    </div>

                    {/* Progress Bar */}
                    <div className="space-y-1">
                      <div className="flex justify-between text-[11px] text-apple-subtext font-mono">
                        <span className="truncate max-w-xs">{parseLogInfo(task)}</span>
                      </div>
                      <div className="w-full h-1.5 bg-white/10 rounded-full overflow-hidden">
                        <div className="h-full bg-apple-pink rounded-full animate-pulse w-3/4" />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Queued Items Section */}
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
                    className="bg-[#242428]/60 border border-white/5 rounded-xl p-3 flex items-center justify-between text-xs text-apple-subtext"
                  >
                    <span className="truncate max-w-[280px] text-zinc-300 font-medium">{task.url}</span>
                    <span className="text-[10px] bg-white/10 px-2 py-0.5 rounded-full">Queued</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Completed Recently Section */}
          {completed.length > 0 && (
            <div className="space-y-3">
              <h3 className="text-xs font-semibold text-emerald-400 uppercase tracking-wider flex items-center gap-1.5">
                <CheckCircle2 className="w-3.5 h-3.5" />
                <span>Completed Recently ({completed.length})</span>
              </h3>

              <div className="space-y-2">
                {completed.slice(0, 10).map((task) => (
                  <div
                    key={task.id}
                    className="bg-[#242428]/40 border border-emerald-500/20 rounded-xl p-3 flex items-center justify-between text-xs text-zinc-300"
                  >
                    <span className="truncate max-w-[280px] font-medium">{task.url}</span>
                    <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                      Synced
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Failed Items Section */}
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
        <div className="pt-4 border-t border-white/10 text-center text-xs text-apple-subtext">
          <span>All completed tracks are auto-synced into macOS l'app Musique</span>
        </div>
      </div>
    </div>
  );
};
