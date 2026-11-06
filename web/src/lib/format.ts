export function formatTime(ts: string | null | undefined): string {
  if (!ts) return "";
  return new Date(ts).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatDate(ts: string | null | undefined): string {
  if (!ts) return "";
  const date = new Date(ts);
  return `${date.toLocaleDateString([], {
    year: "numeric",
    month: "short",
    day: "numeric",
  })} ${date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
}

export function formatTimelineDate(ts: string | null | undefined): string {
  if (!ts) return "";
  return new Date(ts).toLocaleDateString([], {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export function formatDurationMs(ms: number): string {
  const totalSeconds = Math.max(0, Math.round(ms / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

export const LAYOUT_MODES = [
  "dagre_tb",
  "dagre_lr",
  "klay_tb",
  "klay_lr",
  "elk_tb",
  "elk_lr",
] as const;

export type LayoutMode = (typeof LAYOUT_MODES)[number];

export function isValidLayoutMode(mode: string): mode is LayoutMode {
  return (LAYOUT_MODES as readonly string[]).includes(mode);
}
