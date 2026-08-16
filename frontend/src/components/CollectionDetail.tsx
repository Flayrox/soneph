import React from "react";
import { Play, Pin, PinOff, Users, Disc3 } from "lucide-react";
import { Glass } from "./Glass";
import { TrackList } from "./TrackList";
import type { DownloadedFile, DownloadTask, PlaylistSummary } from "@/types";
import type { Pin as PinType } from "@/pins";
import { useI18n } from "@/i18n";

interface CollectionDetailProps {
  kind: "artist" | "album";
  name: string;
  files: DownloadedFile[];
  activeTasks?: DownloadTask[];
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onPlayTrack: (path: string) => void;
  onPlayAll: (paths: string[]) => void;
  onPlayNext?: (paths: string[]) => void;
  onSelectTrack: (track: DownloadedFile) => void;
  onDelete: (path: string) => void;
  getApiUrl: () => string;
  playlists?: PlaylistSummary[];
  onAddToPlaylist?: (playlistId: string, path: string) => void;
  onCreatePlaylist?: (name: string) => void;
  likes?: Set<string>;
  onToggleLike?: (path: string) => void;
  isPinned: (pin: PinType) => boolean;
  onTogglePin: (pin: PinType) => void;
}

export const CollectionDetail: React.FC<CollectionDetailProps> = ({
  kind,
  name,
  files,
  activeTasks = [],
  currentPlayingPath,
  isPlaying,
  onPlayTrack,
  onPlayAll,
  onPlayNext,
  onSelectTrack,
  onDelete,
  getApiUrl,
  playlists,
  onAddToPlaylist,
  onCreatePlaylist,
  likes,
  onToggleLike,
  isPinned,
  onTogglePin,
}) => {
  const { t } = useI18n();
  const pin: PinType = { kind, value: name };
  const pinned = isPinned(pin);
  const Icon = kind === "artist" ? Users : Disc3;
  const cover = files[0];

  return (
    <div className="select-none">
      {/* Header */}
      <div className="px-6 py-6">
      <Glass cornerRadius={16}>
      <div className="px-5 py-5 flex items-center gap-5">
        <div className="w-24 h-24 rounded-xl bg-apple-pink/15 border border-apple-pink/30 flex items-center justify-center shrink-0 shadow-lg overflow-hidden relative">
          <Icon className="w-10 h-10 text-apple-pink" />
          {cover && (
            <img
              src={`${getApiUrl()}/cover?path=${encodeURIComponent(cover.rel_path)}`}
              alt={name}
              className="absolute inset-0 w-full h-full object-cover"
              onError={(e) => {
                e.currentTarget.style.display = "none";
              }}
            />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <span className="text-[11px] font-semibold text-apple-pink uppercase tracking-wider">
            {kind === "artist" ? t("Artist") : t("Album")}
          </span>
          <h2 className="text-2xl font-bold text-white truncate">{name}</h2>
          <p className="text-xs text-apple-subtext mt-1">
            {files.length} {t("songs")}
          </p>
          <div className="flex items-center gap-2 mt-3">
            <button
              onClick={() => onPlayAll(files.map((f) => f.rel_path))}
              disabled={files.length === 0}
              className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-40 px-4 py-2 rounded-full transition-colors"
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              {t("Play All")}
            </button>
            <button
              onClick={() => onTogglePin(pin)}
              className={`flex items-center gap-1.5 text-xs font-semibold px-3 py-2 rounded-full transition-colors ${
                pinned
                  ? "bg-apple-pink/20 text-apple-pink"
                  : "text-zinc-400 hover:text-white hover:bg-white/10"
              }`}
            >
              {pinned ? <PinOff className="w-3.5 h-3.5" /> : <Pin className="w-3.5 h-3.5" />}
              {pinned ? t("Unpin") : t("Pin")}
            </button>
          </div>
        </div>
      </div>
      </Glass>
      </div>

      {/* Tracks */}
      {files.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-apple-subtext select-none">
          <Icon className="w-10 h-10 mb-3 opacity-30" />
          <p className="text-sm font-semibold text-zinc-400">
            {kind === "artist" ? t("No songs by this artist") : t("No songs on this album")}
          </p>
        </div>
      ) : (
        <TrackList
          files={files}
          activeTasks={activeTasks}
          currentPlayingPath={currentPlayingPath}
          isPlaying={isPlaying}
          onTrackPlay={onPlayTrack}
          onPlayNext={onPlayNext}
          onSelectTrack={onSelectTrack}
          onDelete={onDelete}
          getApiUrl={getApiUrl}
          playlists={playlists}
          onAddToPlaylist={onAddToPlaylist}
          onCreatePlaylist={onCreatePlaylist}
          likes={likes}
          onToggleLike={onToggleLike}
        />
      )}
    </div>
  );
};
