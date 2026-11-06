import * as React from "react";

import { isValidLayoutMode, type LayoutMode } from "@/lib/format";

const PREFS_KEY = "cairn.localPrefs";
const PANEL_KEY = "cairn.sidePanelWidth";

export interface LocalPrefs {
  actor_name: string;
  layout_mode: LayoutMode;
}

const DEFAULT_PREFS: LocalPrefs = { actor_name: "Human", layout_mode: "dagre_tb" };

function readPrefs(): LocalPrefs {
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    if (!raw) return DEFAULT_PREFS;
    const parsed = JSON.parse(raw) as Partial<LocalPrefs> & { layout_dir?: string };
    return {
      actor_name: typeof parsed.actor_name === "string" && parsed.actor_name.trim() ? parsed.actor_name : "Human",
      layout_mode: isValidLayoutMode(parsed.layout_mode ?? "")
        ? (parsed.layout_mode as LayoutMode)
        : parsed.layout_dir === "LR"
          ? "dagre_lr"
          : "dagre_tb",
    };
  } catch {
    return DEFAULT_PREFS;
  }
}

export function useLocalPrefs() {
  const [prefs, setPrefs] = React.useState<LocalPrefs>(readPrefs);

  const save = React.useCallback((next: LocalPrefs) => {
    const normalized: LocalPrefs = {
      actor_name: next.actor_name.trim() || "Human",
      layout_mode: isValidLayoutMode(next.layout_mode) ? next.layout_mode : "dagre_tb",
    };
    setPrefs(normalized);
    try {
      localStorage.setItem(PREFS_KEY, JSON.stringify(normalized));
    } catch {
      /* storage unavailable */
    }
  }, []);

  const actorName = prefs.actor_name.trim() || "Human";

  return { prefs, save, actorName };
}

export function readPanelWidth(): number | null {
  try {
    const raw = localStorage.getItem(PANEL_KEY);
    if (raw === null) return null;
    const value = Number(raw);
    return Number.isFinite(value) ? value : null;
  } catch {
    return null;
  }
}

export function savePanelWidth(width: number) {
  try {
    localStorage.setItem(PANEL_KEY, String(width));
  } catch {
    /* storage unavailable */
  }
}
