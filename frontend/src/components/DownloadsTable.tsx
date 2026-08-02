"use client";

import React, { useState } from "react";
import { Search, Play, Pause, Trash2, Copy, Check, FileText, HardDrive, RefreshCw, Music2 } from "lucide-react";

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

interface DownloadsTableProps {
  files: DownloadedFile[];
  onDelete: (path: string) => Promise<void>;
  onRefresh: () => void;
  getStreamUrl: (relPath: string) => string;
}

export const DownloadsTable: React.FC<DownloadsTableProps> = ({
  files,
  onDelete,
  onRefresh,
  getStreamUrl,
}) => {
  const [searchTerm, setSearchTerm] = useState("");
  const [playingPath, setPlayingPath] = useState<string | null>(null);
  const [copiedPath, setCopiedPath] = useState<string | null>(null);
  const [audioElement, setAudioElement] = useState<HTMLAudioElement | null>(null);

  const formatSize = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  const filteredFiles = files.filter(
    (file) =>
      file.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
      file.artist.toLowerCase().includes(searchTerm.toLowerCase()) ||
      file.album.toLowerCase().includes(searchTerm.toLowerCase()) ||
      file.rel_path.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const handlePlayToggle = (relPath: string) => {
    if (playingPath === relPath) {
      if (audioElement) {
        audioElement.pause();
      }
      setPlayingPath(null);
    } else {
      if (audioElement) {
        audioElement.pause();
      }
      const streamUrl = getStreamUrl(relPath);
      const audio = new Audio(streamUrl);
      audio.play().catch((err) => console.error("Audio playback error:", err));
      audio.onended = () => setPlayingPath(null);
      setAudioElement(audio);
      setPlayingPath(relPath);
    }
  };

  const copyPathToClipboard = (path: string) => {
    navigator.clipboard.writeText(path);
    setCopiedPath(path);
    setTimeout(() => setCopiedPath(null), 2000);
  };

  return (
    <div className="w-full max-w-6xl mx-auto my-6 px-4">
      <div className="bg-dev-card border border-dev-border rounded-xl p-5 shadow-lg">
        {/* Header Controls */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 mb-5">
          <div>
            <div className="flex items-center gap-2">
              <HardDrive className="w-4 h-4 text-emerald-400" />
              <h3 className="text-xs font-mono font-bold text-white uppercase tracking-wider">
                Target Storage Index ({files.length} items)
              </h3>
            </div>
            <p className="text-[11px] font-mono text-zinc-500 mt-1">
              Location: <code className="text-zinc-400 bg-zinc-900 px-1.5 py-0.5 rounded border border-zinc-800">/app/downloads/</code>
            </p>
          </div>

          <div className="flex items-center gap-2 w-full sm:w-auto font-mono text-xs">
            <div className="relative flex-1 sm:w-64">
              <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Filter path, title, artist..."
                className="w-full bg-dev-bg border border-dev-border focus:border-emerald-500 rounded-lg py-1.5 pl-8 pr-3 text-xs font-mono text-white placeholder-zinc-500 focus:outline-none transition-colors"
              />
            </div>

            <button
              onClick={onRefresh}
              className="p-2 rounded bg-zinc-900 hover:bg-zinc-800 border border-zinc-800 text-zinc-400 hover:text-white transition-colors"
              title="Refresh Files"
            >
              <RefreshCw className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        {/* File Table */}
        {filteredFiles.length === 0 ? (
          <div className="text-center py-16 border border-dashed border-zinc-800 rounded-lg font-mono">
            <Music2 className="w-8 h-8 text-zinc-600 mx-auto mb-2 opacity-50" />
            <p className="text-xs text-zinc-400">
              {searchTerm ? "No files match the specified query." : "No audio files indexed in storage."}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-xs">
              <thead className="border-b border-zinc-800 text-zinc-500 text-[10px] uppercase tracking-wider bg-zinc-900/50">
                <tr>
                  <th className="py-2.5 px-3">Status</th>
                  <th className="py-2.5 px-3">Track Metadata</th>
                  <th className="py-2.5 px-3 hidden lg:table-cell">Album</th>
                  <th className="py-2.5 px-3">Lyrics (.lrc)</th>
                  <th className="py-2.5 px-3 hidden sm:table-cell">Size</th>
                  <th className="py-2.5 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800/60">
                {filteredFiles.map((file) => {
                  const isPlaying = playingPath === file.rel_path;
                  return (
                    <tr key={file.id} className="hover:bg-zinc-900/40 transition-colors group">
                      {/* Preview Button */}
                      <td className="py-3 px-3">
                        <button
                          onClick={() => handlePlayToggle(file.rel_path)}
                          className={`w-7 h-7 rounded border flex items-center justify-center transition-all ${
                            isPlaying
                              ? "bg-emerald-500 text-black border-emerald-400 shadow-sm shadow-emerald-500/20"
                              : "bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-white hover:border-zinc-700"
                          }`}
                          title={isPlaying ? "Pause Preview" : "Play Audio Preview"}
                        >
                          {isPlaying ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5 ml-0.5" />}
                        </button>
                      </td>

                      {/* Title & Artist */}
                      <td className="py-3 px-3">
                        <div>
                          <p className="text-white font-medium truncate max-w-xs">{file.title}</p>
                          <p className="text-[11px] text-zinc-500 truncate mt-0.5">{file.artist}</p>
                        </div>
                      </td>

                      {/* Album */}
                      <td className="py-3 px-3 hidden lg:table-cell text-zinc-400 truncate max-w-xs">
                        {file.album}
                      </td>

                      {/* Lyrics Badge */}
                      <td className="py-3 px-3">
                        {file.has_lyrics ? (
                          <span className="inline-flex items-center gap-1 text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-2 py-0.5 rounded">
                            <FileText className="w-3 h-3" /> LRC_SYNCED
                          </span>
                        ) : (
                          <span className="text-[10px] text-zinc-500 bg-zinc-900 border border-zinc-800 px-2 py-0.5 rounded">
                            NO_LRC
                          </span>
                        )}
                      </td>

                      {/* Size */}
                      <td className="py-3 px-3 hidden sm:table-cell text-zinc-400 text-[11px]">
                        {formatSize(file.size)}
                      </td>

                      {/* Actions */}
                      <td className="py-3 px-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            onClick={() => copyPathToClipboard(file.rel_path)}
                            className="p-1.5 text-zinc-400 hover:text-white hover:bg-zinc-800 rounded transition-colors"
                            title="Copy Relative Path"
                          >
                            {copiedPath === file.rel_path ? (
                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                            ) : (
                              <Copy className="w-3.5 h-3.5" />
                            )}
                          </button>
                          <button
                            onClick={() => onDelete(file.rel_path)}
                            className="p-1.5 text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10 rounded transition-colors"
                            title="Delete File"
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
        )}
      </div>
    </div>
  );
};
