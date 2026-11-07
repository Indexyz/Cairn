import { useMemo } from "react";
import { useTranslation } from "react-i18next";

export { LAYOUT_MODES, isValidLayoutMode } from "@/lib/format-shared";
export type { LayoutMode, LayoutMode as LayoutModeType } from "@/lib/format-shared";

function bcp47(locale: string | undefined): string | undefined {
  return locale ? locale.replace("_", "-") : undefined;
}

export function formatTime(ts: string | null | undefined, locale?: string): string {
  if (!ts) return "";
  return new Date(ts).toLocaleTimeString(bcp47(locale), {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatDate(ts: string | null | undefined, locale?: string): string {
  if (!ts) return "";
  const date = new Date(ts);
  return `${date.toLocaleDateString(bcp47(locale), {
    year: "numeric",
    month: "short",
    day: "numeric",
  })} ${date.toLocaleTimeString(bcp47(locale), { hour: "2-digit", minute: "2-digit" })}`;
}

export function formatTimelineDate(ts: string | null | undefined, locale?: string): string {
  if (!ts) return "";
  return new Date(ts).toLocaleDateString(bcp47(locale), {
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

export function useFormat() {
  const { i18n } = useTranslation();
  const locale = i18n.resolvedLanguage ?? i18n.language;
  return useMemo(
    () => ({
      locale,
      time: (ts: string | null | undefined) => formatTime(ts, locale),
      date: (ts: string | null | undefined) => formatDate(ts, locale),
      timelineDate: (ts: string | null | undefined) => formatTimelineDate(ts, locale),
      duration: formatDurationMs,
    }),
    [locale],
  );
}
