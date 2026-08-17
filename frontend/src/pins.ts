import { useCallback, useEffect, useState } from "react";
import { apiFetch } from "@/api";

// ── Pins ────────────────────────────────────────────────────────────────
// Depuis M3, les pins vivent côté serveur (/api/pins, table pins SQLite) :
// ils survivent à un wipe de localStorage et sont visibles depuis un second
// navigateur. Le hook garde un cache optimiste — l'UI réagit avant le réseau.

export type PinKind = "artist" | "album" | "playlist";

export interface Pin {
  kind: PinKind;
  value: string;
}

const STORAGE_KEY = "soneph_pins_v1";

export function pinKey(pin: Pin): string {
  return `${pin.kind}:${pin.value}`;
}

// loadLegacyPins lit l'ancienne persistence localStorage (pré-M3), pour la
// migration one-shot vers le serveur.
function loadLegacyPins(): Pin[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const arr = JSON.parse(raw);
    if (!Array.isArray(arr)) return [];
    return arr.filter(
      (p): p is Pin =>
        !!p &&
        (p.kind === "artist" || p.kind === "album" || p.kind === "playlist") &&
        typeof p.value === "string"
    );
  } catch {
    return [];
  }
}

export function usePins() {
  const [pins, setPins] = useState<Pin[]>([]);

  // Hydrate depuis le serveur (source de vérité). Si le serveur est vide
  // mais que l'ancien localStorage contient des pins, on les y migre une
  // fois, puis on supprime la clé locale.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const res = await apiFetch("/api/pins");
        if (!res.ok) return;
        const data = await res.json();
        let serverPins: Pin[] = (data.pins || []).map((p: { kind: string; value: string }) => ({
          kind: p.kind as PinKind,
          value: p.value,
        }));
        const legacy = loadLegacyPins();
        if (serverPins.length === 0 && legacy.length > 0) {
          for (const pin of legacy) {
            try {
              await apiFetch("/api/pins", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(pin),
              });
            } catch {
              // réseau KO pendant la migration : on garde la copie locale
            }
          }
          serverPins = legacy;
          try {
            window.localStorage.removeItem(STORAGE_KEY);
          } catch {
            // storage indisponible — sans effet
          }
        }
        if (!cancelled) setPins(serverPins);
      } catch {
        // Hors-ligne : les pins restent vides jusqu'au prochain hydrate.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const togglePin = useCallback(
    (pin: Pin) => {
      const willPin = !pins.some((p) => p.kind === pin.kind && p.value === pin.value);
      // Optimiste : on met à jour le cache immédiatement.
      setPins((prev) =>
        willPin
          ? [...prev, pin]
          : prev.filter((p) => !(p.kind === pin.kind && p.value === pin.value))
      );
      // Le serveur ratrappe ; en cas d'échec, le prochain hydrate corrige.
      void (async () => {
        try {
          if (willPin) {
            await apiFetch("/api/pins", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify(pin),
            });
          } else {
            await apiFetch(
              `/api/pins?kind=${encodeURIComponent(pin.kind)}&value=${encodeURIComponent(pin.value)}`,
              { method: "DELETE" }
            );
          }
        } catch {
          // silencieux : le cache optimiste reste, l'état serveur reviendra
        }
      })();
    },
    [pins]
  );

  const isPinned = useCallback(
    (pin: Pin) => pins.some((p) => p.kind === pin.kind && p.value === pin.value),
    [pins]
  );

  return { pins, togglePin, isPinned };
}
