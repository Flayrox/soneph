import {
  Home as HomeIcon,
  Music,
  Heart,
  FileText,
  Settings2,
  Puzzle,
  Sparkles,
  Users,
  Disc3,
  ListMusic,
} from "lucide-react";
import { HomeView } from "@/components/HomeView";
import { LibraryView } from "@/components/LibraryView";
import { LyricsManagerView } from "@/components/LyricsManagerView";
import { PlaylistView } from "@/components/PlaylistView";
import { SyncSettingsView } from "@/components/SyncSettingsView";
import { MarketplaceView } from "@/components/MarketplaceView";
import { ArtistsView, AlbumsView, CollectionDetailView } from "@/components/CollectionViews";
import type { PluginManifest } from "@/framework/plugin.types";

// The built-in plugin: the library, playback and lyrics shell. Always on.
// Every view is declared with its component and rendered through the host
// with the single `{ app }` prop — App.tsx only routes nav ids to the host.
export const corePlugin: PluginManifest = {
  id: "core",
  nameKey: "Core",
  descKey: "Core Desc",
  version: "1.0.0",
  icon: Sparkles,
  core: true,
  contributes: {
    views: [
      {
        id: "home",
        labelKey: "Home",
        section: "music",
        icon: HomeIcon,
        component: HomeView,
      },
      {
        id: "songs",
        labelKey: "All Music",
        section: "music",
        icon: Music,
        component: LibraryView,
        badge: (app) => app.files.length,
      },
      {
        id: "liked",
        labelKey: "Liked tracks",
        section: "music",
        icon: Heart,
        component: LibraryView,
      },
      {
        id: "artists",
        labelKey: "Artists",
        section: "music",
        icon: Users,
        component: ArtistsView,
      },
      {
        id: "albums",
        labelKey: "Albums",
        section: "music",
        icon: Disc3,
        component: AlbumsView,
      },
      {
        id: "lyrics",
        labelKey: "Lyrics",
        section: "library",
        icon: FileText,
        component: LyricsManagerView,
        badge: (app) => {
          const synced = app.files.filter((f) => f.lyrics_type === "synced").length;
          return `${synced}/${app.files.length}`;
        },
      },
      {
        id: "sync",
        labelKey: "Sync & Settings",
        section: "library",
        icon: Settings2,
        component: SyncSettingsView,
      },
      {
        id: "marketplace",
        labelKey: "Marketplace",
        section: "library",
        icon: Puzzle,
        component: MarketplaceView,
      },

      // ── Dynamic routes (host matches the nav prefix, not in the sidebar) ──
      {
        id: "playlist",
        labelKey: "Playlist",
        section: "playlists",
        icon: ListMusic,
        component: PlaylistView,
        hidden: true,
      },
      {
        id: "collection",
        labelKey: "Collection",
        section: "music",
        icon: Disc3,
        component: CollectionDetailView,
        hidden: true,
      },
    ],
  },
};
