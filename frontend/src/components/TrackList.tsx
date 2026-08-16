import React, { useEffect, useMemo, useState } from "react";
import { Play, Pause, Trash2, Music, Clock, Plus, ListPlus, Heart, X, ListMusic } from "lucide-react";
import { Glass } from "./Glass";
import { TrackContextMenu, useTrackCtxMenu } from "./TrackContextMenu";
import { cleanTitle } from "@/format";
import type { DownloadedFile, DownloadTask, PlaylistSummary } from "@/types";
import { useI18n } from "@/i18n";

// Ré-export pour compatibilité avec les imports existants
export type { DownloadedFile, DownloadTask };

interface TrackListProps {
  files: DownloadedFile[];
  activeTasks: DownloadTask[];
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onTrackPlay: (relPath: string) => void;
  /** Play a specific list (used by the multi-selection action bar). */
  onPlayList?: (paths: string[], index: number) => void;
  /** Insert track(s) right after the current one. */
  onPlayNext?: (paths: string[]) => void;
  onSelectTrack?: (track: DownloadedFile) => void;
  /** Enable drag-and-drop reordering (used by playlists). */
  onReorder?: (path: string, toIndex: number) => void;
  onDelete: (path: string) => void;
  getApiUrl?: () => string;
  playlists?: PlaylistSummary[];
  onAddToPlaylist?: (playlistId: string, path: string) => void;
  onCreatePlaylist?: (name: string) => void;
  likes?: Set<string>;
  onToggleLike?: (path: string) => void;
}

export const TrackList: React.FC<TrackListProps> = ({
  files,
  activeTasks,
  currentPlayingPath,
  isPlaying,
  onTrackPlay,
  onPlayList,
  onPlayNext,
  onSelectTrack,
  onReorder,
  onDelete,
  getApiUrl,
  playlists = [],
  onAddToPlaylist,
  onCreatePlaylist,
  likes,
  onToggleLike,
}) => {
  const { t } = useI18n();
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [newPlaylistName, setNewPlaylistName] = useState("");
  // Shared right-click context menu (works everywhere in the app).
  const trackCtx = useTrackCtxMenu();

  // ── Track multi-selection (desktop feel) ─────────────────────────────
  // Plain click selects a single row (and keeps the legacy action);
  // ⌘/Ctrl+click toggles, ⇧+click extends the range from the anchor,
  // ⌘/Ctrl+A selects everything, Delete removes the selection, Escape clears.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [anchorPath, setAnchorPath] = useState<string | null>(null);

  // Drag-and-drop reordering (enabled when `onReorder` is provided).
  const [dragPath, setDragPath] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<number | null>(null);
  const SELECTION_KEY = "__selection__";

  const selectSingle = (path: string) => {
    setSelected(new Set([path]));
    setAnchorPath(path);
  };

  const toggleSelect = (path: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
    setAnchorPath(path);
  };

  const selectRange = (path: string) => {
    setSelected((prev) => {
      const anchor = anchorPath ?? path;
      const a = files.findIndex((f) => f.rel_path === anchor);
      const b = files.findIndex((f) => f.rel_path === path);
      if (a < 0 || b < 0) return new Set([path]);
      const [lo, hi] = a <= b ? [a, b] : [b, a];
      const next = new Set(prev);
      for (let i = lo; i <= hi; i++) next.add(files[i].rel_path);
      return next;
    });
    setAnchorPath(path);
  };

  const clearSelection = () => {
    setSelected(new Set());
    setAnchorPath(null);
  };

  const selectedFiles = useMemo(
    () => files.filter((f) => selected.has(f.rel_path)),
    [files, selected]
  );

  const handleRowClick = (e: React.MouseEvent, file: DownloadedFile) => {
    if (e.shiftKey) {
      e.preventDefault();
      selectRange(file.rel_path);
      return;
    }
    if (e.metaKey || e.ctrlKey) {
      e.preventDefault();
      toggleSelect(file.rel_path);
      return;
    }
    // Plain click: select only (desktop feel). Double-click plays, the
    // lyrics badge opens the drawer.
    selectSingle(file.rel_path);
  };

  // Global shortcuts — ignored while typing in an input.
  // ⌘/Ctrl+A select all · Delete removes · Escape clears · ↑/↓ move the
  // selection (⇧ extends) · Enter plays it · Space toggles play/pause.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = document.activeElement as HTMLElement | null;
      const typing =
        !!el &&
        (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" ||
          el.isContentEditable);
      if (typing) return;

      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "a") {
        e.preventDefault();
        setSelected(new Set(files.map((f) => f.rel_path)));
        setAnchorPath(null);
        return;
      }
      if ((e.key === "Delete" || e.key === "Backspace") && selected.size > 0) {
        e.preventDefault();
        selected.forEach(onDelete);
        clearSelection();
        return;
      }
      if (e.key === "Escape" && selected.size > 0) {
        clearSelection();
        return;
      }

      const onInteractive = () =>
        !!el && !!el.closest && el.closest("button, a, input, textarea, select, [role=button]");

      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        if (files.length === 0 || onInteractive()) return;
        e.preventDefault();
        const dir = e.key === "ArrowDown" ? 1 : -1;
        const anchor = anchorPath ?? currentPlayingPath ?? files[0].rel_path;
        let idx = files.findIndex((f) => f.rel_path === anchor);
        if (idx < 0) idx = 0;
        const nextIdx = Math.min(files.length - 1, Math.max(0, idx + dir));
        const nextPath = files[nextIdx].rel_path;
        if (e.shiftKey) selectRange(nextPath);
        else selectSingle(nextPath);
        return;
      }
      if (e.key === "Enter") {
        if (selected.size > 0) {
          e.preventDefault();
          playSelection();
        }
        return;
      }
      if (e.key === " " && currentPlayingPath && !onInteractive()) {
        e.preventDefault();
        onTrackPlay(currentPlayingPath);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [files, selected, anchorPath, currentPlayingPath, onDelete]);

  const playSelection = () => {
    const paths = selectedFiles.map((f) => f.rel_path);
    if (paths.length === 0) return;
    if (onPlayList) onPlayList(paths, 0);
    else onTrackPlay(paths[0]);
  };

  const toggleLikeSelection = () => {
    if (!onToggleLike) return;
    const anyUnliked = selectedFiles.some((f) => !(likes?.has(f.rel_path) ?? false));
    selectedFiles.forEach((f) => {
      const isLiked = likes?.has(f.rel_path) ?? false;
      if (anyUnliked ? !isLiked : isLiked) onToggleLike(f.rel_path);
    });
  };

  const deleteSelection = () => {
    selectedFiles.forEach((f) => onDelete(f.rel_path));
    clearSelection();
  };

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

    if (diffMins < 1) return t("Just now");
    if (diffMins < 60) return `${diffMins}${t("m ago")}`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}${t("h ago")}`;
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
            <th className="py-2.5 px-4">{t("Title")}</th>
            <th className="py-2.5 px-4 hidden md:table-cell">{t("Album")}</th>
            <th className="py-2.5 px-4 hidden sm:table-cell">{t("Added")}</th>
            <th className="py-2.5 px-4 text-center w-28">{t("Lyrics Sync")}</th>
            <th className="py-2.5 px-4 text-right w-16">{t("Time")}</th>
            <th className="py-2.5 px-4 text-right w-12"></th>
          </tr>
        </thead>

        <tbody>
          {/* Active Downloading Song Rows */}
          {downloadingTasks.map((task, idx) => {
            const currentSong = task.current_track || task.url;
            const total = task.total_tracks || 0;
            const done = task.completed_count || 0;
            const percent = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;

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
                        <span>{cleanTitle(currentSong)}</span>
                        {total > 0 && (
                          <span className="text-[10px] bg-apple-pink text-white font-bold px-2 py-0.5 rounded-full">
                            {done}/{total} ({percent}%)
                          </span>
                        )}
                      </p>
                      <p className="text-[11px] text-apple-pink font-medium truncate">
                        {task.status === "queued" ? t("Queued in download engine...") : task.progress}
                      </p>
                    </div>
                  </div>
                </td>

                <td className="py-3 px-4 text-apple-subtext hidden md:table-cell truncate">
                  {task.order === "reverse" ? t("Newest Added First") : t("Import Queue")}
                </td>

                <td className="py-3 px-4 text-apple-subtext hidden sm:table-cell">
                  <div className="flex items-center gap-1 text-[11px] text-apple-pink">
                    <Clock className="w-3 h-3" />
                    <span>{t("Syncing now")}</span>
                  </div>
                </td>

                <td className="py-3 px-4 text-center">
                  <span className="text-[10px] font-bold text-apple-pink bg-apple-pink/20 px-2 py-0.5 rounded-full border border-apple-pink/30">
                    {t("Downloading")}
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
            const isRowSelected = selected.has(file.rel_path);

            return (
              <tr
                key={file.rel_path}
                draggable={!!onReorder}
                onDragStart={(e) => {
                  setDragPath(file.rel_path);
                  setDropTarget(null);
                  e.dataTransfer.effectAllowed = "move";
                }}
                onDragOver={(e) => {
                  if (onReorder && dragPath && dragPath !== file.rel_path) {
                    e.preventDefault();
                    setDropTarget(idx);
                  }
                }}
                onDrop={(e) => {
                  e.preventDefault();
                  if (dragPath && dragPath !== file.rel_path && onReorder) {
                    onReorder(dragPath, idx);
                  }
                  setDragPath(null);
                  setDropTarget(null);
                }}
                onDragEnd={() => {
                  setDragPath(null);
                  setDropTarget(null);
                }}
                className={`border-b border-white/5 group hover:bg-white/5 transition-colors cursor-pointer ${
                  isSelected
                    ? "bg-apple-pink/15 text-white"
                    : isRowSelected
                    ? "bg-apple-pink/10 text-white shadow-[inset_2px_0_0_0_rgb(250,45,72)]"
                    : "text-zinc-300"
                } ${dragPath === file.rel_path ? "opacity-40" : ""} ${
                  dropTarget === idx
                    ? "shadow-[inset_0_2px_0_0_rgb(250,45,72)]"
                    : ""
                }`}
                onClick={(e) => handleRowClick(e, file)}
                onDoubleClick={() => onTrackPlay(file.rel_path)}
                onContextMenu={(e) => {
                  e.preventDefault();
                  trackCtx.setCtx({ x: e.clientX, y: e.clientY, file });
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
                        {cleanTitle(file.title)}
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

                {/* Date Added */}
                <td className="py-2.5 px-4 text-apple-subtext hidden sm:table-cell text-[11px] font-normal">
                  {formatModTime(file.mod_time)}
                </td>

                {/* Lyrics Sync Status Badge — click opens the lyrics drawer */}
                <td className="py-2.5 px-4 text-center">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      if (onSelectTrack) onSelectTrack(file);
                    }}
                    disabled={!onSelectTrack}
                    title={
                      onSelectTrack
                        ? t("View Lyrics →")
                        : isSynced
                        ? t("Time-Synced Karaoke LRC lyrics")
                        : undefined
                    }
                    className={`inline-block ${
                      onSelectTrack ? "hover:opacity-75 transition-opacity cursor-pointer" : "cursor-default"
                    }`}
                  >
                    {isSynced ? (
                      <span className="inline-block text-[10px] font-semibold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded">
                        {t("Synced")}
                      </span>
                    ) : isUnsynced ? (
                      <span className="inline-block text-[10px] font-semibold text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded">
                        {t("Text")}
                      </span>
                    ) : (
                      <span className="inline-block text-[10px] font-semibold text-rose-400 bg-rose-500/10 border border-rose-500/20 px-2 py-0.5 rounded">
                        {t("Missing")}
                      </span>
                    )}
                  </button>
                </td>

                {/* Duration */}
                <td className="py-2.5 px-4 text-right text-apple-subtext text-[11px] font-normal">
                  {formatDuration(file.duration)}
                </td>

                {/* Actions */}
                <td className="py-2.5 px-4 text-right relative">
                  <div className="flex items-center justify-end gap-0.5 opacity-0 group-hover:opacity-100 transition-all">
                    {likes && onToggleLike && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onToggleLike(file.rel_path);
                        }}
                        className="p-1 rounded text-zinc-500 hover:bg-white/10 transition-all"
                        title={likes.has(file.rel_path) ? "Unlike" : "Like"}
                      >
                        <Heart
                          className={`w-3.5 h-3.5 ${
                            likes.has(file.rel_path)
                              ? "text-apple-pink fill-apple-pink"
                              : "hover:text-apple-pink"
                          }`}
                        />
                      </button>
                    )}
                    {onAddToPlaylist && onCreatePlaylist && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setMenuFor(menuFor === file.rel_path ? null : file.rel_path);
                        }}
                        className="p-1 rounded text-zinc-500 hover:text-apple-pink hover:bg-white/10 transition-all"
                        title={t("Add to Playlist")}
                      >
                        {menuFor === file.rel_path ? (
                          <ListPlus className="w-3.5 h-3.5 text-apple-pink" />
                        ) : (
                          <Plus className="w-3.5 h-3.5" />
                        )}
                      </button>
                    )}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onDelete(file.rel_path);
                      }}
                      className="p-1 rounded text-zinc-500 hover:text-rose-400 hover:bg-white/10 transition-all"
                      title={t("Delete track")}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>

                  {/* Add-to-playlist popover */}
                  {menuFor === file.rel_path && (
                    <>
                      <div className="fixed inset-0 z-40" onClick={(e) => { e.stopPropagation(); setMenuFor(null); }} />
                      <div
                        className="absolute right-8 top-1/2 -translate-y-1/2 z-50 w-52 bg-[#1e1e22] border border-white/10 rounded-xl p-1.5 shadow-2xl text-xs"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <div className="px-2 py-1 text-[10px] font-semibold text-apple-subtext uppercase tracking-wider">
                          {t("Add to Playlist")}
                        </div>
                        <div className="max-h-36 overflow-y-auto scrollbar-none space-y-0.5">
                          {playlists.map((p) => (
                            <button
                              key={p.id}
                              onClick={() => {
                                onAddToPlaylist?.(p.id, file.rel_path);
                                setMenuFor(null);
                              }}
                              className="w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-lg text-left hover:bg-white/10 transition-colors"
                            >
                              <span className="truncate text-zinc-200">{p.name}</span>
                              <span className="text-[10px] text-apple-subtext shrink-0">{p.track_count}</span>
                            </button>
                          ))}
                          {playlists.length === 0 && (
                            <div className="px-2 py-1.5 text-zinc-500">{t("No playlists yet")}</div>
                          )}
                        </div>
                        <form
                          className="mt-1 border-t border-white/10 pt-1.5"
                          onSubmit={(e) => {
                            e.preventDefault();
                            if (newPlaylistName.trim() && onCreatePlaylist) {
                              onCreatePlaylist(newPlaylistName.trim());
                              setNewPlaylistName("");
                              setMenuFor(null);
                            }
                          }}
                        >
                          <input
                            value={newPlaylistName}
                            onChange={(e) => setNewPlaylistName(e.target.value)}
                            placeholder={t("New Playlist")}
                            className="w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg px-2 py-1.5 text-xs text-white placeholder-apple-subtext focus:outline-none"
                          />
                        </form>
                      </div>
                    </>
                  )}
                </td>
              </tr>
            );
          })}

          {files.length === 0 && downloadingTasks.length === 0 && (
            <tr>
              <td colSpan={7} className="text-center py-20 text-apple-subtext">
                <Music className="w-10 h-10 mx-auto mb-3 opacity-30" />
                <p className="text-sm font-semibold text-zinc-400">{t("No tracks imported yet")}</p>
                <p className="text-xs text-zinc-500 mt-1">
                  {t("Paste a playlist or track URL above to start syncing")}
                </p>
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {/* Shared right-click context menu — works on any track in the app */}
      <TrackContextMenu
        ctx={trackCtx.ctx}
        close={trackCtx.close}
        onPlay={onTrackPlay}
        onPlayNext={onPlayNext}
        likes={likes}
        onToggleLike={onToggleLike}
        onSelectTrack={onSelectTrack}
        playlists={playlists}
        onAddToPlaylist={onAddToPlaylist}
        onCreatePlaylist={onCreatePlaylist}
        onDelete={onDelete}
      />

      {/* Selection action bar — a floating glass pill above the player */}
      {selected.size > 0 && (
        <div className="sticky bottom-28 z-50 flex justify-center pointer-events-none">
          <div className="pointer-events-auto relative">
            <Glass cornerRadius={999} className="w-fit">
              <div className="flex items-center gap-1 pl-4 pr-2 py-1.5 text-xs">
                <span className="font-bold text-white mr-2">
                  {selectedFiles.length} {t("selected")}
                </span>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    playSelection();
                  }}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-apple-pink text-white font-semibold hover:bg-apple-pinkHover transition-colors"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  {t("Play")}
                </button>
                {onPlayNext && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onPlayNext(selectedFiles.map((f) => f.rel_path));
                      clearSelection();
                    }}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-white/10 text-white font-semibold hover:bg-white/20 transition-colors"
                    title={t("Play Next")}
                  >
                    <ListMusic className="w-3.5 h-3.5" />
                    {t("Play Next")}
                  </button>
                )}
                {likes && onToggleLike && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleLikeSelection();
                    }}
                    className="p-2 rounded-full text-zinc-300 hover:bg-white/10 hover:text-apple-pink transition-colors"
                    title={t("Like")}
                  >
                    <Heart className="w-3.5 h-3.5" />
                  </button>
                )}
                {onAddToPlaylist && onCreatePlaylist && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setMenuFor(menuFor === SELECTION_KEY ? null : SELECTION_KEY);
                    }}
                    className="p-2 rounded-full text-zinc-300 hover:bg-white/10 hover:text-apple-pink transition-colors"
                    title={t("Add to Playlist")}
                  >
                    <ListPlus className="w-3.5 h-3.5" />
                  </button>
                )}
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteSelection();
                  }}
                  className="p-2 rounded-full text-zinc-300 hover:bg-rose-500/20 hover:text-rose-400 transition-colors"
                  title={t("Delete track")}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    clearSelection();
                  }}
                  className="p-2 rounded-full text-zinc-500 hover:bg-white/10 hover:text-white transition-colors"
                  title={t("Close")}
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
            </Glass>

            {/* Add-to-playlist popover for the selection */}
            {menuFor === SELECTION_KEY && (
              <>
                <div
                  className="fixed inset-0 z-40 pointer-events-auto"
                  onClick={(e) => {
                    e.stopPropagation();
                    setMenuFor(null);
                  }}
                />
                <div
                  className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 w-52 bg-[#1e1e22]/95 backdrop-blur-xl border border-white/10 rounded-xl p-1.5 shadow-2xl text-xs pointer-events-auto"
                  onClick={(e) => e.stopPropagation()}
                >
                  <div className="px-2 py-1 text-[10px] font-semibold text-apple-subtext uppercase tracking-wider">
                    {t("Add to Playlist")}
                  </div>
                  <div className="max-h-36 overflow-y-auto scrollbar-none space-y-0.5">
                    {playlists.map((p) => (
                      <button
                        key={p.id}
                        onClick={(e) => {
                          e.stopPropagation();
                          selectedFiles.forEach((f) => onAddToPlaylist?.(p.id, f.rel_path));
                          setMenuFor(null);
                        }}
                        className="w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-lg text-left hover:bg-white/10 transition-colors"
                      >
                        <span className="truncate text-zinc-200">{p.name}</span>
                        <span className="text-[10px] text-apple-subtext shrink-0">{p.track_count}</span>
                      </button>
                    ))}
                    {playlists.length === 0 && (
                      <div className="px-2 py-1.5 text-zinc-500">{t("No playlists yet")}</div>
                    )}
                  </div>
                  <form
                    className="mt-1 border-t border-white/10 pt-1.5"
                    onSubmit={(e) => {
                      e.preventDefault();
                      if (newPlaylistName.trim() && onCreatePlaylist) {
                        onCreatePlaylist(newPlaylistName.trim());
                        setNewPlaylistName("");
                        setMenuFor(null);
                      }
                    }}
                  >
                    <input
                      value={newPlaylistName}
                      onChange={(e) => setNewPlaylistName(e.target.value)}
                      placeholder={t("New Playlist")}
                      className="w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg px-2 py-1.5 text-xs text-white placeholder-apple-subtext focus:outline-none"
                    />
                  </form>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
