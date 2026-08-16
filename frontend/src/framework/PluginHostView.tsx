import React from "react";
import { viewById } from "./pluginRegistry";
import type { PluginApp } from "./plugin.types";

interface PluginHostViewProps {
  viewId: string;
  app: PluginApp;
}

/**
 * Mount point for plugin-contributed views. Looks the view up in the
 * registry and renders it with the host's shared `app` context.
 */
export const PluginHostView: React.FC<PluginHostViewProps> = ({ viewId, app }) => {
  const view = viewById(viewId);
  if (!view || !view.component) return null;
  const Component = view.component;
  return <Component app={app} />;
};
