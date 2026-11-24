import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck, Lightbulb, Maximize2, Sparkles } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tooltip, TooltipPopup, TooltipTrigger } from "@/components/ui/tooltip";
import { LAYOUT_MODES, type LayoutMode } from "@/lib/format";
import type { ProjectDetail } from "@/lib/types";

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

  const layoutItems = useMemo(
    () => LAYOUT_MODES.map((mode) => ({ value: mode, label: t(`layouts.${mode}`) })),
    [t],
  );

  return (
    <>
      <div className="absolute top-3 left-3 z-10 flex items-center gap-1.5">
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="outline"
                size="icon-sm"
                className="bg-background/95 backdrop-blur-xs"
                aria-label={t("project.fit")}
                title={t("project.fit")}
                onClick={onFit}
              />
            }
          >
            <Maximize2 className="size-3.5" />
          </TooltipTrigger>
          <TooltipPopup>{t("project.fit")}</TooltipPopup>
        </Tooltip>

        <div className="w-30">
          <Select
            items={layoutItems}
            value={layoutMode}
            onValueChange={(value) => {
              if (value) onLayoutChange(value as LayoutMode);
            }}
          >
            <SelectTrigger
              size="sm"
              className="bg-background/95 backdrop-blur-xs"
              aria-label={t("settings.defaultLayout")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectPopup>
              {layoutItems.map((mode) => (
                <SelectItem key={mode.value} value={mode.value}>
                  {mode.label}
                </SelectItem>
              ))}
            </SelectPopup>
          </Select>
        </div>
      </div>

      {reason && (
        <div className="absolute top-3 right-3 z-10 w-72 rounded-xl border border-info/25 bg-popover/95 p-3 shadow-md/5 backdrop-blur-sm">
          <div className="flex items-start gap-2.5">
            <span className="pulse-soft mt-1.5 size-2 shrink-0 rounded-full bg-info" />
            <div className="min-w-0">
              <p className="text-[11px] font-semibold text-info-foreground">
                {t("project.reasonRunning")}
              </p>
              <p className="truncate text-sm font-medium text-popover-foreground">
                {reason.worker}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("project.heartbeat", { time: heartbeatLabel })}
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="absolute top-13 left-3 z-10 flex flex-col gap-1.5">
        {(isRunnable || canWriteHints) && (
          <div className="flex flex-wrap gap-1.5">
            {isRunnable && (
              <>
                {isActive && (
                  <Button
                    size="xs"
                    variant="outline"
                    className="bg-background/95 backdrop-blur-xs"
                    disabled={!canActOnFacts}
                    onClick={onIntent}
                  >
                    <Sparkles className="size-3.5" />
                    {t("project.actions.intent")}
                  </Button>
                )}
                <Button
                  size="xs"
                  variant="outline"
                  className="bg-background/95 backdrop-blur-xs"
                  disabled={!canActOnFacts}
                  onClick={onComplete}
                >
                  <CircleCheck className="size-3.5" />
                  {t("project.actions.complete")}
                </Button>
              </>
            )}
            {canWriteHints && (
              <Button
                size="xs"
                variant="outline"
                className="bg-background/95 backdrop-blur-xs"
                onClick={onHint}
              >
                <Lightbulb className="size-3.5" />
                {t("project.actions.hint")}
              </Button>
            )}
          </div>
        )}
        {isRunnable && (
          <div className="flex flex-wrap gap-1.5">
            <Button
              size="xs"
              variant="outline"
              className="bg-background/95 backdrop-blur-xs"
              disabled={!hasActionableIntent}
              onClick={onHeartbeat}
            >
              {t(intentClaimed ? "project.actions.heartbeat" : "project.actions.claim")}
            </Button>
            <Button
              size="xs"
              variant="outline"
              className="bg-background/95 backdrop-blur-xs"
              disabled={!hasReleasableIntent}
              onClick={onRelease}
            >
              {t("project.actions.release")}
            </Button>
            <Button
              size="xs"
              variant="outline"
              className="bg-background/95 backdrop-blur-xs"
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
