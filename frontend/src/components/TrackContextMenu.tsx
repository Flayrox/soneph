import React, { useEffect, useState } from "react";
import { Play, ListMusic, Heart, MessageSquareQuote, Plus, Trash2, ChevronLeft, ListPlus } from "lucide-react";
import { cleanTitle } from "@/format";
import { Glass } from "./Glass";
import type { DownloadedFile, PlaylistSummary } from "@/types";
import { useI18n } from "@/i18n";

// ── Shared right-click context menu for tracks ──────────────────────────
// Centralized so the SAME menu works from the library table, the Home
// cards (recent / top / liked), collection details, playlists — anywhere.
// A view calls `useTrackCtxMenu()` once, wires `onContextMenu` on its rows
// and renders `<TrackContextMenu ... />` at the end. The Add-to-playlist
// popover is built in, so the host needs no extra wiring.

export interface TrackCtxState {
  x: number;
  y: number;
  file: DownloadedFile;
}

/** Manages open/close state + global dismissal (outside click, Escape, scroll). */
export function useTrackCtxMenu() {
  const [ctx, setCtx] = useState<TrackCtxState | null>(null);

  useEffect(() => {
    if (!ctx) return;
    const close = () => setCtx(null);
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    window.addEventListener("mousedown", close);
    window.addEventListener("keydown", onKey);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("mousedown", close);
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", close, true);
    };
  }, [ctx]);

  return { ctx, setCtx, close: () => setCtx(null) };
}

export interface TrackContextMenuProps {
  ctx: TrackCtxState | null;
  close: () => void;
  onPlay: (path: string) => void;
  onPlayNext?: (paths: string[]) => void;
  likes?: Set<string>;
  onToggleLike?: (path: string) => void;
  onSelectTrack?: (track: DownloadedFile) => void;
  playlists?: PlaylistSummary[];
  onAddToPlaylist?: (playlistId: string, path: string) => void;
  onCreatePlaylist?: (name: string) => void;
  onDelete: (path: string) => void;
}

export const TrackContextMenu: React.FC<TrackContextMenuProps> = ({
  ctx,
  close,
  onPlay,
  onPlayNext,
  likes,
  onToggleLike,
  onSelectTrack,
  playlists = [],
  onAddToPlaylist,
  onCreatePlaylist,
  onDelete,
}) => {
  const { t } = useI18n();
  const [inPlaylists, setInPlaylists] = useState(false);
  const [newName, setNewName] = useState("");

  // Reset the sub-view each time the menu opens on a different track.
  useEffect(() => {
    setInPlaylists(false);
    setNewName("");
  }, [ctx?.file.rel_path]);

  if (!ctx) return null;

  const file = ctx.file;

  return (
    <>
      <div className="fixed inset-0 z-[70]" onMouseDown={close} />
      <div
        className="fixed z-[80] w-56"
        style={{
          left: Math.min(ctx.x, window.innerWidth - 236),
          top: Math.min(ctx.y, window.innerHeight - 340),
        }}
      >
        <Glass cornerRadius={12} className="w-full">
          <div className="p-1 text-xs">
            {!inPlaylists ? (
              <>
                {/* Track header */}
                <div className="px-2.5 py-1.5 border-b border-white/10 mb-1">
                  <p className="font-bold text-white truncate">{cleanTitle(file.title)}</p>
                  <p className="text-[10px] text-apple-subtext truncate">{file.artist}</p>
                </div>

                <Item
                  icon={<Play className="w-3.5 h-3.5" />}
                  label={t("Play")}
                  onClick={() => {
                    onPlay(file.rel_path);
                    close();
                  }}
                />
                {onPlayNext && (
                  <Item
                    icon={<ListMusic className="w-3.5 h-3.5" />}
                    label={t("Play Next")}
                    onClick={() => {
                      onPlayNext([file.rel_path]);
                      close();
                    }}
                  />
                )}
                {likes && onToggleLike && (
                  <Item
                    icon={
                      <Heart
                        className={`w-3.5 h-3.5 ${
                          likes.has(file.rel_path) ? "fill-apple-pink text-apple-pink" : ""
                        }`}
                      />
                    }
                    label={likes.has(file.rel_path) ? t("Unlike") : t("Like")}
                    onClick={() => {
                      onToggleLike(file.rel_path);
                      close();
                    }}
                  />
                )}
                {onSelectTrack && (
                  <Item
                    icon={<MessageSquareQuote className="w-3.5 h-3.5" />}
                    label={t("View Lyrics →")}
                    onClick={() => {
                      onSelectTrack(file);
                      close();
                    }}
                  />
                )}
                {onAddToPlaylist && (
                  <Item
                    icon={<ListPlus className="w-3.5 h-3.5" />}
                    label={t("Add to Playlist")}
                    onClick={() => setInPlaylists(true)}
                  />
                )}
                <div className="border-t border-white/10 my-1" />
                <Item
                  danger
                  icon={<Trash2 className="w-3.5 h-3.5" />}
                  label={t("Delete track")}
                  onClick={() => {
                    onDelete(file.rel_path);
                    close();
                  }}
                />
              </>
            ) : (
              <>
                {/* Add to playlist sub-view */}
                <button
                  onClick={() => setInPlaylists(false)}
                  className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-left text-apple-subtext hover:bg-white/10 hover:text-white transition-colors"
                >
                  <ChevronLeft className="w-3.5 h-3.5" />
                  <span className="font-semibold uppercase tracking-wider text-[10px]">
                    {t("Add to Playlist")}
                  </span>
                </button>
                <div className="max-h-40 overflow-y-auto scrollbar-none space-y-0.5 mt-1">
                  {playlists.map((p) => (
                    <button
                      key={p.id}
                      onClick={() => {
                        onAddToPlaylist?.(p.id, file.rel_path);
                        close();
                      }}
                      className="w-full flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg text-left hover:bg-white/10 transition-colors"
                    >
                      <span className="truncate text-zinc-200">{p.name}</span>
                      <span className="text-[10px] text-apple-subtext shrink-0">{p.track_count}</span>
                    </button>
                  ))}
                  {playlists.length === 0 && (
                    <div className="px-2.5 py-1.5 text-zinc-500">{t("No playlists yet")}</div>
                  )}
                </div>
                <form
                  className="mt-1 border-t border-white/10 pt-1.5"
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (newName.trim() && onCreatePlaylist) {
                      onCreatePlaylist(newName.trim());
                      setNewName("");
                      close();
                    }
                  }}
                >
                  <div className="relative">
                    <Plus className="w-3 h-3 absolute left-2.5 top-2 text-apple-subtext" />
                    <input
                      autoFocus
                      value={newName}
                      onChange={(e) => setNewName(e.target.value)}
                      placeholder={t("New Playlist")}
                      className="w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg pl-7 pr-2 py-1.5 text-xs text-white placeholder-apple-subtext focus:outline-none"
                    />
                  </div>
                </form>
              </>
            )}
          </div>
        </Glass>
      </div>
    </>
  );
};

/** One row of the shared context menu. */
const Item: React.FC<{
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  danger?: boolean;
}> = ({ icon, label, onClick, danger = false }) => (
  <button
    onMouseDown={(e) => e.stopPropagation()}
    onClick={(e) => {
      e.stopPropagation();
      onClick();
    }}
    className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-left transition-colors ${
      danger ? "text-rose-400 hover:bg-rose-500/15" : "text-zinc-200 hover:bg-white/10"
    }`}
  >
    <span className="text-apple-subtext shrink-0">{icon}</span>
    <span className="truncate">{label}</span>
  </button>
);
