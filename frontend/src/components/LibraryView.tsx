import React from "react";
import { TrackList } from "./TrackList";
import type { PluginViewProps } from "@/framework/plugin.types";

/**
 * Renders the library track table for the "All Music" and "Liked" nav views.
 * Reads everything it needs from the shared `{ app }` context.
 */
export const LibraryView: React.FC<PluginViewProps> = ({ app }) => {
  const files =
    app.nav === "liked"
      ? app.likedFiles.filter((f) => app.filteredFiles.includes(f))
      : app.filteredFiles;

  return (
    <TrackList
      files={files}
      activeTasks={app.tasks}
      currentPlayingPath={app.currentPlayingPath}
      isPlaying={app.isPlaying}
      onTrackPlay={app.playTrack}
      onPlayList={app.playList}
      onPlayNext={app.playNext}
      onSelectTrack={app.openLyricsDrawer}
      onDelete={app.deleteFile}
      getApiUrl={app.getApiUrl}
      playlists={app.playlists}
      onAddToPlaylist={app.addToPlaylist}
      onCreatePlaylist={app.createPlaylist}
      likes={app.likes}
      onToggleLike={app.toggleLike}
    />
  );
};
