import React, { useEffect, useState } from "react";
import {
  Play,
  Square,
  RefreshCw,
  FolderOpen,
  HardDrive,
  Settings2,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Music2,
  Gauge,
  KeyRound,
  ScanSearch,
  Trash2,
  FolderOutput,
} from "lucide-react";
import { useI18n } from "@/i18n";
import { apiFetch, setToken, getToken } from "@/api";
import { cleanTitle } from "@/format";
import type { PluginViewProps } from "@/framework/plugin.types";
import type { DownloadedFile } from "@/types";

interface SyncStatus {
  available: boolean;
  running: boolean;
  platform: string;
  downloads_dir: string;
  auto_add_dir?: string;
  imported_count: number;
  pid?: number;
  script_path?: string;
  state_file: string;
  log_file: string;
  error?: string;
}

interface AppSettings {
  workers: number;
  threads: number;
  playlist_export_dir?: string;
}

interface DupGroup {
  title: string;
  artist: string;
  files: DownloadedFile[];
  keep_rel_path: string;
}

export const SyncSettingsView: React.FC<PluginViewProps> = ({ app }) => {
  const { t } = useI18n();
  const getApiUrl = app.getApiUrl;
  const onNotify = app.notify;
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [settings, setSettings] = useState<AppSettings>({ workers: 4, threads: 6 });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"start" | "stop" | "save" | null>(null);
  const [token, setTokenInput] = useState<string>(() => getToken());

  // ── Dédoublonnage ──
  const [dupGroups, setDupGroups] = useState<DupGroup[]>([]);
  const [dupTotal, setDupTotal] = useState(0);
  const [dupScanning, setDupScanning] = useState(false);
  const [dupDeleting, setDupDeleting] = useState(false);
  const [scannedOnce, setScannedOnce] = useState(false);
  const [toDelete, setToDelete] = useState<Set<string>>(new Set());
  const [keepFor, setKeepFor] = useState<Record<string, string>>({});

  // ── Export playlists ──
  const [exportDir, setExportDir] = useState<string>("");
  const [exportBusy, setExportBusy] = useState(false);
  const [exported, setExported] = useState<{ name: string; track_count: number }[] | null>(null);

  const refresh = async () => {
    try {
      const [sRes, setRes] = await Promise.all([
        apiFetch(`${getApiUrl()}/sync/status`),
        apiFetch(`${getApiUrl()}/settings`),
      ]);
      if (sRes.ok) setStatus(await sRes.json());
      if (setRes.ok) {
        const s = await setRes.json();
        setSettings(s);
        setExportDir(s.playlist_export_dir || "");
      }
    } catch {
      onNotify("error", t("Network Error"), t("Cannot reach the backend"));
    } finally {
      setLoading(false);
    }
  };

  const scanDuplicates = async () => {
    setDupScanning(true);
    setScannedOnce(true);
    setDupGroups([]);
    setDupTotal(0);
    try {
      const res = await apiFetch(`${getApiUrl()}/duplicates`);
      const data = await res.json();
      if (res.ok) {
        setDupGroups(data.groups ?? []);
        setDupTotal(data.total ?? 0);
        // Pré-sélection : tout sauf le fichier recommandé à garder. On garde
        // aussi la correspondance supprimé → gardé pour transférer les stats
        // (écoutes, likes, playlists) des copies vers celle qu'on conserve.
        const sel = new Set<string>();
        const keep: Record<string, string> = {};
        for (const g of data.groups ?? []) {
          for (const f of g.files) {
            if (f.rel_path !== g.keep_rel_path) {
              sel.add(f.rel_path);
              keep[f.rel_path] = g.keep_rel_path;
            }
          }
        }
        setToDelete(sel);
        setKeepFor(keep);
      } else {
        onNotify("error", t("Error"), data.error || t("Action denied"));
      }
    } catch {
      onNotify("error", t("Network Error"), t("Action failed"));
    } finally {
      setDupScanning(false);
    }
  };

  const togglePath = (p: string) => {
    setToDelete((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  };

  const removeDuplicates = async () => {
    if (toDelete.size === 0) return;
    setDupDeleting(true);
    try {
      const res = await apiFetch(`${getApiUrl()}/duplicates/remove`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ paths: [...toDelete], keep_for: keepFor }),
      });
      const data = await res.json();
      if (res.ok) {
        onNotify(
          "success",
          t("Duplicates removed"),
          `${data.deleted} ${t("duplicates")} — ${t("stats kept on the kept copy")}`
        );
        setDupGroups([]);
        setDupTotal(0);
        setToDelete(new Set());
        setKeepFor({});
      } else {
        onNotify("error", t("Error"), data.error || t("Action denied"));
      }
    } catch {
      onNotify("error", t("Network Error"), t("Action failed"));
    } finally {
      setDupDeleting(false);
    }
  };

  const exportPlaylists = async () => {
    const dir = exportDir.trim();
    if (!dir) return;
    setExportBusy(true);
    setExported(null);
    try {
      const res = await apiFetch(`${getApiUrl()}/playlists/export`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dir }),
      });
      const data = await res.json();
      if (res.ok) {
        setExported(data.files ?? []);
        onNotify(
          "success",
          t("Playlists exported"),
          `${data.count ?? 0} .m3u8 → ${data.dir}`
        );
      } else {
        onNotify("error", t("Error"), data.error || t("Action denied"));
      }
    } catch {
      onNotify("error", t("Network Error"), t("Action failed"));
    } finally {
      setExportBusy(false);
    }
  };

  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggleSync = async (action: "start" | "stop") => {
    setBusy(action);
    try {
      const res = await apiFetch(`${getApiUrl()}/sync/${action}`, { method: "POST" });
      const data = await res.json();
      if (res.ok) {
        setStatus(data);
        onNotify(
          "success",
          action === "start" ? t("Watcher started") : t("Watcher stopped"),
          action === "start"
            ? t("New files will be automatically imported into Music.")
            : t("Auto-import is disabled.")
        );
      } else {
        onNotify("error", t("Failed"), data.error || t("Action denied"));
      }
    } catch {
      onNotify("error", t("Network Error"), t("Action failed"));
    } finally {
      setBusy(null);
    }
  };

  const saveSettings = async () => {
    setBusy("save");
    try {
      const res = await apiFetch(`${getApiUrl()}/settings`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(settings),
      });
      const data = await res.json();
      if (res.ok) {
        setSettings(data.settings);
        onNotify(
          "success",
          t("Settings saved"),
          t("Threads apply to next downloads; workers at next start.")
        );
      } else {
        onNotify("error", t("Error"), data.error || t("Save failed"));
      }
    } catch {
      onNotify("error", t("Network Error"), t("Cannot save"));
    } finally {
      setBusy(null);
    }
  };

  const saveToken = () => {
    setToken(token);
    onNotify("success", t("Token saved"), t("Token stored in this browser only."));
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-apple-subtext gap-2 text-sm">
        <Loader2 className="w-4 h-4 animate-spin" /> {t("Loading…")}
      </div>
    );
  }

  const statusPill = status?.available
    ? status.running
      ? { label: t("In progress"), cls: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30" }
      : { label: t("Stopped"), cls: "bg-white/5 text-zinc-300 border-white/10" }
    : { label: t("Unavailable"), cls: "bg-rose-500/15 text-rose-400 border-rose-500/30" };

  return (
    <div className="max-w-3xl mx-auto px-6 py-8 space-y-8 select-none">
      {/* ── Auto-Import ──────────────────────────────────────────── */}
      <section className="bg-[#1e1e20] border border-white/10 rounded-2xl p-6 space-y-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-apple-pink/15 border border-apple-pink/30 flex items-center justify-center">
              <Music2 className="w-5 h-5 text-apple-pink" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white">{t("Auto-Import into Music")}</h2>
              <p className="text-[11px] text-apple-subtext">
                {t("Copies new files automatically into the Music app — no duplicates")}
              </p>
            </div>
          </div>
          <span
            className={`text-[10px] font-bold px-3 py-1 rounded-full border ${
              statusPill.cls
            } ${status?.running ? "animate-pulse" : ""}`}
          >
            {statusPill.label}
          </span>
        </div>

        {status?.platform !== "darwin" && (
          <div className="flex items-start gap-2.5 text-xs text-amber-400 bg-amber-500/10 border border-amber-500/20 rounded-xl p-3">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
            <p>
              {t("Auto-import requires macOS with the Music app installed. On a server (VPS), distribution is handled by Syncthing — install the watcher on your Mac with the script")}{" "}
              <code className="text-amber-300">{status?.script_path || "scripts/watch_and_import.sh"}</code>.
            </p>
          </div>
        )}

        {status?.error && (
          <div className="flex items-start gap-2.5 text-xs text-amber-400 bg-amber-500/10 border border-amber-500/20 rounded-xl p-3">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
            <p>{status.error}</p>
          </div>
        )}

        <div className="space-y-2 text-xs">
          <div className="flex items-center gap-2.5 text-apple-subtext">
            <FolderOpen className="w-3.5 h-3.5 shrink-0" />
            <span className="w-36 shrink-0">{t("Watched folder")}</span>
            <span className="text-zinc-200 truncate">{status?.downloads_dir}</span>
          </div>
          {status?.auto_add_dir && (
            <div className="flex items-center gap-2.5 text-apple-subtext">
              <HardDrive className="w-3.5 h-3.5 shrink-0" />
              <span className="w-36 shrink-0">{t("Music folder")}</span>
              <span className="text-zinc-200 truncate">{status.auto_add_dir}</span>
            </div>
          )}
          <div className="flex items-center gap-2.5 text-apple-subtext">
            <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
            <span className="w-36 shrink-0">{t("Files imported")}</span>
            <span className="text-zinc-200">{status?.imported_count ?? 0}</span>
          </div>
          {status?.script_path && (
            <div className="flex items-center gap-2.5 text-apple-subtext">
              <FolderOutput className="w-3.5 h-3.5 shrink-0" />
              <span className="w-36 shrink-0">{t("Watcher script")}</span>
              <span className="text-zinc-200 truncate" title={status.script_path}>
                {status.script_path}
              </span>
            </div>
          )}
        </div>

        <div className="flex items-center gap-2.5 pt-2">
          {status?.available && (
            <>
              <button
                onClick={() => toggleSync("start")}
                disabled={busy !== null || status?.running}
                className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-40 disabled:cursor-not-allowed px-4 py-2 rounded-full transition-colors"
              >
                {busy === "start" ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Play className="w-3.5 h-3.5 fill-current" />
                )}
                {t("Start")}
              </button>
              <button
                onClick={() => toggleSync("stop")}
                disabled={busy !== null || !status?.running}
                className="flex items-center gap-2 text-xs font-semibold text-zinc-200 bg-white/10 hover:bg-white/20 disabled:opacity-40 disabled:cursor-not-allowed px-4 py-2 rounded-full transition-colors"
              >
                {busy === "stop" ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Square className="w-3.5 h-3.5 fill-current" />
                )}
                {t("Stop")}
              </button>
            </>
          )}
          <button
            onClick={refresh}
            disabled={busy !== null}
            className="flex items-center gap-2 text-xs font-semibold text-apple-subtext hover:text-white disabled:opacity-40 px-3 py-2 rounded-full transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${busy === "start" || busy === "stop" ? "animate-spin" : ""}`} />
            {t("Refresh")}
          </button>
        </div>
      </section>

      {/* ── Réglages de téléchargement ──────────────────────────── */}
      <section className="bg-[#1e1e20] border border-white/10 rounded-2xl p-6 space-y-5">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-white/5 border border-white/10 flex items-center justify-center">
            <Gauge className="w-5 h-5 text-apple-pink" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-white">{t("Download settings")}</h2>
            <p className="text-[11px] text-apple-subtext">
              {t("Speed vs stability: too much parallelism triggers platform rate limiting")}
            </p>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <label className="block">
            <span className="text-[11px] font-semibold text-apple-subtext uppercase tracking-wider">
              {t("Parallel songs (threads)")}
            </span>
            <input
              type="number"
              min={1}
              max={32}
              value={settings.threads}
              onChange={(e) => setSettings({ ...settings, threads: parseInt(e.target.value) || 1 })}
              className="mt-1.5 w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg px-3 py-2 text-sm text-white focus:outline-none"
            />
            <span className="text-[10px] text-apple-subtext mt-1 block">
              {t("Applies to next downloads (default 6)")}
            </span>
          </label>
          <label className="block">
            <span className="text-[11px] font-semibold text-apple-subtext uppercase tracking-wider">
              {t("Parallel playlists (workers)")}
            </span>
            <input
              type="number"
              min={1}
              max={16}
              value={settings.workers}
              onChange={(e) => setSettings({ ...settings, workers: parseInt(e.target.value) || 1 })}
              className="mt-1.5 w-full bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg px-3 py-2 text-sm text-white focus:outline-none"
            />
            <span className="text-[10px] text-apple-subtext mt-1 block">
              {t("Applies at next start (default 4)")}
            </span>
          </label>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={saveSettings}
            disabled={busy !== null}
            className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-40 px-4 py-2 rounded-full transition-colors"
          >
            {busy === "save" ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Settings2 className="w-3.5 h-3.5" />
            )}
            {t("Save")}
          </button>
        </div>
      </section>

      {/* ── Dédoublonnage ──────────────────────────────────────── */}
      <section className="bg-[#1e1e20] border border-white/10 rounded-2xl p-6 space-y-5">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-white/5 border border-white/10 flex items-center justify-center">
            <ScanSearch className="w-5 h-5 text-apple-pink" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-white">{t("Deduplicate library")}</h2>
            <p className="text-[11px] text-apple-subtext">
              {t("Apple Music rips, (1) copies and same-title tracks are grouped — keep one, delete the rest.")}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={scanDuplicates}
            disabled={dupScanning || dupDeleting}
            className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-40 disabled:cursor-not-allowed px-4 py-2 rounded-full transition-colors"
          >
            {dupScanning ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <ScanSearch className="w-3.5 h-3.5" />
            )}
            {dupScanning ? t("Scanning…") : t("Scan duplicates")}
          </button>
          {dupGroups.length > 0 && (
            <button
              onClick={removeDuplicates}
              disabled={dupDeleting || toDelete.size === 0}
              className="flex items-center gap-2 text-xs font-semibold text-rose-300 bg-rose-500/15 hover:bg-rose-500/25 border border-rose-500/30 disabled:opacity-40 disabled:cursor-not-allowed px-4 py-2 rounded-full transition-colors"
            >
              {dupDeleting ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <Trash2 className="w-3.5 h-3.5" />
              )}
              {t("Delete selected")} ({toDelete.size})
            </button>
          )}
        </div>

        {dupScanning && <p className="text-xs text-apple-subtext">{t("Scanning…")}</p>}

        {!dupScanning && scannedOnce && dupGroups.length === 0 && (
          <p className="text-xs text-emerald-400 font-semibold">{t("No duplicates found")}</p>
        )}

        {dupGroups.length > 0 && (
          <div className="space-y-3">
            <p className="text-[11px] text-apple-subtext">
              {dupGroups.length} {t("duplicate group(s) found")} · {dupTotal} {t("duplicates")}
            </p>
            <div className="space-y-2 max-h-80 overflow-y-auto pr-1">
              {dupGroups.map((g, gi) => (
                <div key={gi} className="bg-white/5 rounded-xl p-3 space-y-1">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs font-bold text-white truncate">
                      {cleanTitle(g.title)}{" "}
                      <span className="text-apple-subtext font-normal">— {g.artist}</span>
                    </p>
                    <span className="text-[10px] text-apple-subtext shrink-0">
                      {g.files.length} {t("files")}
                    </span>
                  </div>
                  {g.files.map((f) => {
                    const isKeep = f.rel_path === g.keep_rel_path;
                    const checked = toDelete.has(f.rel_path);
                    return (
                      <label
                        key={f.rel_path}
                        className={`flex items-center gap-2.5 text-xs rounded-lg px-2 py-1.5 transition-colors ${
                          isKeep ? "bg-emerald-500/10" : "cursor-pointer hover:bg-white/5"
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={isKeep ? false : checked}
                          disabled={isKeep}
                          onChange={() => togglePath(f.rel_path)}
                          className="accent-apple-pink shrink-0"
                        />
                        <span
                          className={`flex-1 min-w-0 truncate ${
                            isKeep ? "text-emerald-400 font-semibold" : "text-zinc-300"
                          }`}
                        >
                          {isKeep && <>★ {t("Keep")} — </>}
                          {cleanTitle(f.title)} · {f.album} ·{" "}
                          {f.size ? `${(f.size / 1048576).toFixed(1)} MB` : ""}
                        </span>
                      </label>
                    );
                  })}
                </div>
              ))}
            </div>
          </div>
        )}
      </section>

      {/* ── Export playlists (iPhone / Syncthing) ────────────────── */}
      <section className="bg-[#1e1e20] border border-white/10 rounded-2xl p-6 space-y-5">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-white/5 border border-white/10 flex items-center justify-center">
            <FolderOutput className="w-5 h-5 text-apple-pink" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-white">{t("Export playlists")}</h2>
            <p className="text-[11px] text-apple-subtext">
              {t("Writes one .m3u8 per playlist with relative paths — drop them in a Syncthing folder or on the iPhone via USB.")}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <input
            value={exportDir}
            onChange={(e) => setExportDir(e.target.value)}
            placeholder="~/Syncthing/iPhone · /Volumes/iPhone/Playlists"
            className="flex-1 bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg px-3 py-2 text-sm text-white focus:outline-none font-mono placeholder-apple-subtext"
          />
          <button
            onClick={exportPlaylists}
            disabled={exportBusy || !exportDir.trim()}
            className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover disabled:opacity-40 disabled:cursor-not-allowed px-4 py-2 rounded-full transition-colors shrink-0"
          >
            {exportBusy ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <FolderOutput className="w-3.5 h-3.5" />
            )}
            {exportBusy ? t("Exporting…") : t("Export")}
          </button>
        </div>

        {exported && (
          <div className="text-xs space-y-1">
            <p className="text-emerald-400 font-semibold">
              {t("Playlists exported")} → {exportDir.trim()}
            </p>
            <p className="text-apple-subtext">
              {exported.length} .m3u8 — {t("Playlist files written to")} {exportDir.trim()}
            </p>
          </div>
        )}
      </section>

      {/* ── API Token ───────────────────────────────────────────── */}
      <section className="bg-[#1e1e20] border border-white/10 rounded-2xl p-6 space-y-5">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-white/5 border border-white/10 flex items-center justify-center">
            <KeyRound className="w-5 h-5 text-apple-pink" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-white">{t("API Token (optional)")}</h2>
            <p className="text-[11px] text-apple-subtext">
              {t("Protect the API with a token. Must match the server SONEPH_TOKEN env var.")}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <input
            type="password"
            value={token}
            onChange={(e) => setTokenInput(e.target.value)}
            placeholder="••••••••••••"
            className="flex-1 bg-[#242428] border border-white/10 focus:border-apple-pink rounded-lg px-3 py-2 text-sm text-white focus:outline-none"
          />
          <button
            onClick={saveToken}
            className="flex items-center gap-2 text-xs font-semibold text-white bg-apple-pink hover:bg-apple-pinkHover px-4 py-2 rounded-full transition-colors shrink-0"
          >
            <KeyRound className="w-3.5 h-3.5" />
            {t("Save")}
          </button>
        </div>
      </section>
    </div>
  );
};
