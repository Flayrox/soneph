import React, { useEffect, useState } from "react";
import { Music, Pin, PinOff, Play, ListMusic, Users, Disc3 } from "lucide-react";
import { Glass } from "./Glass";
import type { DownloadedFile } from "@/types";
import type { Pin as PinType } from "@/pins";
import { useI18n } from "@/i18n";

export interface CollectionEntry {
  name: string;
  files: DownloadedFile[];
}

interface CollectionGridProps {
  kind: "artist" | "album";
  entries: CollectionEntry[];
  getApiUrl: () => string;
  onOpen: (name: string) => void;
  isPinned: (pin: PinType) => boolean;
  onTogglePin: (pin: PinType) => void;
  /** Play all tracks of the collection. */
  onPlayAll?: (paths: string[]) => void;
  /** Insert the collection's tracks right after the current one. */
  onPlayNext?: (paths: string[]) => void;
}

const Cover: React.FC<{ file: DownloadedFile; getApiUrl: () => string }> = ({ file, getApiUrl }) => (
  <div className="w-full aspect-square rounded-lg overflow-hidden bg-[#28282c] flex items-center justify-center text-apple-pink relative">
    <Music className="w-8 h-8 opacity-50" />
    <img
      src={`${getApiUrl()}/cover?path=${encodeURIComponent(file.rel_path)}`}
      alt={file.title}
      className="absolute inset-0 w-full h-full object-cover"
      loading="lazy"
      onError={(e) => {
        e.currentTarget.style.display = "none";
      }}
    />
  </div>
);

export const CollectionGrid: React.FC<CollectionGridProps> = ({
  kind,
  entries,
  getApiUrl,
  onOpen,
  isPinned,
  onTogglePin,
  onPlayAll,
  onPlayNext,
}) => {
  const { t } = useI18n();
  // Right-click context menu: { x, y, entry } | null
  const [ctxMenu, setCtxMenu] = useState<{
    x: number;
    y: number;
    entry: CollectionEntry;
  } | null>(null);

  // Close the context menu on outside click, scroll or Escape.
  useEffect(() => {
    if (!ctxMenu) return;
    const close = () => setCtxMenu(null);
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    window.addEventListener("mousedown", close);
    window.addEventListener("keydown", onKey);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("mousedown", close);
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", close, true);
    };
  }, [ctxMenu]);

  if (entries.length === 0) {
    return (
      <Glass cornerRadius={14}>
        <div className="px-5 py-12 text-center">
          <Music className="w-8 h-8 mx-auto mb-2 opacity-40 text-apple-subtext" />
          <p className="text-sm text-zinc-400">
            {kind === "artist" ? t("No artists yet") : t("No albums yet")}
          </p>
          <p className="text-xs text-zinc-500 mt-1">{t("Import music to build your library")}</p>
        </div>
      </Glass>
    );
  }

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3">
      {entries.map((e) => {
        const pin: PinType = { kind, value: e.name };
        const pinned = isPinned(pin);
        return (
          <Glass key={e.name} cornerRadius={14}>
          <div
            onClick={() => onOpen(e.name)}
            onContextMenu={(ev) => {
              ev.preventDefault();
              setCtxMenu({ x: ev.clientX, y: ev.clientY, entry: e });
            }}
            className="group relative p-3 cursor-pointer hover:bg-white/5 transition-colors"
          >
            <Cover file={e.files[0]} getApiUrl={getApiUrl} />
            <p className="text-xs font-semibold text-white truncate mt-2">{e.name}</p>
            <p className="text-[11px] text-apple-subtext truncate">
              {e.files.length} {t("songs")}
            </p>
            <button
              onClick={(ev) => {
                ev.stopPropagation();
                onTogglePin(pin);
              }}
              className={`absolute top-5 right-5 p-1.5 rounded-full transition-all opacity-0 group-hover:opacity-100 ${
                pinned
                  ? "bg-apple-pink text-white"
                  : "bg-black/60 text-white hover:bg-apple-pink"
              }`}
              title={pinned ? t("Unpin") : t("Pin")}
            >
              {pinned ? <PinOff className="w-3.5 h-3.5" /> : <Pin className="w-3.5 h-3.5" />}
            </button>
          </div>
          </Glass>
        );
      })}

      {/* Right-click context menu — glass, same style as the track menu */}
      {ctxMenu && (
        <>
          <div className="fixed inset-0 z-[70]" onMouseDown={() => setCtxMenu(null)} />
          <div
            className="fixed z-[80] w-56"
            style={{
              left: Math.min(ctxMenu.x, window.innerWidth - 236),
              top: Math.min(ctxMenu.y, window.innerHeight - 280),
            }}
          >
            <Glass cornerRadius={12} className="w-full">
              <div className="p-1 text-xs">
                {/* Entry header */}
                <div className="px-2.5 py-1.5 border-b border-white/10 mb-1">
                  <p className="font-bold text-white truncate">{ctxMenu.entry.name}</p>
                  <p className="text-[10px] text-apple-subtext truncate">
                    {kind === "artist" ? t("Artist") : t("Album")} · {ctxMenu.entry.files.length}{" "}
                    {t("songs")}
                  </p>
                </div>

                <MenuItem
                  icon={<Play className="w-3.5 h-3.5" />}
                  label={t("Play All")}
                  onClick={() => {
                    onPlayAll?.(ctxMenu.entry.files.map((f) => f.rel_path));
                    setCtxMenu(null);
                  }}
                />
                {onPlayNext && (
                  <MenuItem
                    icon={<ListMusic className="w-3.5 h-3.5" />}
                    label={t("Play Next")}
                    onClick={() => {
                      onPlayNext(ctxMenu.entry.files.map((f) => f.rel_path));
                      setCtxMenu(null);
                    }}
                  />
                )}
                <MenuItem
                  icon={
                    isPinned({ kind, value: ctxMenu.entry.name }) ? (
                      <PinOff className="w-3.5 h-3.5" />
                    ) : (
                      <Pin className="w-3.5 h-3.5" />
                    )
                  }
                  label={
                    isPinned({ kind, value: ctxMenu.entry.name }) ? t("Unpin") : t("Pin")
                  }
                  onClick={() => {
                    onTogglePin({ kind, value: ctxMenu.entry.name });
                    setCtxMenu(null);
                  }}
                />
                <MenuItem
                  icon={
                    kind === "artist" ? (
                      <Users className="w-3.5 h-3.5" />
                    ) : (
                      <Disc3 className="w-3.5 h-3.5" />
                    )
                  }
                  label={kind === "artist" ? t("View Artist") : t("View Album")}
                  onClick={() => {
                    onOpen(ctxMenu.entry.name);
                    setCtxMenu(null);
                  }}
                />
              </div>
            </Glass>
          </div>
        </>
      )}
    </div>
  );
};

/** One row of the right-click context menu. */
const MenuItem: React.FC<{
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
}> = ({ icon, label, onClick }) => (
  <button
    onMouseDown={(e) => e.stopPropagation()}
    onClick={(e) => {
      e.stopPropagation();
      onClick();
    }}
    className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-left text-zinc-200 hover:bg-white/10 transition-colors"
  >
    <span className="text-apple-subtext shrink-0">{icon}</span>
    <span className="truncate">{label}</span>
  </button>
);
