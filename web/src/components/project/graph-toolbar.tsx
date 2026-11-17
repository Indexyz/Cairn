import { useTranslation } from "react-i18next";
import { CircleCheck, Lightbulb, Maximize2, Sparkles } from "lucide-react";

import { LAYOUT_MODES, type LayoutMode } from "@/lib/format";
import type { ProjectDetail } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface GraphToolbarProps {
  reason: ProjectDetail["project"]["reason"];
  heartbeatLabel: string;
  layoutMode: LayoutMode;
  onLayoutChange: (mode: LayoutMode) => void;
  onFit: () => void;
  isRunnable: boolean;
  isActive: boolean;
  canActOnFacts: boolean;
  canWriteHints: boolean;
  hasActionableIntent: boolean;
  intentClaimed: boolean;
  hasReleasableIntent: boolean;
  onIntent: () => void;
  onComplete: () => void;
  onHint: () => void;
  onHeartbeat: () => void;
  onRelease: () => void;
  onConclude: () => void;
}

export function GraphToolbar({
  reason,
  heartbeatLabel,
  layoutMode,
  onLayoutChange,
  onFit,
  isRunnable,
  isActive,
  canActOnFacts,
  canWriteHints,
  hasActionableIntent,
  intentClaimed,
  hasReleasableIntent,
  onIntent,
  onComplete,
  onHint,
  onHeartbeat,
  onRelease,
  onConclude,
}: GraphToolbarProps) {
  const { t } = useTranslation();

  return (
    <>
      <div className="absolute top-3 left-3 z-10 flex items-center gap-1.5">
        <Button variant="outline" size="icon-sm" className="bg-background/90" onClick={onFit}>
          <Maximize2 className="size-4" />
        </Button>
        <Select value={layoutMode} onValueChange={(value) => onLayoutChange(value as LayoutMode)}>
          <SelectTrigger className="h-8 w-28 bg-background/90 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {LAYOUT_MODES.map((mode) => (
              <SelectItem key={mode} value={mode}>
                {t(`layouts.${mode}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {reason && (
        <div className="absolute top-3 right-3 z-10 w-72 rounded-xl border border-sky-200 bg-background/95 p-3 shadow-lg">
          <div className="flex items-start gap-2">
            <span className="pulse-soft mt-1 size-2 rounded-full bg-sky-500" />
            <div className="min-w-0">
              <p className="text-[10px] font-semibold tracking-widest text-sky-600 uppercase">
                Reason running
              </p>
              <p className="truncate text-sm font-semibold">{reason.worker}</p>
              <p className="text-[11px] text-muted-foreground">
                {t("project.heartbeat", { time: heartbeatLabel })}
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="absolute top-14 left-3 z-10 flex flex-col gap-2">
        <div className="flex gap-1.5">
          {isRunnable && (
            <>
              {isActive && (
                <Button
                  size="sm"
                  variant="outline"
                  className="bg-background/90"
                  disabled={!canActOnFacts}
                  onClick={onIntent}
                >
                  <Sparkles className="size-3.5" />
                  {t("project.actions.intent")}
                </Button>
              )}
              <Button
                size="sm"
                variant="outline"
                className="bg-background/90"
                disabled={!canActOnFacts}
                onClick={onComplete}
              >
                <CircleCheck className="size-3.5" />
                {t("project.actions.complete")}
              </Button>
            </>
          )}
          {canWriteHints && (
            <Button size="sm" variant="outline" className="bg-background/90" onClick={onHint}>
              <Lightbulb className="size-3.5" />
              {t("project.actions.hint")}
            </Button>
          )}
        </div>
        {isRunnable && (
          <div className="flex gap-1.5">
            <Button
              size="sm"
              variant="outline"
              className="bg-background/90"
              disabled={!hasActionableIntent}
              onClick={onHeartbeat}
            >
              {t(intentClaimed ? "project.actions.heartbeat" : "project.actions.claim")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="bg-background/90"
              disabled={!hasReleasableIntent}
              onClick={onRelease}
            >
              {t("project.actions.release")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="bg-background/90"
              disabled={!hasActionableIntent}
              onClick={onConclude}
            >
              {t("project.actions.conclude")}
            </Button>
          </div>
        )}
      </div>
    </>
  );
}
