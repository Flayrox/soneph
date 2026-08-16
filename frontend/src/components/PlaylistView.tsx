import React, { useMemo, useState } from "react";
import { Play, Trash2, ListMusic, Pin, PinOff, Search, Plus, Users, Disc3 } from "lucide-react";
import { cleanTitle } from "@/format";
import { Glass } from "./Glass";
import { TrackList } from "./TrackList";
import type { DownloadedFile, Playlist, PlaylistSummary } from "@/types";
import type { Pin as PinType } from "@/pins";
import { useI18n } from "@/i18n";

interface PlaylistViewProps {
  playlist: Playlist | null;
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onPlayTrack: (relPath: string) => void;
  onPlayNext?: (paths: string[]) => void;
  onSelectTrack: (track: DownloadedFile) => void;
  onRemoveTrack: (path: string) => void;
  onDeletePlaylist: () => void;
  onPlayAll: () => void;
  getApiUrl: () => string;
  playlists: PlaylistSummary[];
  onAddToPlaylist: (playlistId: string, path: string) => void;
  onCreatePlaylist: (name: string, path?: string) => void;
  onReorder?: (path: string, toIndex: number) => void;
  likes?: Set<string>;
  onToggleLike?: (path: string) => void;
  isPinned?: (pin: PinType) => boolean;
  onTogglePin?: (pin: PinType) => void;
  /** The full library — used to search & suggest tracks to add. */
  libraryFiles?: DownloadedFile[];
  /** Suggest tracks by these artists (from the library). */
  suggestArtists?: string[];
}

export const PlaylistView: React.FC<PlaylistViewProps> = ({
  playlist,
  currentPlayingPath,
  isPlaying,
  onPlayTrack,
  onPlayNext,
  onSelectTrack,
  onRemoveTrack,
  onDeletePlaylist,
  onPlayAll,
  getApiUrl,
  playlists,
  onAddToPlaylist,
  onCreatePlaylist,
  onReorder,
  likes,
  onToggleLike,
  isPinned,
  onTogglePin,
  libraryFiles = [],
  suggestArtists = [],
}) => {
  const { t } = useI18n();
  const [addQuery, setAddQuery] = useState("");

  // ── Search & suggestions to add tracks to the playlist ─────────────
  const inPlaylist = useMemo(
    () => new Set((playlist?.tracks ?? []).map((f) => f.rel_path)),
    [playlist]
  );
  const candidates = useMemo(
    () => libraryFiles.filter((f) => !inPlaylist.has(f.rel_path)),
    [libraryFiles, inPlaylist]
  );

  // Search results: filter the library by title / artist / album.
  const searchResults = useMemo(() => {
    const q = addQuery.trim().toLowerCase();
    if (!q) return [];
    return candidates
      .filter((f) =>
        [f.title, f.artist, f.album].some((s) => s?.toLowerCase().includes(q))
      )
      .slice(0, 12);
  }, [candidates, addQuery]);

  // Suggestions: tracks by the same artists / albums already in the playlist.
  const suggestions = useMemo(() => {
    const artistKeys = suggestArtists.length > 0 ? suggestArtists : [];
    const playlistArtists = artistKeys.length > 0 ? artistKeys : [
      ...new Set((playlist?.tracks ?? []).map((f) => f.artist).filter(Boolean)),
    ];
    const playlistAlbums = [
      ...new Set((playlist?.tracks ?? []).map((f) => f.album).filter(Boolean)),
    ];
    const byArtist = playlistArtists.slice(0, 3).map((artist) => ({
      kind: "artist" as const,
      key: artist,
      items: candidates
        .filter((f) => f.artist === artist)
        .slice(0, 4),
    }));
    const byAlbum = playlistAlbums.slice(0, 3).map((album) => ({
      kind: "album" as const,
      key: album,
      items: candidates
        .filter((f) => f.album === album)
        .slice(0, 4),
    }));
    return [...byArtist, ...byAlbum].filter((g) => g.items.length > 0);
  }, [candidates, playlist, suggestArtists]);

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
      <div className="px-6 py-6">
      <Glass cornerRadius={16}>
      <div className="px-5 py-5 flex items-center gap-5">
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
            {isPinned && onTogglePin && (
              <button
                onClick={() => onTogglePin({ kind: "playlist", value: playlist.id })}
                className={`flex items-center gap-1.5 text-xs font-semibold px-3 py-2 rounded-full transition-colors ${
                  isPinned({ kind: "playlist", value: playlist.id })
                    ? "bg-apple-pink/20 text-apple-pink"
                    : "text-zinc-400 hover:text-white hover:bg-white/10"
                }`}
              >
                {isPinned({ kind: "playlist", value: playlist.id }) ? (
                  <PinOff className="w-3.5 h-3.5" />
                ) : (
                  <Pin className="w-3.5 h-3.5" />
                )}
                {isPinned({ kind: "playlist", value: playlist.id }) ? t("Unpin") : t("Pin")}
              </button>
            )}
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
      </Glass>
      </div>

      {/* Add tracks: search + suggestions */}
      <div className="px-6 pb-2">
        <Glass cornerRadius={12}>
          <div className="p-3">
            <div className="relative">
              <Search className="w-4 h-4 absolute left-3 top-2.5 text-apple-subtext" />
              <input
                type="text"
                value={addQuery}
                onChange={(e) => setAddQuery(e.target.value)}
                placeholder={t("Search tracks to add…")}
                className="w-full bg-[#242428]/80 border border-white/10 focus:border-apple-pink rounded-xl py-2 pl-9 pr-3 text-xs text-white placeholder-apple-subtext focus:outline-none transition-colors"
              />
            </div>

            {addQuery.trim() ? (
              /* Search results */
              <div className="mt-2 space-y-1 max-h-56 overflow-y-auto scrollbar-none">
                {searchResults.length === 0 ? (
                  <p className="px-1 py-2 text-[11px] text-zinc-500">{t("No results")}</p>
                ) : (
                  searchResults.map((f) => (
                    <AddRow
                      key={f.rel_path}
                      file={f}
                      getApiUrl={getApiUrl}
                      onAdd={() => onAddToPlaylist?.(playlist.id, f.rel_path)}
                    />
                  ))
                )}
              </div>
            ) : suggestions.length > 0 ? (
              /* Suggestions based on the playlist's artists & albums */
              <div className="mt-2 space-y-3 max-h-64 overflow-y-auto scrollbar-none">
                {suggestions.map((g) => (
                  <div key={`${g.kind}:${g.key}`}>
                    <div className="flex items-center gap-1.5 px-1 pb-1 text-[10px] font-semibold text-apple-subtext uppercase tracking-wider">
                      {g.kind === "artist" ? (
                        <Users className="w-3 h-3 text-apple-pink" />
                      ) : (
                        <Disc3 className="w-3 h-3 text-apple-pink" />
                      )}
                      {g.kind === "artist" ? t("More from") : t("More from album")}{" "}
                      <span className="text-white/90 truncate">{g.key}</span>
                    </div>
                    <div className="space-y-1">
                      {g.items.map((f) => (
                        <AddRow
                          key={f.rel_path}
                          file={f}
                          getApiUrl={getApiUrl}
                          onAdd={() => onAddToPlaylist?.(playlist.id, f.rel_path)}
                        />
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="px-1 pt-2 pb-1 text-[11px] text-zinc-500">
                {t("Search your library to add tracks to this playlist")}
              </p>
            )}
          </div>
        </Glass>
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
          onPlayNext={onPlayNext}
          onSelectTrack={onSelectTrack}
          onReorder={onReorder}
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

/** One row inside the add-tracks search / suggestions list. */
const AddRow: React.FC<{
  file: DownloadedFile;
  getApiUrl: () => string;
  onAdd: () => void;
}> = ({ file, getApiUrl, onAdd }) => {
  const { t } = useI18n();
  return (
  <div className="flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-white/5 transition-colors group">
    <div className="w-7 h-7 rounded bg-[#2a2a2e] border border-white/10 flex items-center justify-center shrink-0 overflow-hidden relative">
      <ListMusic className="w-3.5 h-3.5 text-apple-subtext absolute inset-0 m-auto opacity-60" />
      <img
        src={`${getApiUrl()}/cover?path=${encodeURIComponent(file.rel_path)}`}
        alt={file.title}
        className="w-full h-full object-cover relative z-10"
        onError={(e) => {
          e.currentTarget.style.display = "none";
        }}
      />
    </div>
    <div className="min-w-0 flex-1">
      <p className="text-[11px] font-semibold text-zinc-100 truncate">{cleanTitle(file.title)}</p>
      <p className="text-[10px] text-apple-subtext truncate">
        {file.artist}
        {file.album ? ` — ${file.album}` : ""}
      </p>
    </div>
    <button
      onClick={onAdd}
      className="p-1.5 rounded-full text-apple-subtext hover:text-white hover:bg-apple-pink/20 transition-colors shrink-0"
      title={t("Add to Playlist")}
    >
      <Plus className="w-3.5 h-3.5" />
    </button>
  </div>
  );
};
