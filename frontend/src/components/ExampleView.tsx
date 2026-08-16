import React, { useState } from "react";
import { Play, Heart, ListMusic, Bell, Sparkles, ArrowRight, FileText } from "lucide-react";
import { Glass } from "./Glass";
import { useI18n } from "@/i18n";
import { cleanTitle } from "@/format";
import type { PluginViewProps } from "@/framework/plugin.types";

/**
 * Example plugin view — the minimal template for contributing a view.
 *
 * A plugin view receives ONE prop: `app` (the PluginApp contract). From it
 * you can read the library (app.files, app.playlists, app.likes…) and call
 * host actions (app.playTrack, app.playNext, app.setNav, app.notify,
 * app.toggleLike…). See docs/plugins.md for the full contract.
 */
export const ExampleView: React.FC<PluginViewProps> = ({ app }) => {
  const { t } = useI18n();
  const [pressed, setPressed] = useState<string | null>(null);

  const demo = (label: string, fn: () => void) => {
    fn();
    setPressed(label);
    setTimeout(() => setPressed(null), 1200);
  };

  return (
    <div className="max-w-3xl mx-auto px-6 py-8 space-y-6 select-none">
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 rounded-xl bg-apple-pink/15 border border-apple-pink/30 flex items-center justify-center">
          <Sparkles className="w-5 h-5 text-apple-pink" />
        </div>
        <div>
          <h2 className="text-sm font-bold text-white">{t("Example plugin")}</h2>
          <p className="text-[11px] text-apple-subtext">{t("Everything below reads from the { app } prop")}</p>
        </div>
      </div>

      {/* ── Reading host data ─────────────────────────────────────────── */}
      <Glass cornerRadius={14}>
        <div className="p-5 space-y-3">
          <p className="text-[11px] font-bold text-apple-subtext uppercase tracking-wider">
            {t("Read from app")}
          </p>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="bg-white/5 rounded-lg px-3 py-2 flex justify-between">
              <span className="text-apple-subtext">{t("Library")}</span>
              <span className="text-white font-semibold">{app.files.length}</span>
            </div>
            <div className="bg-white/5 rounded-lg px-3 py-2 flex justify-between">
              <span className="text-apple-subtext">{t("Playlists")}</span>
              <span className="text-white font-semibold">{app.playlists.length}</span>
            </div>
            <div className="bg-white/5 rounded-lg px-3 py-2 flex justify-between">
              <span className="text-apple-subtext">{t("Liked")}</span>
              <span className="text-white font-semibold">{app.likes.size}</span>
            </div>
            <div className="bg-white/5 rounded-lg px-3 py-2 flex justify-between">
              <span className="text-apple-subtext">{t("Current nav")}</span>
              <span className="text-white font-semibold truncate">{app.nav}</span>
            </div>
          </div>
        </div>
      </Glass>

      {/* ── Calling host actions ──────────────────────────────────────── */}
      <Glass cornerRadius={14}>
        <div className="p-5 space-y-3">
          <p className="text-[11px] font-bold text-apple-subtext uppercase tracking-wider">
            {t("Call app actions")}
          </p>
          <div className="flex flex-wrap gap-2 text-xs">
            <button
              onClick={() => {
                const f = app.files[0];
                if (f) demo("play", () => app.playTrack(f.rel_path));
              }}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-semibold transition-colors ${
                pressed === "play"
                  ? "bg-apple-pink text-white"
                  : "bg-white/5 text-zinc-200 hover:bg-apple-pink/20"
              }`}
            >
              <Play className="w-3.5 h-3.5" /> {t("Play first track")}
            </button>
            <button
              onClick={() => {
                const f = app.files[1];
                if (f) demo("like", () => app.toggleLike(f.rel_path));
              }}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-semibold transition-colors ${
                pressed === "like"
                  ? "bg-apple-pink text-white"
                  : "bg-white/5 text-zinc-200 hover:bg-apple-pink/20"
              }`}
            >
              <Heart className="w-3.5 h-3.5" /> {t("Like second track")}
            </button>
            <button
              onClick={() => demo("nav", () => app.setNav("songs"))}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-semibold transition-colors ${
                pressed === "nav"
                  ? "bg-apple-pink text-white"
                  : "bg-white/5 text-zinc-200 hover:bg-apple-pink/20"
              }`}
            >
              <ListMusic className="w-3.5 h-3.5" /> {t("Jump to library")}
            </button>
            <button
              onClick={() =>
                demo("notify", () =>
                  app.notify("success", t("Example plugin"), t("This toast came from a plugin action"))
                )
              }
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg font-semibold transition-colors ${
                pressed === "notify"
                  ? "bg-apple-pink text-white"
                  : "bg-white/5 text-zinc-200 hover:bg-apple-pink/20"
              }`}
            >
              <Bell className="w-3.5 h-3.5" /> {t("Show a toast")}
            </button>
          </div>
          {app.files[0] && (
            <p className="text-[11px] text-apple-subtext">
              {t("First track")} : <span className="text-white">{cleanTitle(app.files[0].title)}</span> —{" "}
              {app.files[0].artist}
            </p>
          )}
        </div>
      </Glass>

      {/* ── Badge & docs pointer ──────────────────────────────────────── */}
      <Glass cornerRadius={14}>
        <div className="p-5 flex items-start gap-3">
          <FileText className="w-4 h-4 text-apple-pink shrink-0 mt-0.5" />
          <div className="text-xs text-zinc-300 space-y-1">
            <p className="text-white font-bold">{t("How this works")}</p>
            <p>
              {t("This view is declared in src/plugins/example.ts with its component and a badge. Disable it anytime from the Marketplace.")}
            </p>
            <button
              onClick={() => demo("docs", () => app.setNav("marketplace"))}
              className="flex items-center gap-1 text-apple-pink hover:underline font-semibold"
            >
              {t("Open the Marketplace")} <ArrowRight className="w-3 h-3" />
            </button>
          </div>
        </div>
      </Glass>
    </div>
  );
};
