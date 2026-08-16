import React, { useEffect, useState } from "react";
import { Clock, Play, TrendingUp, User, Music, BarChart3, Heart } from "lucide-react";
import type { DownloadedFile } from "@/types";
import { apiFetch } from "@/api";
import { useI18n } from "@/i18n";

interface StatsData {
  total_plays: number;
  total_seconds: number;
  top_artists: { artist: string; plays: number }[];
  top_tracks: { path: string; plays: number }[];
  plays_by_day: { day: string; plays: number }[];
}

interface StatsViewProps {
  files: DownloadedFile[];
  likes: Set<string>;
  onToggleLike: (path: string) => void;
  onPlayTrack: (path: string) => void;
  getApiUrl: () => string;
}

const formatDuration = (seconds: number) => {
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  if (h > 0) return `${h} h ${m > 0 ? `${m} min` : ""}`;
  return `${m} min`;
};

export const StatsView: React.FC<StatsViewProps> = ({ files, likes, onToggleLike, onPlayTrack, getApiUrl }) => {
  const { t } = useI18n();
  const [stats, setStats] = useState<StatsData | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch("/api/stats")
      .then((r) => r.json())
      .then((data) => {
        if (!cancelled) setStats(data);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  if (!stats) {
    return (
      <div className="flex items-center justify-center h-full text-apple-subtext select-none">
        <BarChart3 className="w-8 h-8 animate-pulse opacity-40" />
      </div>
    );
  }

  const byPath = new Map(files.map((f) => [f.rel_path, f]));
  const maxDay = Math.max(1, ...stats.plays_by_day.map((d) => d.plays));

  const cards = [
    { label: t("Total plays"), value: String(stats.total_plays), icon: Play },
    { label: t("Listening time"), value: formatDuration(stats.total_seconds), icon: Clock },
    {
      label: t("Top artist"),
      value: stats.top_artists[0]?.artist || "—",
      icon: User,
    },
  ];

  return (
    <div className="w-full text-zinc-200 select-none font-sans p-6 space-y-8 max-w-4xl">
      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {cards.map((c) => (
          <div
            key={c.label}
            className="bg-white/5 border border-white/10 rounded-2xl px-4 py-4 flex items-center gap-3"
          >
            <div className="w-10 h-10 rounded-xl bg-apple-pink/15 border border-apple-pink/30 flex items-center justify-center text-apple-pink shrink-0">
              <c.icon className="w-4.5 h-4.5" />
            </div>
            <div className="min-w-0">
              <div className="text-lg font-bold text-white leading-tight truncate">{c.value}</div>
              <div className="text-[11px] text-apple-subtext">{c.label}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Plays per day chart */}
      {stats.plays_by_day.some((d) => d.plays > 0) && (
        <section>
          <h3 className="text-sm font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
            <TrendingUp className="w-4 h-4 text-apple-pink" />
            {t("Plays per day")}
          </h3>
          <div className="flex items-end gap-1 h-28">
            {stats.plays_by_day.map((d) => (
              <div key={d.day} className="flex-1 flex flex-col items-center gap-1 group" title={`${d.day} — ${d.plays}`}>
                <span className="text-[9px] text-apple-subtext opacity-0 group-hover:opacity-100 transition-opacity">
                  {d.plays}
                </span>
                <div
                  className="w-full rounded-t-md bg-gradient-to-t from-apple-pink/70 to-apple-pink transition-all"
                  style={{ height: `${Math.max((d.plays / maxDay) * 100, d.plays > 0 ? 8 : 2)}%` }}
                />
                <span className="text-[8px] text-zinc-600">
                  {d.day.slice(8)}
                </span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Top artists */}
      {stats.top_artists.length > 0 && (
        <section>
          <h3 className="text-sm font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
            <User className="w-4 h-4 text-apple-pink" />
            {t("Top artists")}
          </h3>
          <div className="space-y-1">
            {stats.top_artists.map((a, i) => (
              <div key={a.artist} className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-white/5 transition-colors">
                <span className="w-6 text-center text-sm font-bold text-apple-subtext">{i + 1}</span>
                <div className="w-9 h-9 rounded-md bg-apple-pink/15 border border-apple-pink/20 flex items-center justify-center text-apple-pink shrink-0">
                  <Music className="w-4 h-4" />
                </div>
                <span className="text-xs font-semibold text-white truncate flex-1">{a.artist}</span>
                <span className="text-[10px] text-zinc-500 shrink-0">
                  {a.plays} {a.plays > 1 ? t("plays") : t("times")}
                </span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Top tracks */}
      {stats.top_tracks.length > 0 && (
        <section>
          <h3 className="text-sm font-bold text-white uppercase tracking-wider mb-3 flex items-center gap-2">
            <BarChart3 className="w-4 h-4 text-apple-pink" />
            {t("Top tracks")}
          </h3>
          <div className="space-y-1">
            {stats.top_tracks.map((tt, i) => {
              const file = byPath.get(tt.path);
              return (
                <div
                  key={tt.path}
                  onClick={() => file && onPlayTrack(tt.path)}
                  className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-white/5 cursor-pointer transition-colors"
                >
                  <span className="w-6 text-center text-sm font-bold text-apple-subtext">{i + 1}</span>
                  <div className="w-9 h-9 rounded-md bg-[#28282c] border border-white/10 flex items-center justify-center text-apple-pink shrink-0 overflow-hidden relative">
                    <Music className="w-4 h-4 absolute inset-0 m-auto opacity-60" />
                    {file && (
                      <img
                        src={`${getApiUrl()}/cover?path=${encodeURIComponent(tt.path)}`}
                        alt=""
                        className="w-full h-full object-cover relative z-10"
                        loading="lazy"
                        onError={(e) => {
                          e.currentTarget.style.display = "none";
                        }}
                      />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-semibold text-white truncate">{file?.title || tt.path}</p>
                    <p className="text-[11px] text-apple-subtext truncate">{file?.artist || ""}</p>
                  </div>
                  <span className="text-[10px] text-zinc-500 shrink-0">
                    {tt.plays} {tt.plays > 1 ? t("plays") : t("times")}
                  </span>
                  {file && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onToggleLike(tt.path);
                      }}
                      className="p-1.5 rounded-full transition-all"
                      title={likes.has(tt.path) ? "Unlike" : "Like"}
                    >
                      <Heart
                        className={`w-3.5 h-3.5 ${
                          likes.has(tt.path) ? "text-apple-pink fill-apple-pink" : "text-zinc-500 hover:text-apple-pink"
                        }`}
                      />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </section>
      )}

      {stats.total_plays === 0 && (
        <div className="bg-white/5 border border-white/10 rounded-xl px-5 py-10 text-center">
          <BarChart3 className="w-8 h-8 mx-auto mb-2 opacity-40 text-apple-subtext" />
          <p className="text-sm text-zinc-400">{t("No stats yet")}</p>
          <p className="text-xs text-zinc-500 mt-1">{t("Play something to fill in your stats")}</p>
        </div>
      )}
    </div>
  );
};
