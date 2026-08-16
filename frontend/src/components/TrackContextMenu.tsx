import React, { useEffect, useState } from "react";
import {
  Play,
  ListMusic,
  Heart,
  MessageSquareQuote,
  Plus,
  Trash2,
  ChevronLeft,
  ListPlus,
  Info,
  Loader2,
  ExternalLink,
} from "lucide-react";
import { cleanTitle } from "@/format";
import { Glass } from "./Glass";
import { apiFetch } from "@/api";
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
  const [inDetails, setInDetails] = useState(false);
  const [details, setDetails] = useState<Record<string, any> | null>(null);
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [detailsError, setDetailsError] = useState(false);

  // Reset the sub-views each time the menu opens on a different track.
  useEffect(() => {
    setInPlaylists(false);
    setInDetails(false);
    setNewName("");
    setDetails(null);
    setDetailsError(false);
  }, [ctx?.file.rel_path]);

  // Fetch the full ID3 metadata for the details panel.
  useEffect(() => {
    if (!inDetails || !ctx) return;
    let cancelled = false;
    setDetailsLoading(true);
    setDetailsError(false);
    apiFetch(`/api/file/details?path=${encodeURIComponent(ctx.file.rel_path)}`)
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        if (!cancelled) setDetails(data);
      })
      .catch(() => {
        if (!cancelled) setDetailsError(true);
      })
      .finally(() => {
        if (!cancelled) setDetailsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [inDetails, ctx]);

  if (!ctx) return null;

  const file = ctx.file;
  const width = inDetails ? 384 : 224;
  const reserve = inDetails ? 600 : 340;

  return (
    <>
      <div className="fixed inset-0 z-[70]" onMouseDown={close} />
      <div
        className={`fixed z-[80] ${inDetails ? "" : "w-56"}`}
        style={{
          width: inDetails ? width : undefined,
          left: Math.min(ctx.x, window.innerWidth - width - 12),
          top: Math.min(ctx.y, window.innerHeight - reserve),
        }}
      >
        <Glass cornerRadius={12} className="w-full">
          <div className="p-1 text-xs">
            {inDetails ? (
              <DetailsView
                file={file}
                details={details}
                loading={detailsLoading}
                error={detailsError}
                onBack={() => setInDetails(false)}
              />
            ) : !inPlaylists ? (
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
                <Item
                  icon={<Info className="w-3.5 h-3.5" />}
                  label={t("More details")}
                  onClick={() => setInDetails(true)}
                />
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

/** Sub-view « Plus de détails » : métadonnées ID3 complètes du morceau. */
const DetailsView: React.FC<{
  file: DownloadedFile;
  details: Record<string, any> | null;
  loading: boolean;
  error: boolean;
  onBack: () => void;
}> = ({ file, details, loading, error, onBack }) => {
  const { t } = useI18n();

  const fmtDuration = (s?: number) => {
    if (!s) return null;
    const m = Math.floor(s / 60);
    const sec = Math.floor(s % 60);
    return `${m}:${sec.toString().padStart(2, "0")}`;
  };

  const people = (pairs?: string[][]) =>
    (pairs ?? []).map(([role, name]) => `${role} — ${name}`).join(" · ") || null;

  const lyricsType =
    details?.lyrics_type === "synced"
      ? t("Synced")
      : details?.lyrics_type === "unsynced"
        ? t("Plain")
        : details?.lyrics_type === "none"
          ? t("Missing")
          : null;

  return (
    <div className="max-h-[480px] flex flex-col">
      <div className="px-2.5 py-1.5 border-b border-white/10 mb-1 flex items-center gap-2">
        <button
          onClick={onBack}
          className="flex items-center gap-1 px-1.5 py-1 rounded-lg text-apple-subtext hover:bg-white/10 hover:text-white transition-colors"
        >
          <ChevronLeft className="w-3.5 h-3.5" />
          <span className="font-semibold uppercase tracking-wider text-[10px]">
            {t("Details")}
          </span>
        </button>
      </div>

      <div className="px-3 pb-3 overflow-y-auto scrollbar-none text-[11px]">
        {loading && (
          <div className="flex items-center gap-2 py-2 text-apple-subtext">
            <Loader2 className="w-3.5 h-3.5 animate-spin" /> {t("Loading…")}
          </div>
        )}
        {error && <p className="py-2 text-rose-400">{t("Action failed")}</p>}
        {details && (
          <div className="space-y-px">
            <Row label={t("Quality")} value={details.quality} />
            <Row label={t("Duration")} value={fmtDuration(details.duration_seconds)} />
            <Row
              label={t("Lyrics")}
              value={[lyricsType, details.lyrics_source].filter(Boolean).join(" · ")}
            />
            <Row label={t("Artist")} value={details.artist} />
            <Row label={t("Album")} value={details.album} />
            <Row label={t("Album artist")} value={details.album_artist} />
            <Row label={t("Year")} value={details.year} />
            <Row label={t("Genre")} value={details.genre} />
            <Row label={t("Track")} value={details.track} />
            <Row label={t("Disc")} value={details.disc} />
            <Row label={t("Writers")} value={details.writer} />
            <Row label={t("Producers")} value={people(details.involved_people)} />
            <Row label={t("Musicians")} value={people(details.musicians)} />
            <Row label={t("Publisher")} value={details.publisher} />
            <Row label="ISRC" value={details.isrc} />
            <Row label={t("Copyright")} value={details.copyright} />
            <Row label={t("Source")} value={details.source_url} />

            {details.spotify_url && (
              <a
                href={details.spotify_url}
                target="_blank"
                rel="noreferrer"
                className="flex items-center justify-between gap-2 py-1.5 text-apple-pink hover:text-apple-pinkHover transition-colors"
              >
                <span>{t("Open in Spotify")}</span>
                <ExternalLink className="w-3 h-3 shrink-0" />
              </a>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

const Row: React.FC<{ label: string; value?: string | null }> = ({ label, value }) =>
  value ? (
    <div className="flex items-start justify-between gap-3 py-1 border-b border-white/5 last:border-0">
      <span className="text-apple-subtext shrink-0">{label}</span>
      <span className="text-zinc-200 text-right break-words min-w-0 max-w-[240px]">{value}</span>
    </div>
  ) : null;
