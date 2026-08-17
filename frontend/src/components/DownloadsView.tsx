import React, { useEffect, useState } from "react";
import { Glass } from "./Glass";
import {
  CheckCircle2,
  AlertCircle,
  Clock,
  Zap,
  Music,
  Disc,
  DownloadCloud,
  Filter,
  RefreshCw,
  Loader2,
} from "lucide-react";
import { cleanTitle } from "@/format";
import type { PluginViewProps } from "@/framework/plugin.types";
import type { JobRow } from "@/types";
import { useI18n } from "@/i18n";

export const DownloadsView: React.FC<PluginViewProps> = ({ app }) => {
  const { t } = useI18n();
  const tasks = app.tasks;
  const jobs = app.jobs;

  // Tic à la seconde : alimente le compte à rebours des retries (backoff M4).
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const active = tasks.filter((x) => x.status === "downloading");
  const queued = tasks.filter((x) => x.status === "queued");
  const failed = tasks.filter((x) => x.status === "failed");
  const recent = tasks.flatMap((x) => x.recent_tracks || []);

  // ── File M4 (jobs) ──────────────────────────────────────────────────
  const runningJobs = jobs.filter((j) => j.status === "running");
  // En attente d'un worker (FIFO) vs en backoff (retry_at futur → compte à
  // rebours affiché dans le chip).
  const queuedJobs = jobs.filter(
    (j) => j.status === "queued" && !(j.retry_at && new Date(j.retry_at).getTime() > now)
  );
  const retryingJobs = jobs.filter(
    (j) => j.status === "queued" && !!j.retry_at && new Date(j.retry_at).getTime() > now
  );
  const failedJobs = jobs.filter((j) => j.status === "failed");
  const doneJobs = jobs.filter((j) => j.status === "done").slice(-12);

  const jobTypeLabel = (j: JobRow) =>
    j.type === "fast_filter" ? t("Fast Filter") : j.type === "download" ? t("Download") : j.type;

  const retryLeft = (j: JobRow): number | null => {
    if (!j.retry_at) return null;
    const ms = new Date(j.retry_at).getTime() - now;
    if (ms <= 0) return null; // prêt — repris au prochain dequeue
    return Math.ceil(ms / 1000);
  };

  const jobRow = (j: JobRow, muted = false) => {
    const retry = retryLeft(j);
    const isRetry = j.status === "queued" && retry !== null;
    return (
      <div
        key={j.id}
        className={`bg-[#242428]/40 border rounded-xl p-2.5 flex items-center gap-2.5 text-xs ${
          muted ? "border-white/5 opacity-55" : "border-white/10"
        }`}
      >
        <div
          className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 ${
            j.type === "fast_filter"
              ? "bg-sky-500/15 text-sky-400 border border-sky-500/30"
              : "bg-apple-pink/15 text-apple-pink border border-apple-pink/30"
          }`}
        >
          {j.type === "fast_filter" ? <Filter className="w-3.5 h-3.5" /> : <Music className="w-3.5 h-3.5" />}
        </div>
        <div className="min-w-0 flex-1">
          <p className="font-semibold text-white truncate">{jobTypeLabel(j)}</p>
          <p className="text-[10px] text-apple-subtext truncate">
            {j.status === "failed" && j.error
              ? j.error
              : j.type === "fast_filter"
                ? t("Scans the link against your library")
                : t("Downloads tracks from the link")}
          </p>
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          {isRetry && <RefreshCw className="w-3 h-3 text-amber-400 animate-spin" />}
          <span
            className={`px-2 py-0.5 rounded-full font-medium text-[10px] ${
              j.status === "running"
                ? "bg-sky-500/15 text-sky-300"
                : isRetry
                  ? "bg-amber-500/15 text-amber-300"
                  : j.status === "failed"
                    ? "bg-rose-500/15 text-rose-300"
                    : j.status === "done"
                      ? "bg-emerald-500/15 text-emerald-300"
                      : "bg-white/10 text-zinc-300"
            }`}
          >
            {j.status === "running" ? (
              <span className="inline-flex items-center gap-1">
                <Loader2 className="w-3 h-3 animate-spin" />
                {t("Running")}
              </span>
            ) : isRetry ? (
              `${t("Retry in")} ${retry}s`
            ) : j.status === "failed" ? (
              t("Failed")
            ) : j.status === "done" ? (
              t("Done")
            ) : (
              t("Queued")
            )}
          </span>
          <span className="text-[10px] text-zinc-500" title={`${j.attempts}/${j.max_attempts} ${t("attempts")}`}>
            {j.attempts}/{j.max_attempts}
          </span>
        </div>
      </div>
    );
  };

  const hasJobs = jobs.length > 0;

  if (tasks.length === 0 && !hasJobs) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-apple-subtext select-none">
        <DownloadCloud className="w-12 h-12 mb-3 opacity-30" />
        <p className="text-sm font-semibold text-zinc-400">{t("No downloads yet")}</p>
        <p className="text-xs text-zinc-500 mt-1">{t("Import a link above to start downloading")}</p>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-8 space-y-8 select-none">
      {/* Active tasks */}
      {active.length > 0 && (
        <section className="space-y-3">
          <h3 className="text-xs font-semibold text-apple-pink uppercase tracking-wider flex items-center gap-1.5">
            <Zap className="w-3.5 h-3.5" />
            <span>{t("Downloading Now")}</span>
          </h3>
          <div className="space-y-3">
            {active.map((task) => {
              const total = task.total_tracks || 0;
              const done = task.completed_count || 0;
              const percent = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
              const currentSong = task.current_track || t("Processing playlist...");
              return (
                <Glass key={task.id} cornerRadius={14}>
                <div className="border border-apple-pink/30 rounded-xl p-3.5 space-y-3">
                  <div className="flex items-start gap-3">
                    <div className="w-9 h-9 rounded-lg bg-apple-pink/20 border border-apple-pink/40 flex items-center justify-center text-apple-pink shrink-0 mt-0.5">
                      <Music className="w-4.5 h-4.5 animate-bounce" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <span className="text-[10px] uppercase font-bold tracking-wider text-apple-pink">{t("Now Downloading")}</span>
                      <p className="text-xs font-bold text-white truncate leading-snug">{cleanTitle(currentSong)}</p>
                      <p className="text-[10px] text-apple-subtext truncate mt-0.5">{task.url}</p>
                    </div>
                  </div>
                  <div className="space-y-1.5 pt-1 border-t border-white/5">
                    <div className="flex justify-between text-xs text-apple-subtext font-medium">
                      <span>{total > 0 ? `${Math.min(done, total)} ${t("of")} ${total} ${t("songs")}` : task.progress}</span>
                      <span className="font-bold text-white">{percent}%</span>
                    </div>
                    <div className="w-full h-2 bg-black/40 rounded-full overflow-hidden p-0.5 border border-white/10">
                      <div className="h-full bg-apple-pink rounded-full transition-all duration-300 shadow-sm" style={{ width: `${Math.max(percent, 5)}%` }} />
                    </div>
                  </div>
                  <p className="text-[11px] text-zinc-400 truncate pt-0.5">{task.progress}</p>
                </div>
                </Glass>
              );
            })}
          </div>
        </section>
      )}

      {/* Recent downloads */}
      {recent.length > 0 && (
        <section className="space-y-3">
          <h3 className="text-xs font-semibold text-emerald-400 uppercase tracking-wider flex items-center gap-1.5">
            <CheckCircle2 className="w-3.5 h-3.5" />
            <span>{t("Downloaded Songs")} ({recent.length})</span>
          </h3>
          <div className="space-y-2">
            {recent.slice(0, 20).map((song, idx) => (
              <div key={`recent_${idx}`} className="bg-[#242428]/60 border border-emerald-500/20 rounded-xl p-2.5 flex items-center justify-between text-xs text-zinc-200">
                <div className="flex items-center gap-2.5 min-w-0">
                  <Disc className="w-4 h-4 text-emerald-400 shrink-0" />
                  <span className="truncate font-semibold text-white">{cleanTitle(song)}</span>
                </div>
                <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full font-medium shrink-0">{t("Downloaded")}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Queued */}
      {queued.length > 0 && (
        <section className="space-y-3">
          <h3 className="text-xs font-semibold text-apple-subtext uppercase tracking-wider flex items-center gap-1.5">
            <Clock className="w-3.5 h-3.5" />
            <span>{t("Up Next in Queue")} ({queued.length})</span>
          </h3>
          <div className="space-y-2">
            {queued.map((task) => (
              <div key={task.id} className="bg-[#242428]/40 border border-white/5 rounded-xl p-3 flex items-center justify-between text-xs text-apple-subtext">
                <span className="truncate max-w-[420px] text-zinc-300 font-medium">{task.url}</span>
                <span className="text-[10px] bg-white/10 text-zinc-300 px-2 py-0.5 rounded-full font-medium shrink-0 ml-2">{t("Queued")}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Failed */}
      {failed.length > 0 && (
        <section className="space-y-3">
          <h3 className="text-xs font-semibold text-rose-400 uppercase tracking-wider flex items-center gap-1.5">
            <AlertCircle className="w-3.5 h-3.5" />
            <span>{t("Failed Imports")} ({failed.length})</span>
          </h3>
          <div className="space-y-2">
            {failed.map((task) => (
              <div key={task.id} className="bg-rose-500/10 border border-rose-500/30 rounded-xl p-3 text-xs text-rose-300 space-y-1">
                <p className="font-semibold truncate">{task.url}</p>
                <p className="text-[11px] text-rose-400/80">{task.error || t("Execution error")}</p>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Job Queue (file M4) — poussée en direct par les événements job_update */}
      {(runningJobs.length > 0 || queuedJobs.length > 0 || retryingJobs.length > 0 || failedJobs.length > 0 || doneJobs.length > 0) && (
        <section className="space-y-3">
          <h3 className="text-xs font-semibold text-apple-subtext uppercase tracking-wider flex items-center gap-1.5">
            <Zap className="w-3.5 h-3.5" />
            <span>{t("Job Queue")} ({jobs.length})</span>
          </h3>
          <div className="space-y-2">
            {runningJobs.map((j) => jobRow(j))}
            {queuedJobs.map((j) => jobRow(j))}
            {retryingJobs.map((j) => jobRow(j))}
            {failedJobs.map((j) => jobRow(j))}
            {doneJobs.map((j) => jobRow(j, true))}
          </div>
        </section>
      )}
    </div>
  );
};
