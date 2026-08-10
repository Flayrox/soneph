"use client";

import React from "react";
import { Terminal, RefreshCw, Server, Cpu, HardDrive } from "lucide-react";

interface HeaderProps {
  isConnected: boolean;
  onRefresh: () => void;
  fileCount: number;
  totalSize: number;
}

export const Header: React.FC<HeaderProps> = ({
  isConnected,
  onRefresh,
  fileCount,
  totalSize,
}) => {
  const formatSize = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  return (
    <header className="w-full bg-dev-bg border-b border-dev-border sticky top-0 z-40 px-6 py-3.5 flex items-center justify-between">
      <div className="flex items-center gap-6">
        <div className="flex items-center gap-3">
          <div className="w-7 h-7 rounded bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center">
            <Terminal className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="flex items-center gap-2">
            <span className="font-mono font-bold text-sm tracking-tight text-white">
              son<span className="text-rose-500 font-extrabold">ephe</span>
            </span>
            <span className="text-[10px] font-mono text-zinc-400 bg-zinc-800/80 px-2 py-0.5 rounded border border-zinc-700/60">
              v1.2.0-dev
            </span>
          </div>
        </div>

        {/* Status Metrics */}
        <div className="hidden md:flex items-center gap-4 text-xs font-mono text-zinc-400 border-l border-zinc-800 pl-6">
          <div className="flex items-center gap-1.5">
            <Server className="w-3.5 h-3.5 text-zinc-500" />
            <span>Go/spotdl engine</span>
          </div>
          <span className="text-zinc-700">•</span>
          <div className="flex items-center gap-1.5">
            <Cpu className="w-3.5 h-3.5 text-zinc-500" />
            <span>4 parallel threads</span>
          </div>
          <span className="text-zinc-700">•</span>
          <div className="flex items-center gap-1.5">
            <HardDrive className="w-3.5 h-3.5 text-zinc-500" />
            <span>{fileCount} tracks ({formatSize(totalSize)})</span>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-3 font-mono text-xs">
        <div className="flex items-center gap-2 px-2.5 py-1 rounded bg-zinc-900 border border-zinc-800">
          <span className="relative flex h-2 w-2">
            {isConnected ? (
              <>
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </>
            ) : (
              <span className="relative inline-flex rounded-full h-2 w-2 bg-rose-500"></span>
            )}
          </span>
          <span className={isConnected ? "text-emerald-400" : "text-rose-400"}>
            {isConnected ? "WS CONNECTED" : "DISCONNECTED"}
          </span>
        </div>

        <button
          onClick={onRefresh}
          className="p-1.5 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-400 hover:text-white border border-zinc-800 transition-colors"
          title="Refresh Data"
        >
          <RefreshCw className="w-3.5 h-3.5" />
        </button>
      </div>
    </header>
  );
};
