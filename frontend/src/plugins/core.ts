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
} from "lucide-react";
import { SyncSettingsView } from "@/components/SyncSettingsView";
import type { PluginManifest } from "@/framework/plugin.types";

// The built-in plugin: the library, playback and lyrics shell. Always on.
// Views migrated to the `{ app }` contract declare their component here
// and are rendered through the host; the others are still rendered
// directly by App.tsx with host-owned data.
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
      },
      {
        id: "songs",
        labelKey: "All Music",
        section: "music",
        icon: Music,
        badge: (app) => app.files.length,
      },
      {
        id: "liked",
        labelKey: "Liked tracks",
        section: "music",
        icon: Heart,
      },
      {
        id: "artists",
        labelKey: "Artists",
        section: "music",
        icon: Users,
      },
      {
        id: "albums",
        labelKey: "Albums",
        section: "music",
        icon: Disc3,
      },
      {
        id: "lyrics",
        labelKey: "Lyrics",
        section: "library",
        icon: FileText,
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
      },
    ],
  },
};
