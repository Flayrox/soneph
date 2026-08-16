import { useState } from "react";

// ── Pins ────────────────────────────────────────────────────────────────
// Users can pin artists and albums so they appear in the sidebar and on the
// Home view. Persisted in localStorage — no backend round-trip needed.

export type PinKind = "artist" | "album" | "playlist";

export interface Pin {
  kind: PinKind;
  value: string;
}

const STORAGE_KEY = "soneph_pins_v1";

export function pinKey(pin: Pin): string {
  return `${pin.kind}:${pin.value}`;
}

function loadPins(): Pin[] {
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

function persist(pins: Pin[]) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(pins));
  } catch {
    // storage unavailable — pins stay in-memory
  }
}

export function usePins() {
  const [pins, setPins] = useState<Pin[]>(() => loadPins());

  const togglePin = (pin: Pin) => {
    setPins((prev) => {
      const next = prev.some((p) => p.kind === pin.kind && p.value === pin.value)
        ? prev.filter((p) => !(p.kind === pin.kind && p.value === pin.value))
        : [...prev, pin];
      persist(next);
      return next;
    });
  };

  const isPinned = (pin: Pin) =>
    pins.some((p) => p.kind === pin.kind && p.value === pin.value);

  return { pins, togglePin, isPinned };
}
