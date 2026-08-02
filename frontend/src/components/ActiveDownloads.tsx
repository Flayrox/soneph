"use client";

import React, { useState } from "react";
import { Terminal, CheckCircle2, AlertTriangle, Loader2, ChevronDown, ChevronUp, Copy, Check } from "lucide-react";

export interface DownloadTask {
  id: string;
  url: string;
  status: "queued" | "downloading" | "completed" | "failed";
  progress: string;
  logs: string[];
  created_at: string;
  error?: string;
}

interface ActiveDownloadsProps {
  tasks: DownloadTask[];
}

export const ActiveDownloads: React.FC<ActiveDownloadsProps> = ({ tasks }) => {
  const [expandedTask, setExpandedTask] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  if (tasks.length === 0) return null;

  const toggleExpand = (id: string) => {
    setExpandedTask(expandedTask === id ? null : id);
  };

  const copyLogs = (task: DownloadTask) => {
    const text = task.logs?.join("\n") || "";
    navigator.clipboard.writeText(text);
    setCopiedId(task.id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  return (
    <div className="w-full max-w-6xl mx-auto my-6 px-4">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2 text-xs font-mono font-bold text-zinc-400 uppercase tracking-wider">
          <Loader2 className="w-3.5 h-3.5 text-emerald-400 animate-spin" />
          <span>Active Pipeline Execution Queue ({tasks.length})</span>
        </div>
      </div>

      <div className="space-y-2.5">
        {tasks.map((task) => {
          const isExpanded = expandedTask === task.id;
          return (
            <div
              key={task.id}
              className="bg-dev-card border border-dev-border rounded-lg p-4 font-mono transition-all"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="flex items-center gap-3 overflow-hidden">
                  {task.status === "downloading" && (
                    <Loader2 className="w-4 h-4 text-emerald-400 animate-spin shrink-0" />
                  )}
                  {task.status === "queued" && (
                    <span className="w-2 h-2 rounded-full bg-amber-400 animate-ping shrink-0" />
                  )}
                  {task.status === "completed" && (
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                  )}
                  {task.status === "failed" && (
                    <AlertTriangle className="w-4 h-4 text-rose-500 shrink-0" />
                  )}

                  <div className="overflow-hidden text-xs">
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] text-zinc-500 bg-zinc-900 px-1.5 py-0.5 rounded border border-zinc-800">
                        {task.id.substring(0, 16)}...
                      </span>
                      <span className="text-zinc-300 font-medium truncate max-w-md">
                        {task.url}
                      </span>
                    </div>
                    <p className="text-[11px] text-zinc-500 truncate mt-1">
                      {task.progress || "Queued in Go worker..."}
                    </p>
                  </div>
                </div>

                <div className="flex items-center justify-between sm:justify-end gap-2 text-xs">
                  <span
                    className={`px-2.5 py-0.5 rounded text-[11px] font-mono border ${
                      task.status === "completed"
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                        : task.status === "failed"
                        ? "bg-rose-500/10 text-rose-400 border-rose-500/20"
                        : task.status === "downloading"
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/30 animate-pulse"
                        : "bg-amber-500/10 text-amber-400 border-amber-500/20"
                    }`}
                  >
                    STATUS_{task.status.toUpperCase()}
                  </span>

                  <button
                    onClick={() => toggleExpand(task.id)}
                    className="flex items-center gap-1 text-[11px] text-zinc-400 hover:text-white px-2.5 py-1 rounded bg-zinc-900 border border-zinc-800 hover:border-zinc-700 transition-colors"
                  >
                    <Terminal className="w-3.5 h-3.5 text-zinc-500" />
                    <span>Logs ({task.logs?.length || 0})</span>
                    {isExpanded ? (
                      <ChevronUp className="w-3 h-3 text-zinc-500" />
                    ) : (
                      <ChevronDown className="w-3 h-3 text-zinc-500" />
                    )}
                  </button>
                </div>
              </div>

              {/* Developer Logs Drawer */}
              {isExpanded && (
                <div className="mt-3 border border-zinc-800 bg-black/90 rounded-md p-3 font-mono text-[11px] text-zinc-300 space-y-1">
                  <div className="flex items-center justify-between border-b border-zinc-800 pb-2 mb-2 text-zinc-500 text-[10px]">
                    <span>STDERR / STDOUT LOG STREAM</span>
                    <button
                      onClick={() => copyLogs(task)}
                      className="flex items-center gap-1 hover:text-white transition-colors"
                    >
                      {copiedId === task.id ? (
                        <Check className="w-3 h-3 text-emerald-400" />
                      ) : (
                        <Copy className="w-3 h-3" />
                      )}
                      <span>{copiedId === task.id ? "COPIED" : "COPY LOGS"}</span>
                    </button>
                  </div>
                  <div className="max-h-48 overflow-y-auto space-y-1 scrollbar-thin">
                    {task.logs && task.logs.length > 0 ? (
                      task.logs.map((log, idx) => (
                        <div key={idx} className="flex gap-2">
                          <span className="text-zinc-600 select-none w-6 text-right">{idx + 1}</span>
                          <span className="text-emerald-400/90">{log}</span>
                        </div>
                      ))
                    ) : (
                      <div className="text-zinc-600">No logs captured yet...</div>
                    )}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
