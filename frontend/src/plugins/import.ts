import { DownloadCloud } from "lucide-react";
import { DownloadsView } from "@/components/DownloadsView";
import type { PluginManifest } from "@/framework/plugin.types";

// Import plugin — the former "Import Module": link-based downloads and the
// live download queue view.
export const importPlugin: PluginManifest = {
  id: "import",
  nameKey: "Import Module",
  descKey: "Import Module Desc",
  version: "1.0.0",
  icon: DownloadCloud,
  defaultEnabled: true,
  contributes: {
    views: [
      {
        id: "downloads",
        labelKey: "Downloads",
        section: "downloads",
        icon: DownloadCloud,
        component: DownloadsView,
        badge: (app) => {
          const active = app.tasks.filter(
            (t) => t.status === "downloading" || t.status === "queued"
          ).length;
          return active > 0 ? active : null;
        },
      },
    ],
  },
};
