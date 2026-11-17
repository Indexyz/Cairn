import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { FileCode2, GitFork, Pencil, RotateCcw, Square, Trash2 } from "lucide-react";

import { useFormat } from "@/lib/format";
import type { ProjectStatus, ProjectSummary } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TableCell, TableRow } from "@/components/ui/table";

const STATUS_BADGE: Record<ProjectStatus, "running" | "paused" | "blocked" | "done"> = {
  active: "running",
  stopped: "paused",
  blocked: "blocked",
  completed: "done",
};

interface ProjectRowProps {
  project: ProjectSummary;
  inGroup: boolean;
  onRename: (project: ProjectSummary) => void;
  onFork: (project: ProjectSummary) => void;
  onSnapshot: (project: ProjectSummary) => void;
  onStatus: (project: ProjectSummary, status: ProjectStatus) => void;
  onDelete: (project: ProjectSummary) => void;
}

export function ProjectRow({ project, inGroup, onRename, onFork, onSnapshot, onStatus, onDelete }: ProjectRowProps) {
  const { t } = useTranslation();
  const format = useFormat();

  return (
      <TableRow key={project.id}>
        <TableCell className={cn("font-mono text-xs text-muted-foreground", inGroup && "pl-9")}>
          {project.id}
        </TableCell>
        <TableCell className="max-w-72">
          <Link
            to={`/projects/${project.id}`}
            className="block truncate font-medium hover:underline"
            title={project.title}
          >
            {project.title}
          </Link>
          {project.reason && (
            <p className="truncate text-[11px] text-sky-600">
              {t("projects.reasonRunning", {
                worker: project.reason.worker,
                time: format.time(project.reason.last_heartbeat_at),
              })}
            </p>
          )}
        </TableCell>
        <TableCell>
          <Badge variant={STATUS_BADGE[project.status]}>{t(`common.status.${project.status}`)}</Badge>
        </TableCell>
        <TableCell className="text-right tabular-nums">{project.fact_count}</TableCell>
        <TableCell className="text-right tabular-nums">
          {project.intent_count}
          {project.max_steps !== null && <span className="text-muted-foreground"> / {project.max_steps}</span>}
          {project.working_intent_count > 0 && (
            <span className="ml-1 text-amber-600">+{project.working_intent_count}</span>
          )}
        </TableCell>
        <TableCell className="text-right tabular-nums">{project.hint_count}</TableCell>
        <TableCell className="text-xs text-muted-foreground">{format.date(project.created_at)}</TableCell>
        <TableCell>
          <div className="flex items-center justify-end gap-1">
            <Button variant="outline" size="sm" onClick={() => void onSnapshot(project)}>
              <FileCode2 className="size-3.5" />
              {t("projects.snapshot")}
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              title={t("common.edit")}
              onClick={() => onRename(project)}
            >
              <Pencil className="size-3.5" />
            </Button>
            {project.status === "active" && (
              <Button
                variant="ghost"
                size="icon-sm"
                title={t("project.stop")}
                onClick={() => void onStatus(project, "stopped")}
              >
                <Square className="size-3.5" />
              </Button>
            )}
            {(project.status === "stopped" || project.status === "blocked") && (
              <Button
                variant="ghost"
                size="icon-sm"
                title={t("project.resume")}
                onClick={() => void onStatus(project, "active")}
              >
                <RotateCcw className="size-3.5" />
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon-sm"
              title={t("projects.fork.action")}
              onClick={() => {
                onFork(project);
              }}
            >
              <GitFork className="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              className="text-destructive hover:text-destructive"
              title={t("common.delete")}
              onClick={() => onDelete(project)}
            >
              <Trash2 className="size-3.5" />
            </Button>
          </div>
        </TableCell>
      </TableRow>
    );
  }
