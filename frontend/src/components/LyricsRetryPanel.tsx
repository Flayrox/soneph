import React, { useState, useEffect, useCallback } from "react";
import { Music2, RefreshCw, CheckCircle2, XCircle, ChevronDown, ChevronUp, Loader2 } from "lucide-react";
import { useI18n } from "@/i18n";
import { apiFetch } from "@/api";

interface LyricsJob {
  status: "idle" | "running" | "done";
  total: number;
  done: number;
  success: number;
  failed: number;
  current: string;
  logs: string[];
  started_at?: string;
}

interface MissingScan {
  total_mp3s: number;
  missing_lrc: number;
  unsynced_lrc: number;
}

export const LyricsRetryPanel: React.FC = () => {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scan, setScan] = useState<MissingScan | null>(null);
  const [job, setJob] = useState<LyricsJob | null>(null);
  const [showLogs, setShowLogs] = useState(false);

  // Poll job status when running
  useEffect(() => {
    if (!job || job.status !== "running") return;
    const interval = setInterval(async () => {
      try {
        const res = await apiFetch("/api/lyrics/retry");
        const data = await res.json();
        setJob(data.job);
      } catch {}
    }, 1500);
    return () => clearInterval(interval);
  }, [job?.status]);

  const handleScan = useCallback(async () => {
    setScanning(true);
    setScan(null);
    try {
      const res = await apiFetch("/api/lyrics/missing");
      const data = await res.json();
      const scanData = data.scan as {
        missing_lrc?: number;
        unsynced_lrc?: number;
        total_mp3s?: number;
      };
      setScan({
        total_mp3s: scanData?.total_mp3s ?? 0,
        missing_lrc: scanData?.missing_lrc ?? 0,
        unsynced_lrc: scanData?.unsynced_lrc ?? 0,
      });
    } catch {
      setScan(null);
    } finally {
      setScanning(false);
    }
  }, []);

  const handleRetry = useCallback(async () => {
    try {
      const res = await apiFetch("/api/lyrics/retry", { method: "POST" });
      const data = await res.json();
      setJob(data.job);
    } catch {}
  }, []);

  const progress = job && job.total > 0
    ? Math.round((job.done / job.total) * 100)
    : 0;

  return (
    <div className="rounded-xl border border-white/8 overflow-hidden bg-white/3">
      {/* Header row */}
      <button
        onClick={() => {
          setExpanded((e) => !e);
          if (!expanded && !scan) handleScan();
        }}
        className="w-full flex items-center justify-between px-3 py-2 text-xs text-zinc-300 hover:bg-white/5 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Music2 className="w-3.5 h-3.5 text-apple-pink" />
          <span className="font-semibold">{t("Synced Lyrics")}</span>
        </div>
        {expanded ? (
          <ChevronUp className="w-3.5 h-3.5 text-zinc-500" />
        ) : (
          <ChevronDown className="w-3.5 h-3.5 text-zinc-500" />
        )}
      </button>

      {expanded && (
        <div className="px-3 pb-3 space-y-2.5">
          {/* Scan result */}
          {scanning && (
            <div className="flex items-center gap-2 text-[11px] text-zinc-400">
              <Loader2 className="w-3 h-3 animate-spin" />
              {t("Scanning library...")}
            </div>
          )}

          {scan && !scanning && (
            <div className="text-[11px] text-zinc-400 space-y-1">
              <div className="flex justify-between">
                <span>{t("Total songs")}</span>
                <span className="text-white font-semibold">{scan.total_mp3s}</span>
              </div>
              <div className="flex justify-between">
                <span>{t("Without synced lyrics")}</span>
                <span
                  className={
                    scan.missing_lrc + scan.unsynced_lrc > 0
                      ? "text-amber-400 font-semibold"
                      : "text-emerald-400 font-semibold"
                  }
                >
                  {scan.missing_lrc + scan.unsynced_lrc}
                </span>
              </div>
              {scan.unsynced_lrc > 0 && (
                <div className="flex justify-between text-zinc-500">
                  <span>{t("Plain text to upgrade")}</span>
                  <span>{scan.unsynced_lrc}</span>
                </div>
              )}
            </div>
          )}

          {/* Retry job progress */}
          {job && job.status !== "idle" && (
            <div className="space-y-1.5">
              {job.status === "running" && (
                <>
                  <div className="h-1 bg-white/10 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-apple-pink rounded-full transition-all duration-700"
                      style={{ width: `${progress}%` }}
                    />
                  </div>
                  <div className="text-[10px] text-zinc-400 flex justify-between">
                    <span className="truncate max-w-[140px]">{job.current || t("Starting…")}</span>
                    <span className="shrink-0 ml-1">{job.done}/{job.total}</span>
                  </div>
                </>
              )}

              {job.status === "done" && (
                <div className="text-[11px] space-y-0.5">
                  <div className="flex items-center gap-1.5 text-emerald-400">
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    <span className="font-semibold">{job.success} {t("lyrics added")}</span>
                  </div>
                  {job.failed > 0 && (
                    <div className="flex items-center gap-1.5 text-zinc-500">
                      <XCircle className="w-3.5 h-3.5" />
                      <span>{job.failed} {t("not found")}</span>
                    </div>
                  )}
                </div>
              )}

              {/* Logs toggle */}
              {job.logs && job.logs.length > 0 && (
                <button
                  onClick={() => setShowLogs((s) => !s)}
                  className="text-[10px] text-zinc-500 hover:text-zinc-300 transition-colors"
                >
                  {showLogs ? t("Hide") : t("Show")} {t("details")}
                </button>
              )}
              {showLogs && job.logs && (
                <div className="max-h-28 overflow-y-auto scrollbar-none space-y-0.5">
                  {job.logs.slice(0, 20).map((log, i) => (
                    <div key={i} className="text-[10px] text-zinc-500 truncate">
                      {log}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Action buttons */}
          <div className="flex gap-1.5 pt-0.5">
            <button
              onClick={handleScan}
              disabled={scanning}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-semibold bg-white/8 hover:bg-white/12 text-zinc-300 transition-colors disabled:opacity-40"
            >
              <RefreshCw className={`w-3 h-3 ${scanning ? "animate-spin" : ""}`} />
              {t("Scan")}
            </button>
            <button
              onClick={handleRetry}
              disabled={
                !scan ||
                scan.missing_lrc + scan.unsynced_lrc === 0 ||
                job?.status === "running"
              }
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-semibold bg-apple-pink/20 hover:bg-apple-pink/30 text-apple-pink transition-colors disabled:opacity-40"
            >
              <Music2 className="w-3 h-3" />
              {job?.status === "running" ? t("Running…") : t("Retry All")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
