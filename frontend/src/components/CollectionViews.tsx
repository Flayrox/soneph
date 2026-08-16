import React from "react";
import { CollectionGrid } from "./CollectionGrid";
import { CollectionDetail } from "./CollectionDetail";
import type { PluginViewProps } from "@/framework/plugin.types";

/**
 * Artists grid — reads the grouped artists + pin helpers from { app }.
 */
export const ArtistsView: React.FC<PluginViewProps> = ({ app }) => (
  <CollectionGrid
    kind="artist"
    entries={app.artists}
    getApiUrl={app.getApiUrl}
    onOpen={(name) => app.setNav(`artist:${encodeURIComponent(name)}`)}
    isPinned={app.isPinned}
    onTogglePin={app.togglePin}
    onPlayAll={(paths) => app.playList(paths, 0)}
    onPlayNext={app.playNext}
  />
);

/**
 * Albums grid — same contract, different kind.
 */
export const AlbumsView: React.FC<PluginViewProps> = ({ app }) => (
  <CollectionGrid
    kind="album"
    entries={app.albums}
    getApiUrl={app.getApiUrl}
    onOpen={(name) => app.setNav(`album:${encodeURIComponent(name)}`)}
    isPinned={app.isPinned}
    onTogglePin={app.togglePin}
    onPlayAll={(paths) => app.playList(paths, 0)}
    onPlayNext={app.playNext}
  />
);

/**
 * Artist / album detail — the nav is `artist:NAME` or `album:NAME`;
 * the host routes both to this view, which derives everything from { app }.
 */
export const CollectionDetailView: React.FC<PluginViewProps> = ({ app }) => {
  const kind = app.nav.startsWith("artist:") ? ("artist" as const) : ("album" as const);
  const name = app.nav.includes(":") ? decodeURIComponent(app.nav.split(":").slice(1).join(":")) : "";
  const files = app.files.filter(
    (f) => (kind === "artist" ? f.artist : f.album) === name
  );

  return (
    <CollectionDetail
      kind={kind}
      name={name}
      files={files}
      currentPlayingPath={app.currentPlayingPath}
      isPlaying={app.isPlaying}
      onPlayTrack={app.playTrack}
      onPlayAll={(paths) => app.playList(paths, 0)}
      onPlayNext={app.playNext}
      onSelectTrack={app.openLyricsDrawer}
      onDelete={app.deleteFile}
      getApiUrl={app.getApiUrl}
      playlists={app.playlists}
      onAddToPlaylist={app.addToPlaylist}
      onCreatePlaylist={app.createPlaylist}
      likes={app.likes}
      onToggleLike={app.toggleLike}
      isPinned={app.isPinned}
      onTogglePin={app.togglePin}
    />
  );
};
