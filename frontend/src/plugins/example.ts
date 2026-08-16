import { FlaskConical, Lightbulb } from "lucide-react";
import { ExampleView } from "@/components/ExampleView";
import type { PluginManifest, PluginApp } from "@/framework/plugin.types";

/**
 * ── Example plugin ─────────────────────────────────────────────────────
 *
 * The smallest complete plugin. It demonstrates every piece of the
 * contract so you can copy it to build your own:
 *
 *   1. A view  — a sidebar entry that renders a React component which
 *      receives the single `{ app }` prop (the host's shared context).
 *   2. A badge — an optional count shown next to the nav entry.
 *   3. An action — a host-level command (e.g. for future command palette).
 *
 * Copy this file, change the ids, register it in src/plugins/index.ts and
 * it shows up in the sidebar + Marketplace, toggleable like any plugin.
 * See docs/plugins.md for the full guide.
 */
export const examplePlugin: PluginManifest = {
  id: "example",
  nameKey: "Example Plugin",
  descKey: "Example Plugin Desc",
  version: "1.0.0",
  icon: FlaskConical,
  // Not core → the user can enable/disable it from the Marketplace.
  defaultEnabled: true,
  contributes: {
    views: [
      {
        id: "example",
        labelKey: "Example",
        section: "library",
        icon: Lightbulb,
        component: ExampleView,
        // A badge is a function of the shared context — here, the number
        // of playlists. Return null to hide it.
        badge: (app: PluginApp) => (app.playlists.length > 0 ? app.playlists.length : null),
      },
    ],
    actions: [
      {
        id: "example.hello",
        labelKey: "Example: hello",
        run: (app: PluginApp) =>
          app.notify(
            "info",
            "Example plugin",
            `Plugin action ran — ${app.files.length} tracks available`
          ),
      },
    ],
  },
};
