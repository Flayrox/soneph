"use client";

import React, { useState } from "react";
import { ArrowRight, Loader2, ListPlus, Link as LinkIcon, Code } from "lucide-react";

interface DownloadInputProps {
  onDownload: (url: string) => Promise<void>;
  onBatchDownload: (urls: string[]) => Promise<void>;
  isLoading: boolean;
}

export const DownloadInput: React.FC<DownloadInputProps> = ({
  onDownload,
  onBatchDownload,
  isLoading,
}) => {
  const [isBatchMode, setIsBatchMode] = useState<boolean>(false);
  const [singleUrl, setSingleUrl] = useState<string>("");
  const [batchUrls, setBatchUrls] = useState<string>("");

  const handleSingleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!singleUrl.trim() || isLoading) return;
    await onDownload(singleUrl.trim());
    setSingleUrl("");
  };

  const handleBatchSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!batchUrls.trim() || isLoading) return;

    const urls = batchUrls
      .split("\n")
      .map((u) => u.trim())
      .filter((u) => u.length > 0);

    if (urls.length > 0) {
      await onBatchDownload(urls);
      setBatchUrls("");
    }
  };

  return (
    <div className="w-full max-w-6xl mx-auto my-6 px-4">
      <div className="bg-dev-card border border-dev-border rounded-xl p-5 shadow-lg">
        {/* Toggle Mode */}
        <div className="flex items-center justify-between border-b border-dev-border pb-4 mb-4">
          <div className="flex items-center gap-2">
            <Code className="w-4 h-4 text-emerald-400" />
            <span className="text-xs font-mono font-bold text-white uppercase tracking-wider">
              Download Pipeline Target
            </span>
          </div>

          <div className="flex items-center gap-1 bg-zinc-900 border border-zinc-800 p-1 rounded font-mono text-[11px]">
            <button
              type="button"
              onClick={() => setIsBatchMode(false)}
              className={`px-3 py-1 rounded transition-colors ${
                !isBatchMode
                  ? "bg-zinc-800 text-white font-medium shadow-sm"
                  : "text-zinc-400 hover:text-white"
              }`}
            >
              Single Target
            </button>
            <button
              type="button"
              onClick={() => setIsBatchMode(true)}
              className={`px-3 py-1 rounded transition-colors flex items-center gap-1.5 ${
                isBatchMode
                  ? "bg-zinc-800 text-white font-medium shadow-sm"
                  : "text-zinc-400 hover:text-white"
              }`}
            >
              <ListPlus className="w-3 h-3 text-emerald-400" />
              <span>Batch Queue</span>
            </button>
          </div>
        </div>

        {/* Form Single URL */}
        {!isBatchMode ? (
          <form onSubmit={handleSingleSubmit} className="flex gap-2">
            <div className="relative flex-1">
              <LinkIcon className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input
                type="text"
                value={singleUrl}
                onChange={(e) => setSingleUrl(e.target.value)}
                placeholder="https://open..com/track/... or /album/ or /playlist/"
                className="w-full bg-dev-bg border border-dev-border focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 rounded-lg py-2.5 pl-10 pr-4 text-xs font-mono text-white placeholder-zinc-500 focus:outline-none transition-colors"
                required
              />
            </div>

            <button
              type="submit"
              disabled={isLoading || !singleUrl.trim()}
              className="px-5 py-2.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-mono text-xs font-semibold rounded-lg flex items-center gap-2 transition-all active:scale-[0.98]"
            >
              {isLoading ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <ArrowRight className="w-3.5 h-3.5" />
              )}
              <span>Execute Task</span>
            </button>
          </form>
        ) : (
          /* Form Batch URLs */
          <form onSubmit={handleBatchSubmit} className="space-y-3">
            <textarea
              rows={4}
              value={batchUrls}
              onChange={(e) => setBatchUrls(e.target.value)}
              placeholder={`https://open..com/track/1...\nhttps://open..com/playlist/2...\nhttps://open..com/album/3...`}
              className="w-full bg-dev-bg border border-dev-border focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500/20 rounded-lg p-3 text-xs font-mono text-white placeholder-zinc-600 focus:outline-none scrollbar-thin transition-colors"
            />
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-mono text-zinc-500">
                Paste line-separated URLs. All targets will execute concurrently across 4 workers.
              </span>
              <button
                type="submit"
                disabled={isLoading || !batchUrls.trim()}
                className="px-5 py-2.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-mono text-xs font-semibold rounded-lg flex items-center gap-2 transition-all active:scale-[0.98]"
              >
                {isLoading ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <ListPlus className="w-3.5 h-3.5" />
                )}
                <span>Enqueue Batch Jobs</span>
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
