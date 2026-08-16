import type { ComponentType } from "react";
import type { LucideIcon } from "lucide-react";
import type { DownloadedFile, DownloadTask, Playlist, PlaylistSummary } from "@/types";

// ── The plugin contract ──────────────────────────────────────────────────
// A plugin is a self-contained, optional feature. It contributes views
// (sidebar entries + panels) and actions to the host application. The host
// (App.tsx) owns all shared state and hands every contributed view a single
// `app` prop — the `PluginApp` context object described below.
//
// Core plugins (`core: true`) are built into the app and can't be disabled;
// every other plugin is toggleable from the Marketplace / onboarding picker.

export type PluginNavSection = "music" | "playlists" | "downloads" | "library";

export interface PluginViewContribution {
  /** Routing / nav id — the same string the host uses to switch views. */
  id: string;
  /** i18n key for the sidebar label. */
  labelKey: string;
  /** Sidebar section this view is listed under. */
  section: PluginNavSection;
  icon: LucideIcon;
  /**
   * Component rendered through the host for this view. Present on plugin
   * views (which receive `{ app }`); core views are rendered directly by
   * the host with its own data, so they only declare the nav entry here.
   */
  component?: ComponentType<PluginViewProps>;
  /** Optional count rendered next to the nav entry (e.g. track counts). */
  badge?: (app: PluginApp) => string | number | null;
}

export interface PluginActionContribution {
  id: string;
  labelKey?: string;
  run: (app: PluginApp) => void;
}

export interface PluginManifest {
  id: string;
  nameKey: string;
  descKey: string;
  version: string;
  icon: LucideIcon;
  /** Built-in plugins can't be disabled. */
  core?: boolean;
  /** Pre-selected during onboarding when the user hasn't configured anything yet. */
  defaultEnabled?: boolean;
  contributes: {
    views?: PluginViewContribution[];
    actions?: PluginActionContribution[];
  };
}

/** Props handed to every contributed view — the only prop it receives. */
export interface PluginViewProps {
  app: PluginApp;
}

/** Shared capabilities the host exposes to plugins. */
export interface PluginApp {
  nav: string;
  setNav: (nav: string) => void;
  files: DownloadedFile[];
  tasks: DownloadTask[];
  playlists: PlaylistSummary[];
  playlistDetail: Playlist | null;
  likes: Set<string>;
  toggleLike: (path: string) => void;
  playTrack: (path: string) => void;
  playList: (paths: string[], index: number) => void;
  /** Insert tracks right after the current one ("Play next"). */
  playNext?: (paths: string[]) => void;
  getApiUrl: () => string;
  notify: (type: "success" | "error" | "info", title: string, message: string) => void;
  refreshFiles: () => void;
  currentPlayingPath: string | null;
  isPlaying: boolean;
}
