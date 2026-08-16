import type { PluginManifest, PluginViewContribution } from "./plugin.types";

// ── Plugin registry ──────────────────────────────────────────────────────
// Standalone catalog: plugin manifests register themselves here (see
// src/plugins/index.ts) and the rest of the app reads from it. Keeping this
// module free of component imports avoids import cycles with the provider.

const plugins: PluginManifest[] = [];

export function registerPlugin(manifest: PluginManifest): void {
  if (!plugins.some((p) => p.id === manifest.id)) plugins.push(manifest);
}

export const getPlugins = (): PluginManifest[] => [...plugins];

export const pluginById = (id: string): PluginManifest | undefined =>
  plugins.find((p) => p.id === id);

/** Plugins the user can enable/disable (everything that isn't core). */
export const toggleablePlugins = (): PluginManifest[] => plugins.filter((p) => !p.core);

export const pluginViews = (): PluginViewContribution[] =>
  plugins.flatMap((p) => p.contributes.views ?? []);

export const viewById = (id: string): PluginViewContribution | undefined =>
  pluginViews().find((v) => v.id === id);

export const pluginForView = (viewId: string): PluginManifest | undefined =>
  plugins.find((p) => (p.contributes.views ?? []).some((v) => v.id === viewId));

/** Plugins pre-selected for a first launch. */
export const defaultPluginIds = (): string[] =>
  plugins.filter((p) => p.defaultEnabled).map((p) => p.id);
