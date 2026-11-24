import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { CalendarClock, Pencil, Play, Plus, Trash2 } from "lucide-react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { ScheduleFormDialog } from "@/components/schedules/schedule-form-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Frame, FrameFooter, FramePanel } from "@/components/ui/frame";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tooltip, TooltipPopup, TooltipTrigger } from "@/components/ui/tooltip";
import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import { useFormat } from "@/lib/format";
import type { Schedule, SchedulePayload, ScheduleRunResult, ScheduleStatus } from "@/lib/types";

const STATUS_VARIANT: Record<ScheduleStatus, "info" | "warning" | "error"> = {
  created: "info",
  skipped: "warning",
  error: "error",
};

export interface ScheduleForm {
  name: string;
  cron: string;
  timezone: string;
  enabled: boolean;
  title: string;
  origin: string;
  goal: string;
  bootstrap: boolean;
  maxSteps: string;
  hints: string;
}

function emptyForm(timezone: string): ScheduleForm {
  return {
    name: "",
    cron: "0 3 * * *",
    timezone,
    enabled: true,
    title: "",
    origin: "",
    goal: "",
    bootstrap: true,
    maxSteps: "",
    hints: "",
  };
}

function toForm(schedule: Schedule): ScheduleForm {
  return {
    name: schedule.name,
    cron: schedule.cron,
    timezone: schedule.timezone,
    enabled: schedule.enabled,
    title: schedule.title,
    origin: schedule.origin,
    goal: schedule.goal,
    bootstrap: schedule.bootstrap_enabled,
    maxSteps: schedule.max_steps === null ? "" : String(schedule.max_steps),
    hints: schedule.hints.join("\n"),
  };
}

function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

export function useSchedulesPage() {
  const toast = useToast();
  const { t } = useTranslation();
  const format = useFormat();
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [form, setForm] = useState<ScheduleForm | null>(null);
  const [editing, setEditing] = useState<Schedule | null>(null);
  const [removal, setRemoval] = useState<Schedule | null>(null);
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState<string | null>(null);

  async function load() {
    try {
      setSchedules(await api<Schedule[]>("GET", "/schedules"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 10000);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(emptyForm(browserTimezone()));
  }

  function openEdit(schedule: Schedule) {
    setEditing(schedule);
    setForm(toForm(schedule));
  }

  async function toggle(schedule: Schedule, enabled: boolean) {
    setSchedules((current) =>
      current.map((item) => (item.id === schedule.id ? { ...item, enabled } : item)),
    );
    try {
      await api("PUT", `/schedules/${schedule.id}`, payloadOf({ ...toForm(schedule), enabled }));
      toast.show(
        t(enabled ? "schedules.toast.enabled" : "schedules.toast.disabled", {
          name: schedule.name,
        }),
        "success",
      );
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      await load();
    }
  }

  function payloadOf(source: ScheduleForm): SchedulePayload {
    return {
      name: source.name.trim(),
      cron: source.cron.trim(),
      timezone: source.timezone.trim() || "UTC",
      enabled: source.enabled,
      title: source.title.trim(),
      origin: source.origin.trim(),
      goal: source.goal.trim(),
      bootstrap_enabled: source.bootstrap,
      max_steps: source.maxSteps.trim() ? Number(source.maxSteps) : null,
      hints: source.hints
        .split("\n")
        .map((hint) => hint.trim())
        .filter(Boolean),
    };
  }

  async function save() {
    if (!form || saving) return;
    setSaving(true);
    try {
      if (editing) await api("PUT", `/schedules/${editing.id}`, payloadOf(form));
      else await api("POST", "/schedules", payloadOf(form));
      setForm(null);
      setEditing(null);
      await load();
      toast.show(t(editing ? "schedules.toast.updated" : "schedules.toast.created"), "success");
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setSaving(false);
    }
  }

  async function run(schedule: Schedule) {
    if (running) return;
    setRunning(schedule.id);
    try {
      const result = await api<ScheduleRunResult>("POST", `/schedules/${schedule.id}/run`);
      if (result.skipped) {
        toast.show(t("schedules.run.skipped", { detail: result.detail }), "warning");
      } else {
        toast.show(t("schedules.run.created", { id: result.project_id }), "success");
      }
      await load();
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setRunning(null);
    }
  }

  async function remove() {
    if (!removal) return;
    try {
      await api("DELETE", `/schedules/${removal.id}`);
      setRemoval(null);
      await load();
      toast.show(t("schedules.toast.deleted", { name: removal.name }), "success");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  const valid = Boolean(
    form &&
      form.name.trim() &&
      form.cron.trim().split(/\s+/).length === 5 &&
      form.title.trim() &&
      form.origin.trim() &&
      form.goal.trim(),
  );

  return {
    editing,
    form,
    format,
    openCreate,
    openEdit,
    removal,
    remove,
    run,
    running,
    save,
    saving,
    schedules,
    setForm,
    setRemoval,
    toggle,
    valid,
  };
}

export function SchedulesPage() {
  const {
    editing,
    form,
    format,
    openCreate,
    openEdit,
    removal,
    remove,
    run,
    running,
    save,
    saving,
    schedules,
    setForm,
    setRemoval,
    toggle,
    valid,
  } = useSchedulesPage();
  const { t } = useTranslation();

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b bg-background px-6 py-4">
        <div className="min-w-0">
          <h1 className="font-heading text-lg font-semibold tracking-tight text-foreground">
            {t("schedules.title")}
          </h1>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("schedules.subtitle")}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="size-4" />
          {t("schedules.newSchedule")}
        </Button>
      </header>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto w-full max-w-6xl">
          <Frame>
            <FramePanel className="overflow-hidden p-0">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-56">{t("schedules.header.name")}</TableHead>
                    <TableHead className="w-48">{t("schedules.header.cron")}</TableHead>
                    <TableHead className="w-44">{t("schedules.header.next")}</TableHead>
                    <TableHead>{t("schedules.header.last")}</TableHead>
                    <TableHead className="w-20">{t("schedules.header.enabled")}</TableHead>
                    <TableHead className="w-32 text-right">
                      {t("schedules.header.actions")}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {schedules.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={6} className="p-0">
                        <Empty className="py-12">
                          <EmptyHeader>
                            <EmptyMedia variant="icon">
                              <CalendarClock />
                            </EmptyMedia>
                            <EmptyTitle>{t("schedules.empty")}</EmptyTitle>
                          </EmptyHeader>
                        </Empty>
                      </TableCell>
                    </TableRow>
                  ) : (
                    schedules.map((schedule) => (
                      <TableRow key={schedule.id}>
                        <TableCell className="font-medium">
                          <div className="truncate text-foreground">{schedule.name}</div>
                          <div className="truncate text-xs font-normal text-muted-foreground">
                            {schedule.title}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="font-mono text-xs text-foreground">{schedule.cron}</div>
                          <div className="text-[11px] text-muted-foreground">
                            {schedule.timezone}
                          </div>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {schedule.enabled
                            ? format.date(schedule.next_run_at) || t("schedules.disabled")
                            : t("schedules.disabled")}
                        </TableCell>
                        <TableCell>
                          {schedule.last_status ? (
                            <div className="flex flex-col gap-1">
                              <div className="flex items-center gap-2">
                                <Badge variant={STATUS_VARIANT[schedule.last_status]}>
                                  {t(`schedules.status.${schedule.last_status}`)}
                                </Badge>
                                <span className="text-[11px] text-muted-foreground">
                                  {format.date(schedule.last_run_at)}
                                </span>
                              </div>
                              {schedule.last_project_id && (
                                <Link
                                  to={`/projects/${schedule.last_project_id}`}
                                  className="font-mono text-[11px] text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                                >
                                  {schedule.last_project_id}
                                </Link>
                              )}
                              {schedule.last_detail && (
                                <span className="text-[11px] text-muted-foreground">
                                  {schedule.last_detail}
                                </span>
                              )}
                              {schedule.skip_count > 0 && (
                                <span className="text-[11px] text-muted-foreground">
                                  {t("schedules.skippedCount", { count: schedule.skip_count })}
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-xs text-muted-foreground">
                              {t("schedules.neverRan")}
                            </span>
                          )}
                        </TableCell>
                        <TableCell>
                          <Switch
                            checked={schedule.enabled}
                            onCheckedChange={(checked: boolean) => void toggle(schedule, checked)}
                            aria-label={t("schedules.header.enabled")}
                          />
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center justify-end gap-1">
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    title={t("schedules.run.action")}
                                    aria-label={t("schedules.run.action")}
                                    disabled={running !== null}
                                    onClick={() => void run(schedule)}
                                  />
                                }
                              >
                                {running === schedule.id ? (
                                  <Spinner className="size-3.5" />
                                ) : (
                                  <Play className="size-3.5" />
                                )}
                              </TooltipTrigger>
                              <TooltipPopup>{t("schedules.run.action")}</TooltipPopup>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    title={t("common.edit")}
                                    aria-label={t("common.edit")}
                                    onClick={() => openEdit(schedule)}
                                  />
                                }
                              >
                                <Pencil className="size-3.5" />
                              </TooltipTrigger>
                              <TooltipPopup>{t("common.edit")}</TooltipPopup>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    className="text-destructive-foreground hover:bg-destructive/8 hover:text-destructive-foreground"
                                    title={t("common.delete")}
                                    aria-label={t("common.delete")}
                                    onClick={() => setRemoval(schedule)}
                                  />
                                }
                              >
                                <Trash2 className="size-3.5" />
                              </TooltipTrigger>
                              <TooltipPopup>{t("common.delete")}</TooltipPopup>
                            </Tooltip>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </FramePanel>
            <FrameFooter className="py-2.5 text-xs text-muted-foreground">
              {t("schedules.footer")}
            </FrameFooter>
          </Frame>
        </div>
      </div>

      <ScheduleFormDialog
        form={form}
        editing={editing}
        setForm={setForm}
        saving={saving}
        valid={valid}
        onSave={save}
      />

      <ConfirmDialog
        open={!!removal}
        title={t("schedules.removal.title")}
        description={t("schedules.removal.body", { name: removal?.name ?? "" })}
        onCancel={() => setRemoval(null)}
        onConfirm={remove}
      />
    </div>
  );
}
