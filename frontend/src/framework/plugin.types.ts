import type { ComponentType } from "react";
import type { LucideIcon } from "lucide-react";
import type { DownloadedFile, DownloadTask, Playlist, PlaylistSummary } from "@/types";
import type { Pin } from "@/pins";

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
   * views (which receive `{ app }`); the host renders it via PluginHostView.
   */
  component?: ComponentType<PluginViewProps>;
  /** Optional count rendered next to the nav entry (e.g. track counts). */
  badge?: (app: PluginApp) => string | number | null;
  /**
   * Dynamic routes (playlist/artist/album details) are resolved by the host
   * but never listed in the sidebar — the host matches the nav prefix and
   * routes to the view id.
   */
  hidden?: boolean;
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

/** One row of the Home "Top tracks" list. */
export interface HomeTopEntry {
  file: DownloadedFile;
  plays: number;
}

/** A pinned artist / album / playlist card shown on Home. */
export interface HomePinnedEntry {
  kind: "artist" | "album" | "playlist";
  name: string;
  files: DownloadedFile[];
  /** Playlist id (only for pinned playlists). */
  id?: string;
  trackCount?: number;
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

  // ── Derived library data (computed by the host) ──────────────────────
  /** Files after the sidebar search filter. */
  filteredFiles: DownloadedFile[];
  /** Liked files (resolved against the library). */
  likedFiles: DownloadedFile[];
  /** Recently played files (resolved, deduplicated). */
  recent: DownloadedFile[];
  /** Most played tracks. */
  top: HomeTopEntry[];
  /** Total number of recorded plays. */
  totalPlays: number;
  /** Pinned artists & albums & playlists, resolved to cards. */
  pinned: HomePinnedEntry[];
  /** Artists grouped from the library (name → tracks), by track count. */
  artists: { name: string; files: DownloadedFile[] }[];
  /** Albums grouped from the library (name → tracks), by track count. */
  albums: { name: string; files: DownloadedFile[] }[];

  // ── Pins ─────────────────────────────────────────────────────────────
  isPinned: (pin: Pin) => boolean;
  togglePin: (pin: Pin) => void;

  // ── Lyrics drawer ────────────────────────────────────────────────────
  /** Open the right-side lyrics drawer for a track. */
  openLyricsDrawer: (track: DownloadedFile) => void;

  // ── File & playlist operations ───────────────────────────────────────
  deleteFile: (path: string) => void;
  addToPlaylist: (playlistId: string, path: string) => void;
  /** Create a playlist and, if a track is given, add it right away. */
  createPlaylist: (name: string, path?: string) => void;
  removeFromPlaylist: (playlistId: string, path: string) => void;
  reorderPlaylist: (id: string, path: string, toIndex: number) => void;
  deletePlaylist: (id: string) => void;
  /** Navigate to a playlist (loads its detail). */
  openPlaylist: (id: string) => void;
}
