import { BarChart3 } from "lucide-react";
import { StatsView } from "@/components/StatsView";
import type { PluginManifest } from "@/framework/plugin.types";

// Stats plugin — the former "Stats Module": Wrapped-style listening stats.
export const statsPlugin: PluginManifest = {
  id: "stats",
  nameKey: "Stats Module",
  descKey: "Stats Module Desc",
  version: "1.0.0",
  icon: BarChart3,
  defaultEnabled: true,
  contributes: {
    views: [
      {
        id: "stats",
        labelKey: "Stats",
        section: "music",
        icon: BarChart3,
        component: StatsView,
      },
    ],
  },
};
