import { useMemo } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  ChevronLeft,
  FileCode2,
  Gauge,
  Pause,
  Pencil,
  Play,
  RotateCcw,
  Square,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipPopup, TooltipTrigger } from "@/components/ui/tooltip";
import type { ProjectDetail, ProjectStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

const STATUS_BADGE: Record<ProjectStatus, "info" | "warning" | "error" | "success"> = {
  active: "info",
  stopped: "warning",
  blocked: "error",
  completed: "success",
};

const REPLAY_SPEEDS = [
  { value: "700", key: "project.replay.fast" },
  { value: "1100", key: "project.replay.normal" },
  { value: "1600", key: "project.replay.slow" },
];

export interface ProjectHeaderProps {
  project: ProjectDetail;
  replay: {
    active: boolean;
    playing: boolean;
    stepMs: string;
    frameIndex: number;
    frames: unknown[];
  };
  onRename: () => void;
  onEditMaxSteps: () => void;
  onStartReplay: () => void;
  onToggleReplay: () => void;
  onRestartReplay: () => void;
  onSpeedChange: (value: string) => void;
  onExitReplay: () => void;
  onSnapshot: () => void;
  onToggleStatus: () => void;
  onReopen: () => void;
  onDelete: () => void;
}

export function ProjectHeader({
  project,
  replay,
  onRename,
  onEditMaxSteps,
  onStartReplay,
  onToggleReplay,
  onRestartReplay,
  onSpeedChange,
  onExitReplay,
  onSnapshot,
  onToggleStatus,
  onReopen,
  onDelete,
}: ProjectHeaderProps) {
  const { t } = useTranslation();
  const status = project.project.status;
  const maxSteps = project.project.max_steps;

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b bg-background px-3">
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t("project.back")}
              render={<Link to="/" title={t("project.back")} />}
            />
          }
        >
          <ChevronLeft className="size-4" />
        </TooltipTrigger>
        <TooltipPopup>{t("project.back")}</TooltipPopup>
      </Tooltip>

      <span className="font-mono text-xs text-muted-foreground">{project.project.id}</span>
      <h1 className="max-w-72 truncate font-heading text-sm font-semibold tracking-tight text-foreground">
        {project.project.title}
      </h1>

      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-xs"
              title={t("project.dialogs.rename.title")}
              aria-label={t("project.dialogs.rename.title")}
              onClick={onRename}
              disabled={replay.active}
            />
          }
        >
          <Pencil className="size-3.5" />
        </TooltipTrigger>
        <TooltipPopup>{t("project.dialogs.rename.title")}</TooltipPopup>
      </Tooltip>

      <Badge variant={STATUS_BADGE[status]}>
        <span
          aria-hidden="true"
          className={cn(
            "size-1.5 rounded-full bg-current",
            status === "active" && "pulse-soft",
          )}
        />
        {t(`common.status.${status}`)}
      </Badge>

      <Separator orientation="vertical" className="mx-1 h-4" />

      <span className="text-xs tabular-nums text-muted-foreground">
        {t("project.facts", { count: project.facts.length })} ·{" "}
        {t("project.intents", { count: project.intents.length })}
      </span>

      {status !== "completed" && !replay.active && (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-xs"
                title={t("project.maxSteps.edit")}
                aria-label={t("project.maxSteps.edit")}
                onClick={onEditMaxSteps}
              />
            }
          >
            <Gauge className="size-3.5" />
          </TooltipTrigger>
          <TooltipPopup>{t("project.maxSteps.edit")}</TooltipPopup>
        </Tooltip>
      )}

      {maxSteps !== null && (
        <span
          className={cn(
            "text-xs tabular-nums",
            project.intents.length >= maxSteps
              ? "font-medium text-destructive-foreground"
              : "text-muted-foreground",
          )}
        >
          {t("project.maxSteps.progress", { count: project.intents.length, max: maxSteps })}
        </span>
      )}

      {status === "blocked" && (
        <Badge variant="error" size="sm">
          {t("project.maxSteps.blocked")}
        </Badge>
      )}

      <div className="flex-1" />

      <ReplayControls
        replay={replay}
        onStart={onStartReplay}
        onToggle={onToggleReplay}
        onRestart={onRestartReplay}
        onSpeed={onSpeedChange}
        onExit={onExitReplay}
      />

      <Button variant="outline" size="sm" onClick={onSnapshot}>
        <FileCode2 className="size-3.5" />
        {t("project.snapshot")}
      </Button>

      <StatusActions
        status={status}
        replayActive={replay.active}
        onToggleStatus={onToggleStatus}
        onReopen={onReopen}
        onDelete={onDelete}
      />
    </header>
  );
}

function ReplayControls({
  replay,
  onStart,
  onToggle,
  onRestart,
  onSpeed,
  onExit,
}: {
  replay: ProjectHeaderProps["replay"];
  onStart: () => void;
  onToggle: () => void;
  onRestart: () => void;
  onSpeed: (value: string) => void;
  onExit: () => void;
}) {
  const { t } = useTranslation();

  const speedItems = useMemo(
    () => REPLAY_SPEEDS.map((speed) => ({ value: speed.value, label: t(speed.key) })),
    [t],
  );

  if (!replay.active) {
    return (
      <Button variant="outline" size="sm" onClick={onStart}>
        <Play className="size-3.5" />
        {t("project.replay.start")}
      </Button>
    );
  }

  return (
    <div className="flex items-center gap-1 rounded-lg border border-info/25 bg-info/8 px-2 py-0.5 text-xs text-info-foreground dark:bg-info/16">
      <span className="px-1 font-medium tabular-nums">
        {t("project.replay.label", {
          current: Math.min(replay.frameIndex + 1, replay.frames.length),
          total: replay.frames.length,
        })}
      </span>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-xs"
              title={t(replay.playing ? "project.replay.pause" : "project.replay.resume")}
              aria-label={t(replay.playing ? "project.replay.pause" : "project.replay.resume")}
              onClick={onToggle}
            />
          }
        >
          {replay.playing ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
        </TooltipTrigger>
        <TooltipPopup>
          {t(replay.playing ? "project.replay.pause" : "project.replay.resume")}
        </TooltipPopup>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-xs"
              title={t("project.replay.restart")}
              aria-label={t("project.replay.restart")}
              onClick={onRestart}
            />
          }
        >
          <RotateCcw className="size-3.5" />
        </TooltipTrigger>
        <TooltipPopup>{t("project.replay.restart")}</TooltipPopup>
      </Tooltip>
      <div className="w-24">
        <Select
          items={speedItems}
          value={replay.stepMs}
          onValueChange={(value) => {
            if (value) onSpeed(value);
          }}
        >
          <SelectTrigger size="sm" className="min-h-6 text-xs sm:min-h-6">
            <SelectValue />
          </SelectTrigger>
          <SelectPopup>
            {speedItems.map((speed) => (
              <SelectItem key={speed.value} value={speed.value}>
                {speed.label}
              </SelectItem>
            ))}
          </SelectPopup>
        </Select>
      </div>
      <Button variant="ghost" size="xs" onClick={onExit}>
        {t("project.replay.exit")}
      </Button>
    </div>
  );
}

function StatusActions({
  status,
  replayActive,
  onToggleStatus,
  onReopen,
  onDelete,
}: {
  status: ProjectStatus;
  replayActive: boolean;
  onToggleStatus: () => void;
  onReopen: () => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  if (replayActive) return null;

  return (
    <>
      {status !== "completed" && (
        <Button variant="outline" size="sm" onClick={onToggleStatus}>
          {status === "active" ? <Square className="size-3.5" /> : <Play className="size-3.5" />}
          {t(status === "active" ? "project.stop" : "project.resume")}
        </Button>
      )}
      {status === "completed" && (
        <Button variant="outline" size="sm" onClick={onReopen}>
          <RotateCcw className="size-3.5" />
          {t("project.reopen")}
        </Button>
      )}
      <Button variant="destructive-outline" size="sm" onClick={onDelete}>
        <Trash2 className="size-3.5" />
        {t("common.delete")}
      </Button>
    </>
  );
}
