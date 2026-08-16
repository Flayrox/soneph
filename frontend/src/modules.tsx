import React, { createContext, useContext, useState } from "react";
import { DownloadCloud, BarChart3, type LucideIcon } from "lucide-react";

// ── Module catalog ────────────────────────────────────────────────────────
// A module is a first-class feature the user can enable/disable from the
// Marketplace (or the first-launch picker). The catalog lives here; the
// backend endpoints stay available regardless (the UI gates what's shown).
export interface ModuleDef {
  id: string;
  nameKey: string;
  descKey: string;
  icon: LucideIcon;
  defaultEnabled: boolean;
}

export const MODULES: ModuleDef[] = [
  {
    id: "import",
    nameKey: "Import Module",
    descKey: "Import Module Desc",
    icon: DownloadCloud,
    defaultEnabled: true,
  },
  {
    id: "stats",
    nameKey: "Stats Module",
    descKey: "Stats Module Desc",
    icon: BarChart3,
    defaultEnabled: true,
  },
];

export const moduleById = (id: string) => MODULES.find((m) => m.id === id);

const STORAGE_KEY = "soneph_modules_v1";

export function defaultModules(): string[] {
  return MODULES.filter((m) => m.defaultEnabled).map((m) => m.id);
}

interface ModulesValue {
  enabled: Set<string>;
  configured: boolean;
  isEnabled: (id: string) => boolean;
  toggle: (id: string) => void;
  finishOnboarding: (ids: string[]) => void;
}

const ModulesContext = createContext<ModulesValue>({
  enabled: new Set(defaultModules()),
  configured: true,
  isEnabled: () => true,
  toggle: () => {},
  finishOnboarding: () => {},
});

export function ModulesProvider({ children }: { children: React.ReactNode }) {
  const [enabled, setEnabled] = useState<Set<string>>(() => {
    if (typeof window === "undefined") return new Set(defaultModules());
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY);
      if (!raw) return new Set(defaultModules());
      const arr = JSON.parse(raw);
      const valid = arr.filter((id: unknown) => moduleById(String(id)));
      return new Set(valid);
    } catch {
      return new Set(defaultModules());
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
      // storage unavailable (private mode…) — modules stay in-memory
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
    <ModulesContext.Provider value={{ enabled, configured, isEnabled, toggle, finishOnboarding }}>
      {children}
    </ModulesContext.Provider>
  );
}

export function useModules() {
  return useContext(ModulesContext);
}
