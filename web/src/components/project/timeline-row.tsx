import { memo } from "react";
import { useTranslation } from "react-i18next";

import type { TimelineEvent } from "@/lib/types";
import {
  timelineEventBadge,
  timelineEventBadgeClass,
  timelineEventDotClass,
  timelineEventIsInteractive,
} from "@/lib/timeline";
import { cn } from "@/lib/utils";

export const TimelineRow = memo(function TimelineRow({
  entry,
  selected,
  onOpen,
  time,
  dateLabel,
}: {
  entry: TimelineEvent;
  selected: boolean;
  onOpen: (entry: TimelineEvent) => void;
  time: string;
  dateLabel: string;
}) {
  const { t } = useTranslation();
  return (
    <div id={`timeline-entry-${entry.id}`} className="flex gap-2">
      <div className="flex w-2.5 flex-col items-center">
        <span className={cn("mt-1.5 size-2.5 rounded-full", timelineEventDotClass(entry))} />
        {!entry.isLast && <span className="mt-1 w-px flex-1 bg-border" />}
      </div>
      <button
        type="button"
        onClick={() => onOpen(entry)}
        className={cn(
          "min-w-0 flex-1 rounded-lg px-1 py-1 text-left",
          timelineEventIsInteractive(entry) ? "cursor-pointer hover:bg-muted/50" : "cursor-default",
          selected && "bg-accent/60 ring-1 ring-border",
        )}
      >
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-[11px] text-muted-foreground">{time}</span>
          <span
            className={cn(
              "rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase",
              timelineEventBadgeClass(entry),
            )}
          >
            {t(timelineEventBadge(entry))}
          </span>
          <span className="text-[11px] text-muted-foreground/70">{dateLabel}</span>
        </div>
        <p className="mt-1 text-sm break-words">{entry.title}</p>
        {(entry.actor || entry.meta.length > 0) && (
          <p className="mt-0.5 text-[11px] break-words text-muted-foreground">
            {[entry.actor, ...entry.meta].filter(Boolean).join(" · ")}
          </p>
        )}
      </button>
    </div>
  );
});
