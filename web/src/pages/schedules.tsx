import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Loader2, Pencil, Play, Plus, Trash2 } from "lucide-react";

import { useFormat } from "@/lib/format";
import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import type { Schedule, SchedulePayload, ScheduleRunResult, ScheduleStatus } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { ScheduleFormDialog } from "@/components/schedules/schedule-form-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const STATUS_VARIANT: Record<ScheduleStatus, "running" | "paused" | "blocked"> = {
  created: "running",
  skipped: "paused",
  error: "blocked",
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


export function useSchedulesPage() {  const toast = useToast();
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
        t(enabled ? "schedules.toast.enabled" : "schedules.toast.disabled", { name: schedule.name }),
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
      toast.show(t(editing ? "schedules.toast.updated" : "schedules.toast.created"));
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
      if (result.skipped) toast.show(t("schedules.run.skipped", { detail: result.detail }), "error");
      else toast.show(t("schedules.run.created", { id: result.project_id }));
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
      toast.show(t("schedules.toast.deleted", { name: removal.name }));
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
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("schedules.title")}</h1>
          <p className="text-xs text-muted-foreground">{t("schedules.subtitle")}</p>
        </div>
        <div className="flex-1" />
        <Button size="sm" onClick={openCreate}>
          <Plus className="size-4" />
          {t("schedules.newSchedule")}
        </Button>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="mx-auto w-full max-w-6xl">
          <div className="rounded-xl border bg-card">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-56">{t("schedules.header.name")}</TableHead>
                  <TableHead className="w-48">{t("schedules.header.cron")}</TableHead>
                  <TableHead className="w-44">{t("schedules.header.next")}</TableHead>
                  <TableHead>{t("schedules.header.last")}</TableHead>
                  <TableHead className="w-20">{t("schedules.header.enabled")}</TableHead>
                  <TableHead className="w-32 text-right">{t("schedules.header.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {schedules.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="py-16 text-center text-muted-foreground">
                      {t("schedules.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {schedules.map((schedule) => (
                  <TableRow key={schedule.id}>
                    <TableCell className="font-medium">
                      <div className="truncate">{schedule.name}</div>
                      <div className="truncate text-[11px] text-muted-foreground">
                        {schedule.title}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="font-mono text-xs">{schedule.cron}</div>
                      <div className="text-[11px] text-muted-foreground">{schedule.timezone}</div>
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
                              className="font-mono text-[11px] text-muted-foreground hover:underline"
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
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("schedules.run.action")}
                          disabled={running !== null}
                          onClick={() => void run(schedule)}
                        >
                          {running === schedule.id ? (
                            <Loader2 className="size-3.5 animate-spin" />
                          ) : (
                            <Play className="size-3.5" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("common.edit")}
                          onClick={() => openEdit(schedule)}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          title={t("common.delete")}
                          onClick={() => setRemoval(schedule)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="border-t px-3 py-2 text-xs text-muted-foreground">
              {t("schedules.footer")}
            </div>
          </div>
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
