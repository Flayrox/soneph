import React from "react";
import LiquidGlass from "liquid-glass-react";
import { Play, Heart, Music, Clock, TrendingUp, Sparkles } from "lucide-react";
import type { DownloadedFile } from "@/types";
import { useI18n } from "@/i18n";

export interface TopEntry {
  file: DownloadedFile;
  plays: number;
}

interface HomeViewProps {
  files: DownloadedFile[];
  recent: DownloadedFile[];
  top: TopEntry[];
  liked: DownloadedFile[];
  likes: Set<string>;
  totalPlays: number;
  currentPlayingPath: string | null;
  isPlaying: boolean;
  onPlayList: (paths: string[], index: number) => void;
  onToggleLike: (path: string) => void;
  onNavChange: (nav: string) => void;
  getApiUrl: () => string;
}

const Cover: React.FC<{ file: DownloadedFile; getApiUrl: () => string }> = ({ file, getApiUrl }) => (
  <div className="w-9 h-9 rounded-md bg-[#28282c] border border-white/10 flex items-center justify-center text-apple-pink shrink-0 overflow-hidden shadow-sm relative">
    <Music className="w-4 h-4 absolute inset-0 m-auto opacity-60" />
    <img
      src={`${getApiUrl()}/cover?path=${encodeURIComponent(file.rel_path)}`}
      alt={file.title}
      className="w-full h-full object-cover relative z-10"
      loading="lazy"
      onError={(e) => {
        e.currentTarget.style.display = "none";
      }}
    />
  </div>
);

const LikeButton: React.FC<{
  liked: boolean;
  onClick: () => void;
  className?: string;
}> = ({ liked, onClick, className = "" }) => (
  <button
    onClick={(e) => {
      e.stopPropagation();
      onClick();
    }}
    className={`p-1.5 rounded-full transition-all ${className}`}
    title={liked ? "Unlike" : "Like"}
  >
    <Heart
      className={`w-3.5 h-3.5 ${liked ? "text-apple-pink fill-apple-pink" : "text-zinc-500 hover:text-apple-pink"}`}
    />
  </button>
);

export const HomeView: React.FC<HomeViewProps> = ({
  files,
  recent,
  top,
  liked,
  likes,
  totalPlays,
  currentPlayingPath,
  isPlaying,
  onPlayList,
  onToggleLike,
  onNavChange,
  getApiUrl,
}) => {
  const { t } = useI18n();

  const playPathIn = (list: DownloadedFile[], path: string) => {
    const idx = list.findIndex((f) => f.rel_path === path);
    if (idx >= 0) onPlayList(list.map((f) => f.rel_path), idx);
  };

  const stats = [
    { label: t("Total tracks"), value: files.length, icon: Music },
    { label: t("Liked tracks"), value: liked.length, icon: Heart },
    { label: t("Recent listens"), value: recent.length, icon: Clock },
  ];

  return (
    <div className="w-full text-zinc-200 select-none font-sans p-6 space-y-8">
      {/* Greeting */}
      <div>
        <div className="flex items-center gap-2 text-apple-pink">
          <Sparkles className="w-4 h-4" />
          <span className="text-[11px] font-semibold uppercase tracking-wider">
            {t("Welcome back")}
          </span>
        </div>
        <h2 className="text-2xl font-bold text-white mt-1">{t("Your library at a glance")}</h2>

        {/* Stats */}
        <div className="flex flex-wrap gap-3 mt-4">
          {stats.map((s) => (
            <LiquidGlass key={s.label} cornerRadius={14} padding="0px" blurAmount={0.02} displacementScale={20}>
              <div className="flex items-center gap-2.5 bg-white/5 rounded-xl px-4 py-2.5">
                <s.icon className="w-4 h-4 text-apple-pink" />
                <div>
                  <div className="text-base font-bold text-white leading-none">{s.value}</div>
                  <div className="text-[10px] text-apple-subtext mt-0.5">{s.label}</div>
                </div>
              </div>
            </LiquidGlass>
          ))}
        </div>
      </div>

      {/* Recent listens */}
      <section>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-bold text-white uppercase tracking-wider">
            {t("Recent listens")}
          </h3>
        </div>
        {recent.length === 0 ? (
          <div className="bg-white/5 border border-white/10 rounded-xl px-5 py-8 text-center">
            <Clock className="w-8 h-8 mx-auto mb-2 opacity-40 text-apple-subtext" />
            <p className="text-sm text-zinc-400">{t("Play something to start building your history")}</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3">
            {recent.slice(0, 12).map((f) => {
              const isPlayingThis = currentPlayingPath === f.rel_path && isPlaying;
              return (
                <div
                  key={f.rel_path}
                  onClick={() => playPathIn(recent, f.rel_path)}
                  className="group bg-white/5 hover:bg-white/10 border border-white/10 rounded-xl p-3 cursor-pointer transition-all"
                >
                  <div className="relative w-full aspect-square rounded-lg overflow-hidden bg-[#28282c] flex items-center justify-center text-apple-pink mb-2">
                    <Music className="w-8 h-8 opacity-50" />
                    <img
                      src={`${getApiUrl()}/cover?path=${encodeURIComponent(f.rel_path)}`}
                      alt={f.title}
                      className="absolute inset-0 w-full h-full object-cover"
                      loading="lazy"
                      onError={(e) => {
                        e.currentTarget.style.display = "none";
                      }}
                    />
                    {isPlayingThis && (
                      <div className="absolute inset-0 bg-black/40 flex items-center justify-center">
                        <div className="flex items-end justify-center gap-0.5 h-5">
                          <span className="w-1 h-full bg-apple-pink animate-bounce rounded-sm" />
                          <span className="w-1 h-2/3 bg-apple-pink animate-bounce delay-75 rounded-sm" />
                          <span className="w-1 h-4/5 bg-apple-pink animate-bounce delay-150 rounded-sm" />
                        </div>
                      </div>
                    )}
                  </div>
                  <p className="text-xs font-semibold text-white truncate">{f.title}</p>
                  <p className="text-[11px] text-apple-subtext truncate">{f.artist}</p>
                  <div className="flex items-center justify-between mt-1.5">
                    <span className="text-[10px] text-zinc-500">{t("Recently played")}</span>
                    <LikeButton liked={likes.has(f.rel_path)} onClick={() => onToggleLike(f.rel_path)} />
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {/* Top tracks */}
      {top.length > 0 && (
        <section>
          <div className="flex items-center gap-2 mb-3">
            <TrendingUp className="w-4 h-4 text-apple-pink" />
            <h3 className="text-sm font-bold text-white uppercase tracking-wider">
              {t("Top tracks")}
            </h3>
          </div>
          <div className="space-y-1">
            {top.slice(0, 10).map((entry, i) => (
              <div
                key={entry.file.rel_path}
                onClick={() => playPathIn(top.map((e) => e.file), entry.file.rel_path)}
                className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-white/5 cursor-pointer transition-colors"
              >
                <span className="w-6 text-center text-sm font-bold text-apple-subtext">{i + 1}</span>
                <Cover file={entry.file} getApiUrl={getApiUrl} />
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-semibold text-white truncate">{entry.file.title}</p>
                  <p className="text-[11px] text-apple-subtext truncate">{entry.file.artist}</p>
                </div>
                <span className="text-[10px] text-zinc-500 shrink-0">
                  {entry.plays} {entry.plays > 1 ? t("plays") : t("times")}
                </span>
                <LikeButton liked={likes.has(entry.file.rel_path)} onClick={() => onToggleLike(entry.file.rel_path)} />
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Liked tracks */}
      <section>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Heart className="w-4 h-4 text-apple-pink" />
            <h3 className="text-sm font-bold text-white uppercase tracking-wider">
              {t("Liked tracks")}
            </h3>
          </div>
          <button
            onClick={() => onNavChange("liked")}
            className="text-[11px] font-semibold text-apple-pink hover:text-white transition-colors"
          >
            {t("All Music")} →
          </button>
        </div>
        {liked.length === 0 ? (
          <div className="bg-white/5 border border-white/10 rounded-xl px-5 py-8 text-center">
            <Heart className="w-8 h-8 mx-auto mb-2 opacity-40 text-apple-subtext" />
            <p className="text-sm text-zinc-400">{t("No likes yet — tap the heart on a track")}</p>
          </div>
        ) : (
          <div className="space-y-1">
            {liked.slice(0, 8).map((f) => (
              <div
                key={f.rel_path}
                onClick={() => playPathIn(liked, f.rel_path)}
                className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-white/5 cursor-pointer transition-colors"
              >
                <Cover file={f} getApiUrl={getApiUrl} />
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-semibold text-white truncate">{f.title}</p>
                  <p className="text-[11px] text-apple-subtext truncate">{f.artist}</p>
                </div>
                <LikeButton liked onClick={() => onToggleLike(f.rel_path)} />
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
};
