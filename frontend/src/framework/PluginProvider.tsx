import React, { createContext, useContext, useState } from "react";
import { defaultPluginIds, pluginById } from "./pluginRegistry";

// Persisted plugin state — replaces the old hardcoded module catalog.
// The storage key is kept from the previous version so existing users
// keep their choices.

const STORAGE_KEY = "soneph_modules_v1";

interface PluginsValue {
  enabled: Set<string>;
  configured: boolean;
  isEnabled: (id: string) => boolean;
  toggle: (id: string) => void;
  finishOnboarding: (ids: string[]) => void;
}

const PluginsContext = createContext<PluginsValue>({
  enabled: new Set(),
  configured: true,
  isEnabled: () => true,
  toggle: () => {},
  finishOnboarding: () => {},
});

export function PluginsProvider({ children }: { children: React.ReactNode }) {
  const [enabled, setEnabled] = useState<Set<string>>(() => {
    if (typeof window === "undefined") return new Set(defaultPluginIds());
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY);
      if (!raw) return new Set(defaultPluginIds());
      const arr = JSON.parse(raw);
      const valid = arr.filter((id: unknown) => pluginById(String(id)));
      return new Set(valid);
    } catch {
      return new Set(defaultPluginIds());
    }
  });

  const [configured, setConfigured] = useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    return window.localStorage.getItem(STORAGE_KEY) !== null;
  });

  const persist = (ids: Set<string>) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]));
    } catch {
      // storage unavailable (private mode…) — plugins stay in-memory
    }
  };

  const toggle = (id: string) => {
    setEnabled((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      persist(next);
      return next;
    });
  };

  const finishOnboarding = (ids: string[]) => {
    const next = new Set(ids);
    persist(next);
    setEnabled(next);
    setConfigured(true);
  };

  const isEnabled = (id: string) => enabled.has(id);

  return (
    <PluginsContext.Provider value={{ enabled, configured, isEnabled, toggle, finishOnboarding }}>
      {children}
    </PluginsContext.Provider>
  );
}

export function usePlugins() {
  return useContext(PluginsContext);
}
