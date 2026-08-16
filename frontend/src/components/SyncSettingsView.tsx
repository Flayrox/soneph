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
} from "lucide-react";

interface SyncStatus {
  available: boolean;
  running: boolean;
  platform: string;
  downloads_dir: string;
  auto_add_dir?: string;
  imported_count: number;
  pid?: number;
  state_file: string;
  log_file: string;
  error?: string;
}

interface AppSettings {
  workers: number;
  threads: number;
}

interface SyncSettingsViewProps {
  getApiUrl: () => string;
  onNotify: (type: "success" | "error" | "info", title: string, message: string) => void;
}

export const SyncSettingsView: React.FC<SyncSettingsViewProps> = ({ getApiUrl, onNotify }) => {
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [settings, setSettings] = useState<AppSettings>({ workers: 4, threads: 6 });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"start" | "stop" | "save" | null>(null);

  const refresh = async () => {
    try {
      const [sRes, setRes] = await Promise.all([
        fetch(`${getApiUrl()}/sync/status`),
        fetch(`${getApiUrl()}/settings`),
      ]);
      if (sRes.ok) setStatus(await sRes.json());
      if (setRes.ok) setSettings(await setRes.json());
    } catch {
      onNotify("error", "Erreur réseau", "Impossible de contacter le backend");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggleSync = async (action: "start" | "stop") => {
    setBusy(action);
    try {
      const res = await fetch(`${getApiUrl()}/sync/${action}`, { method: "POST" });
      const data = await res.json();
      if (res.ok) {
        setStatus(data);
        onNotify(
          "success",
          action === "start" ? "Watcher démarré" : "Watcher arrêté",
          action === "start"
            ? "Les nouveaux fichiers seront automatiquement importés dans Musique."
            : "L'auto-import est désactivé."
        );
      } else {
        onNotify("error", "Impossible", data.error || "Action refusée");
      }
    } catch {
      onNotify("error", "Erreur réseau", "Action impossible");
    } finally {
      setBusy(null);
    }
  };

  const saveSettings = async () => {
    setBusy("save");
    try {
      const res = await fetch(`${getApiUrl()}/settings`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(settings),
      });
      const data = await res.json();
      if (res.ok) {
        setSettings(data.settings);
        onNotify(
          "success",
          "Réglages enregistrés",
          "Threads appliqués aux prochains téléchargements ; workers au prochain démarrage."
        );
      } else {
        onNotify("error", "Erreur", data.error || "Échec de l'enregistrement");
      }
    } catch {
      onNotify("error", "Erreur réseau", "Impossible d'enregistrer");
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-apple-subtext gap-2 text-sm">
        <Loader2 className="w-4 h-4 animate-spin" /> Chargement…
      </div>
    );
  }

  const statusPill = status?.available
    ? status.running
      ? { label: "En cours", cls: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30" }
      : { label: "Arrêté", cls: "bg-white/5 text-zinc-300 border-white/10" }
    : { label: "Indisponible", cls: "bg-rose-500/15 text-rose-400 border-rose-500/30" };

  return (
    <div className="max-w-3xl mx-auto px-6 py-8 space-y-8 select-none">
      {/* ── Auto-Import l'app Musique ─────────────────────────────── */}
      <section className="bg-[#1e1e20] border border-white/10 rounded-2xl p-6 space-y-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-apple-pink/15 border border-apple-pink/30 flex items-center justify-center">
              <Music2 className="w-5 h-5 text-apple-pink" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white">Auto-Import l'app Musique</h2>
              <p className="text-[11px] text-apple-subtext">
                Copie automatiquement les nouveaux fichiers vers « Automatically Add to Music » — sans doublon
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
              L'auto-import nécessite macOS avec l'app Musique installée. Sur un serveur (VPS), la
              distribution est assurée par Syncthing — installe le watcher sur ton Mac avec le script{" "}
              <code className="text-amber-300">scripts/watch_and_import.sh</code>.
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
            <span className="w-36 shrink-0">Dossier surveillé</span>
            <span className="text-zinc-200 truncate">{status?.downloads_dir}</span>
          </div>
          {status?.auto_add_dir && (
            <div className="flex items-center gap-2.5 text-apple-subtext">
              <HardDrive className="w-3.5 h-3.5 shrink-0" />
              <span className="w-36 shrink-0">Dossier Musique</span>
              <span className="text-zinc-200 truncate">{status.auto_add_dir}</span>
            </div>
          )}
          <div className="flex items-center gap-2.5 text-apple-subtext">
            <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
            <span className="w-36 shrink-0">Fichiers importés</span>
            <span className="text-zinc-200">{status?.imported_count ?? 0}</span>
          </div>
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
                Démarrer
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
                Arrêter
              </button>
            </>
          )}
          <button
            onClick={refresh}
            disabled={busy !== null}
            className="flex items-center gap-2 text-xs font-semibold text-apple-subtext hover:text-white disabled:opacity-40 px-3 py-2 rounded-full transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${busy === "start" || busy === "stop" ? "animate-spin" : ""}`} />
            Actualiser
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
            <h2 className="text-sm font-bold text-white">Réglages de téléchargement</h2>
            <p className="text-[11px] text-apple-subtext">
              Vitesse vs stabilité : trop de parallélisme déclenche le rate limiting /YouTube
            </p>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <label className="block">
            <span className="text-[11px] font-semibold text-apple-subtext uppercase tracking-wider">
              Chansons en parallèle (threads)
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
              Appliqué aux prochains téléchargements (défaut 6)
            </span>
          </label>
          <label className="block">
            <span className="text-[11px] font-semibold text-apple-subtext uppercase tracking-wider">
              Playlists en parallèle (workers)
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
              Appliqué au prochain démarrage (défaut 4)
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
            Enregistrer
          </button>
        </div>
      </section>
    </div>
  );
};
