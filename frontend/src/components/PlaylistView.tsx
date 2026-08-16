import React from "react";
import { Play, Trash2, ListMusic } from "lucide-react";
import { TrackList } from "./TrackList";
import type { DownloadedFile, Playlist, PlaylistSummary } from "@/types";
import { useI18n } from "@/i18n";

interface PlaylistViewProps {
  playlist: Playlist | null;
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onPlayTrack: (relPath: string) => void;
  onSelectTrack: (track: DownloadedFile) => void;
  onRemoveTrack: (path: string) => void;
  onDeletePlaylist: () => void;
  onPlayAll: () => void;
  getApiUrl: () => string;
  playlists: PlaylistSummary[];
  onAddToPlaylist: (playlistId: string, path: string) => void;
  onCreatePlaylist: (name: string, path?: string) => void;
  likes?: Set<string>;
  onToggleLike?: (path: string) => void;
}

export const PlaylistView: React.FC<PlaylistViewProps> = ({
  playlist,
  currentPlayingPath,
  isPlaying,
  onPlayTrack,
  onSelectTrack,
  onRemoveTrack,
  onDeletePlaylist,
  onPlayAll,
  getApiUrl,
  playlists,
  onAddToPlaylist,
  onCreatePlaylist,
  likes,
  onToggleLike,
}) => {
  const { t } = useI18n();

  if (!playlist) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-apple-subtext select-none">
        <ListMusic className="w-12 h-12 mb-3 opacity-30" />
        <p className="text-sm font-semibold text-zinc-400">{t("Loading…")}</p>
      </div>
    );
  }

  return (
    <div className="select-none">
      {/* Playlist header */}
      <div className="px-6 py-6 flex items-center gap-5 bg-gradient-to-b from-white/5 to-transparent border-b border-white/10">
        <div className="w-24 h-24 rounded-xl bg-apple-pink/15 border border-apple-pink/30 flex items-center justify-center shrink-0 shadow-lg">
          <ListMusic className="w-10 h-10 text-apple-pink" />
        </div>
        <div className="min-w-0 flex-1">
          <h2 className="text-2xl font-bold text-white truncate">{playlist.name}</h2>
          <p className="text-xs text-apple-subtext mt-1">
            {playlist.tracks.length} {t("songs")}
          </p>
          <div className="flex items-center gap-2 mt-3">
            <button
              onClick={onPlayAll}
              disabled={playlist.tracks.length === 0}
              className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-40 px-4 py-2 rounded-full transition-colors"
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              {t("Play All")}
            </button>
            <button
              onClick={onDeletePlaylist}
              className="flex items-center gap-1.5 text-xs font-semibold text-zinc-400 hover:text-rose-400 px-3 py-2 rounded-full transition-colors"
            >
              <Trash2 className="w-3.5 h-3.5" />
              {t("Delete Playlist")}
            </button>
          </div>
        </div>
      </div>

      {/* Tracks */}
      {playlist.tracks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-apple-subtext select-none">
          <ListMusic className="w-10 h-10 mb-3 opacity-30" />
          <p className="text-sm font-semibold text-zinc-400">{t("This playlist is empty")}</p>
          <p className="text-xs text-zinc-500 mt-1">{t("Add tracks from the library with the + button")}</p>
        </div>
      ) : (
        <TrackList
          files={playlist.tracks}
          activeTasks={[]}
          currentPlayingPath={currentPlayingPath}
          isPlaying={isPlaying}
          onTrackPlay={onPlayTrack}
          onSelectTrack={onSelectTrack}
          onDelete={onRemoveTrack}
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
