import React, { useRef, useState } from "react";
import { Glass } from "./Glass";
import { Search, Plus, X, Check, FolderCheck, ListMusic, Users, Disc3, PinOff, Play, XCircle } from "lucide-react";
import { cleanTitle } from "@/format";
import type { DownloadedFile, PlaylistSummary } from "@/types";
import type {
  PluginApp,
  PluginNavSection,
  PluginViewContribution,
} from "@/framework/plugin.types";
import { pluginForView, pluginViews } from "@/framework/pluginRegistry";
import { usePlugins } from "@/framework/PluginProvider";
import type { Pin } from "@/pins";
import { useI18n } from "@/i18n";

interface SidebarProps {
  app: PluginApp;
  /** Which side of the window the sidebar sits on (flippable). */
  side?: "left" | "right";
  /** Pinned artists & albums (shown as their own section). */
  pins?: Pin[];
  onTogglePin?: (pin: Pin) => void;
  activeFilter: string;
  onFilterChange: (filter: string) => void;
  activeNav: string;
  onNavChange: (nav: string) => void;
  playlists?: PlaylistSummary[];
  onCreatePlaylist?: (name: string) => void;
  /** Playback queue — shown as a compact section. */
  queueTracks?: DownloadedFile[];
  currentIndex?: number;
  onPlayQueueIndex?: (index: number) => void;
  onRemoveFromQueue?: (index: number) => void;
}

// Sidebar sections, in display order. Every nav entry comes from the plugin
// registry — the host only renders the playlists section (dynamic data).
const SECTION_ORDER: { key: PluginNavSection; labelKey: string }[] = [
  { key: "music", labelKey: "Music" },
  { key: "playlists", labelKey: "Playlists" },
  { key: "downloads", labelKey: "Downloads" },
  { key: "library", labelKey: "Library" },
];

const NavButton: React.FC<{
  view: PluginViewContribution;
  label: string;
  active: boolean;
  highlight?: boolean;
  badge?: string | number | null;
  onClick: () => void;
}> = ({ view, label, active, highlight = false, badge = null, onClick }) => {
  const Icon = view.icon;
  const state = active
    ? "bg-apple-pink text-white font-semibold shadow-sm"
    : highlight
    ? "bg-apple-pink/15 text-apple-pink font-semibold border border-apple-pink/30 animate-pulse"
    : "text-zinc-300 hover:bg-white/5";
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${state}`}
    >
      <div className="flex items-center gap-2.5 min-w-0">
        <Icon className={`w-4 h-4 shrink-0 ${highlight ? "animate-bounce" : ""}`} />
        <span className="truncate">{label}</span>
      </div>
      {badge != null && (
        <span
          className={`text-[10px] px-2 py-0.5 rounded-full font-semibold shrink-0 ${
            active ? "bg-white/20 text-white" : "bg-white/10 text-zinc-300"
          }`}
        >
          {badge}
        </span>
      )}
    </button>
  );
};

export const Sidebar: React.FC<SidebarProps> = ({
  app,
  side = "left",
  pins = [],
  onTogglePin,
  activeFilter,
  onFilterChange,
  activeNav,
  onNavChange,
  playlists = [],
  onCreatePlaylist,
  queueTracks = [],
  currentIndex = -1,
  onPlayQueueIndex,
  onRemoveFromQueue,
}) => {
  const { t } = useI18n();
  const { isEnabled } = usePlugins();
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");

  // ── Resizable sidebar ────────────────────────────────────────────────
  // Drag the right edge to resize (persisted), double-click to reset.
  const SIDEBAR_MIN = 180;
  const SIDEBAR_MAX = 420;
  const SIDEBAR_DEFAULT = 256;
  const [width, setWidth] = useState<number>(() => {
    if (typeof window === "undefined") return SIDEBAR_DEFAULT;
    const v = parseInt(window.localStorage.getItem("soneph_sidebar_w") || "", 10);
    return Number.isFinite(v) && v >= SIDEBAR_MIN && v <= SIDEBAR_MAX ? v : SIDEBAR_DEFAULT;
  });
  const resizeRef = useRef<{ startX: number; startW: number } | null>(null);

  const persistWidth = (w: number) => {
    try {
      window.localStorage.setItem("soneph_sidebar_w", String(Math.round(w)));
    } catch {
      // storage unavailable — width stays in-memory
    }
  };

  const onResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    resizeRef.current = { startX: e.clientX, startW: width };
    const onMove = (ev: MouseEvent) => {
      if (!resizeRef.current) return;
      const delta =
        side === "left"
          ? ev.clientX - resizeRef.current.startX
          : resizeRef.current.startX - ev.clientX;
      const next = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, resizeRef.current.startW + delta));
      setWidth(next);
      persistWidth(next);
    };
    const onUp = () => {
      resizeRef.current = null;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const resetWidth = () => {
    setWidth(SIDEBAR_DEFAULT);
    persistWidth(SIDEBAR_DEFAULT);
  };

  // Views visible in the sidebar: core views are always shown, plugin views
  // only while their plugin is enabled. Hidden views (dynamic routes like
  // playlist/collection details) are never listed here.
  const views = pluginViews().filter((v) => {
    if (v.hidden) return false;
    const plugin = pluginForView(v.id);
    return plugin ? plugin.core || isEnabled(plugin.id) : false;
  });

  const badgeFor = (v: PluginViewContribution) => (v.badge ? v.badge(app) ?? null : null);

  const highlightFor = (v: PluginViewContribution) =>
    v.id === "downloads" && badgeFor(v) != null && activeNav !== v.id;

  const submitNewPlaylist = (e: React.FormEvent) => {
    e.preventDefault();
    if (newName.trim() && onCreatePlaylist) {
      onCreatePlaylist(newName.trim());
      setNewName("");
      setCreating(false);
    }
  };

  return (
    <aside
      className={`bg-[#1e1e20]/80 backdrop-blur-2xl h-full flex flex-col justify-between p-3 select-none relative z-10 shrink-0 ${
        side === "left" ? "border-r border-white/10" : "border-l border-white/10"
      }`}
      style={{ width }}
    >
      <div className="space-y-4 overflow-y-auto scrollbar-none pr-1">
        {/* Search Input Bar */}
        <div className="relative px-1 pt-1">
          <Search className="w-4 h-4 absolute left-3.5 top-3 text-apple-subtext" />
          <Glass cornerRadius={999} className="rounded-full">
            <input
              type="text"
              value={activeFilter}
              onChange={(e) => onFilterChange(e.target.value)}
              placeholder={t("Search")}
              className="w-full bg-transparent rounded-full py-1.5 pl-9 pr-3 text-xs text-white placeholder-apple-subtext focus:outline-none transition-all"
            />
          </Glass>
        </div>

        {SECTION_ORDER.map(({ key, labelKey }) => {
          // Playlists is host-managed (dynamic data), not registry-driven.
          if (key === "playlists") {
            return (
              <div key={key} className="space-y-1">
                <div className="flex items-center justify-between px-3">
                  <span className="text-[11px] font-semibold text-apple-subtext uppercase tracking-wider">
                    {t(labelKey)}
                  </span>
                  <button
                    onClick={() => {
                      setCreating((c) => !c);
                      setNewName("");
                    }}
                    className="p-0.5 rounded text-apple-subtext hover:text-white transition-colors"
                    title={t("New Playlist")}
                  >
                    {creating ? <X className="w-3.5 h-3.5" /> : <Plus className="w-3.5 h-3.5" />}
                  </button>
                </div>

                {creating && (
                  <form onSubmit={submitNewPlaylist} className="px-2">
                    <input
                      autoFocus
                      value={newName}
                      onChange={(e) => setNewName(e.target.value)}
                      placeholder={t("Playlist Name")}
                      className="w-full bg-[#2a2a2d] border border-white/10 focus:border-apple-pink rounded-lg px-2 py-1.5 text-xs text-white placeholder-apple-subtext focus:outline-none"
                    />
                  </form>
                )}

                <div className="space-y-0.5 text-xs font-medium">
                  {playlists.map((p) => {
                    const isActive = activeNav === `pl:${p.id}`;
                    return (
                      <button
                        key={p.id}
                        onClick={() => onNavChange(`pl:${p.id}`)}
                        className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md transition-colors ${
                          isActive
                            ? "bg-apple-pink text-white font-semibold shadow-sm"
                            : "text-zinc-300 hover:bg-white/5"
                        }`}
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <ListMusic className="w-4 h-4 shrink-0" />
                          <span className="truncate">{p.name}</span>
                        </div>
                        <span
                          className={`text-[10px] px-2 py-0.5 rounded-full font-semibold shrink-0 ${
                            isActive ? "bg-white/20 text-white" : "bg-white/10 text-zinc-300"
                          }`}
                        >
                          {p.track_count}
                        </span>
                      </button>
                    );
                  })}
                  {playlists.length === 0 && !creating && (
                    <div className="px-3 py-1 text-[11px] text-zinc-500">{t("No playlists yet")}</div>
                  )}
                </div>
              </div>
            );
          }

          const sectionViews = views.filter((v) => v.section === key);
          if (sectionViews.length === 0 && key !== "music") return null;

          const sectionEl = (
            <div key={key} className="space-y-1">
              <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
                {t(labelKey)}
              </span>
              <div className="space-y-0.5 text-xs font-medium">
                {sectionViews.map((v) => (
                  <NavButton
                    key={v.id}
                    view={v}
                    label={t(v.labelKey)}
                    active={activeNav === v.id}
                    highlight={highlightFor(v)}
                    badge={badgeFor(v)}
                    onClick={() => onNavChange(v.id)}
                  />
                ))}
              </div>
            </div>
          );

          return (
            <React.Fragment key={key}>
              {sectionEl}

              {/* Playback queue — compact, below Music & Pins */}
              {key === "music" && queueTracks.length > 0 && (
                <div className="space-y-1">
                  <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
                    {t("Queue")} · {queueTracks.length}
                  </span>
                  <div className="space-y-0.5 text-xs font-medium max-h-40 overflow-y-auto scrollbar-none">
                    {queueTracks.map((track, idx) => {
                      const isCurrent = idx === currentIndex;
                      return (
                        <div key={`${track.rel_path}_${idx}`} className="group flex items-center pr-1">
                          <button
                            onClick={() => onPlayQueueIndex?.(idx)}
                            className={`flex-1 min-w-0 flex items-center gap-2.5 px-3 py-1.5 rounded-l-md transition-colors ${
                              isCurrent
                                ? "text-apple-pink"
                                : "text-zinc-300 hover:bg-white/5"
                            }`}
                            title={isCurrent ? t("Now Playing") : track.title}
                          >
                            <span
                              className={`w-3.5 h-3.5 shrink-0 flex items-center justify-center ${
                                isCurrent ? "text-apple-pink" : "text-apple-subtext"
                              }`}
                            >
                              {isCurrent ? (
                                <Play className="w-3 h-3 fill-current" />
                              ) : (
                                <span className="text-[9px]">{idx + 1}</span>
                              )}
                            </span>
                            <span className={`truncate ${isCurrent ? "font-semibold" : ""}`}>
                              {cleanTitle(track.title)}
                            </span>
                          </button>
                          <button
                            onClick={() => onRemoveFromQueue?.(idx)}
                            className="p-1 rounded-r-md text-zinc-500 opacity-0 group-hover:opacity-100 hover:text-rose-400 transition-all"
                            title={t("Remove from queue")}
                          >
                            <XCircle className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* Pins — right below Music, above Playlists */}
              {key === "music" && pins.length > 0 && (
                <div className="space-y-1">
                  <span className="text-[11px] font-semibold text-apple-subtext px-3 uppercase tracking-wider">
                    {t("Pins")}
                  </span>
                  <div className="space-y-0.5 text-xs font-medium">
                    {pins.map((pin) => {
                      const isPlaylist = pin.kind === "playlist";
                      const name = isPlaylist
                        ? playlists.find((p) => p.id === pin.value)?.name ?? pin.value
                        : pin.value;
                      const target = isPlaylist
                        ? `pl:${pin.value}`
                        : `${pin.kind}:${encodeURIComponent(pin.value)}`;
                      const isActive = activeNav === target;
                      const Icon = isPlaylist ? ListMusic : pin.kind === "artist" ? Users : Disc3;
                      return (
                        <div key={`${pin.kind}:${pin.value}`} className="group flex items-center">
                          <button
                            onClick={() => onNavChange(target)}
                            className={`flex-1 min-w-0 flex items-center gap-2.5 px-3 py-1.5 rounded-l-md transition-colors ${
                              isActive
                                ? "bg-apple-pink text-white font-semibold shadow-sm"
                                : "text-zinc-300 hover:bg-white/5"
                            }`}
                          >
                            <Icon className="w-4 h-4 shrink-0" />
                            <span className="truncate">{name}</span>
                          </button>
                          <button
                            onClick={() => onTogglePin?.(pin)}
                            className="p-1.5 rounded-r-md text-zinc-500 opacity-0 group-hover:opacity-100 hover:text-rose-400 transition-all"
                            title={t("Unpin")}
                          >
                            <PinOff className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </React.Fragment>
          );
        })}
      </div>

      {/* Syncthing P2P Status Footer */}
      <div className="pt-3 border-t border-white/10 px-2 flex items-center justify-between text-xs text-apple-subtext">
        <div className="flex items-center gap-2">
          <FolderCheck className="w-4 h-4 text-apple-pink" />
          <span>{t("Auto-Sync")}</span>
        </div>        <Glass cornerRadius={999} className="w-fit">
          <span className="flex items-center gap-1 text-[10px] bg-apple-pink/20 text-apple-pink px-2 py-0.5 rounded-full font-semibold">
            <Check className="w-3 h-3 inline" /> {t("Synced")}
          </span>
        </Glass>
      </div>

      {/* Drag handle — resize the sidebar */}
      <div
        onMouseDown={onResizeStart}
        onDoubleClick={resetWidth}
        className={`absolute top-0 bottom-0 w-1.5 cursor-col-resize z-20 group/handle ${
          side === "left" ? "right-0" : "left-0"
        }`}
        title={t("Drag to resize — double-click to reset")}
      >
        <div className="w-px h-full mx-auto bg-transparent group-hover/handle:bg-apple-pink/70 transition-colors" />
      </div>
    </aside>
  );
};
