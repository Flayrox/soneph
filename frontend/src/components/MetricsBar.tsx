"use client";

import React from "react";
import { Music, HardDrive, FileCheck, Layers } from "lucide-react";

interface MetricsBarProps {
  totalFiles: number;
  totalSizeBytes: number;
  lyricsCount: number;
  activeTasksCount: number;
}

export const MetricsBar: React.FC<MetricsBarProps> = ({
  totalFiles,
  totalSizeBytes,
  lyricsCount,
  activeTasksCount,
}) => {
  const formatSize = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  const lyricsPercentage = totalFiles > 0 ? Math.round((lyricsCount / totalFiles) * 100) : 0;

  return (
    <div className="w-full max-w-6xl mx-auto mt-6 px-4">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div className="bg-dev-card border border-dev-border rounded-lg p-4">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-mono text-zinc-400 uppercase tracking-wider">Synced Tracks</span>
            <Music className="w-4 h-4 text-zinc-500" />
          </div>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-xl font-mono font-bold text-white">{totalFiles}</span>
            <span className="text-xs font-mono text-emerald-400">320kbps HQ</span>
          </div>
        </div>

        <div className="bg-dev-card border border-dev-border rounded-lg p-4">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-mono text-zinc-400 uppercase tracking-wider">Storage Volume</span>
            <HardDrive className="w-4 h-4 text-zinc-500" />
          </div>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-xl font-mono font-bold text-white">{formatSize(totalSizeBytes)}</span>
            <span className="text-xs font-mono text-zinc-400">/app/downloads</span>
          </div>
        </div>

        <div className="bg-dev-card border border-dev-border rounded-lg p-4">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-mono text-zinc-400 uppercase tracking-wider">Lyrics (.LRC)</span>
            <FileCheck className="w-4 h-4 text-zinc-500" />
          </div>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-xl font-mono font-bold text-white">{lyricsCount}</span>
            <span className="text-xs font-mono text-emerald-400">{lyricsPercentage}% coverage</span>
          </div>
        </div>

        <div className="bg-dev-card border border-dev-border rounded-lg p-4">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-mono text-zinc-400 uppercase tracking-wider">Active Workers</span>
            <Layers className="w-4 h-4 text-zinc-500" />
          </div>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-xl font-mono font-bold text-white">{activeTasksCount}</span>
            <span className="text-xs font-mono text-indigo-400">4 threads active</span>
          </div>
        </div>
      </div>
    </div>
  );
};
