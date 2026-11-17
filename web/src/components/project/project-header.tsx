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

import type { ProjectDetail, ProjectStatus } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const STATUS_BADGE: Record<ProjectStatus, "running" | "paused" | "blocked" | "done"> = {
  active: "running",
  stopped: "paused",
  blocked: "blocked",
  completed: "done",
};

const REPLAY_SPEEDS = [
  { value: "700", key: "project.replay.fast" },
  { value: "1100", key: "project.replay.normal" },
  { value: "1600", key: "project.replay.slow" },
];

export interface ProjectHeaderProps {
  project: ProjectDetail;
  replay: { active: boolean; playing: boolean; stepMs: string; frameIndex: number; frames: unknown[] };
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
      <Button variant="ghost" size="icon-sm" asChild>
        <Link to="/" title={t("project.back")}>
          <ChevronLeft className="size-4" />
        </Link>
      </Button>
      <span className="font-mono text-[11px] text-muted-foreground">{project.project.id}</span>
      <h1 className="max-w-80 truncate text-sm font-semibold">{project.project.title}</h1>
      <Button
        variant="ghost"
        size="icon-sm"
        title={t("project.dialogs.rename.title")}
        onClick={onRename}
        disabled={replay.active}
      >
        <Pencil className="size-3.5" />
      </Button>
      <Badge variant={STATUS_BADGE[status]}>{t(`common.status.${status}`)}</Badge>
      <span className="ml-2 text-xs text-muted-foreground">
        {t("project.facts", { count: project.facts.length })} ·{" "}
        {t("project.intents", { count: project.intents.length })}
      </span>
      {status !== "completed" && !replay.active && (
        <Button
          variant="ghost"
          size="icon-sm"
          title={t("project.maxSteps.edit")}
          onClick={onEditMaxSteps}
        >
          <Gauge className="size-3.5" />
        </Button>
      )}
      {maxSteps !== null && (
        <span
          className={cn(
            "text-xs tabular-nums",
            project.intents.length >= maxSteps
              ? "font-medium text-rose-600"
              : "text-muted-foreground",
          )}
        >
          {t("project.maxSteps.progress", { count: project.intents.length, max: maxSteps })}
        </span>
      )}
      {status === "blocked" && (
        <span className="rounded bg-rose-50 px-1.5 py-0.5 text-[11px] text-rose-700">
          {t("project.maxSteps.blocked")}
        </span>
      )}
      <div className="flex-1" />

      <ReplayControls replay={replay} onStart={onStartReplay} onToggle={onToggleReplay} onRestart={onRestartReplay} onSpeed={onSpeedChange} onExit={onExitReplay} />

      <Button variant="outline" size="sm" onClick={onSnapshot}>
        <FileCode2 className="size-3.5" />
        {t("project.snapshot")}
      </Button>
      <StatusActions status={status} replayActive={replay.active} onToggleStatus={onToggleStatus} onReopen={onReopen} onDelete={onDelete} />
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

  if (!replay.active) {
    return (
      <Button variant="outline" size="sm" onClick={onStart}>
        <Play className="size-3.5" />
        {t("project.replay.start")}
      </Button>
    );
  }

  return (
    <div className="flex items-center gap-1 rounded-lg border border-violet-200 bg-violet-50 px-2 py-1 text-xs text-violet-700">
      <span className="font-medium">
        {t("project.replay.label", {
          current: Math.min(replay.frameIndex + 1, replay.frames.length),
          total: replay.frames.length,
        })}
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        title={t(replay.playing ? "project.replay.pause" : "project.replay.resume")}
        onClick={onToggle}
      >
        {replay.playing ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
      </Button>
      <Button variant="ghost" size="icon-sm" title={t("project.replay.restart")} onClick={onRestart}>
        <RotateCcw className="size-3.5" />
      </Button>
      <Select value={replay.stepMs} onValueChange={onSpeed}>
        <SelectTrigger className="h-6 w-24 border-violet-200 bg-white text-[11px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {REPLAY_SPEEDS.map((speed) => (
            <SelectItem key={speed.value} value={speed.value}>
              {t(speed.key)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button variant="ghost" size="sm" onClick={onExit}>
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
      <Button variant="outline" size="sm" className="text-destructive" onClick={onDelete}>
        <Trash2 className="size-3.5" />
        {t("common.delete")}
      </Button>
    </>
  );
}
