import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  CalendarClock,
  ChevronDown,
  ChevronRight,
  Plus,
  Search,
  Square,
} from "lucide-react";

import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { api, fetchText } from "@/lib/api";
import type { ProjectDetail, ProjectStatus, ProjectSummary, Schedule } from "@/lib/types";
import { ProjectRow } from "@/components/projects/project-row";
import { ProjectsDialogs } from "@/components/projects/projects-dialogs";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";



export function useProjectsPage() {  const toast = useToast();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { actorName } = usePrefs();
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [schedules, setSchedules] = useState<Record<string, string>>({});
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<"all" | ProjectStatus>("all");
  const [stopping, setStopping] = useState(false);

  const [newOpen, setNewOpen] = useState(false);
  const [newForm, setNewForm] = useState({
    title: "",
    origin: "",
    goal: "",
    bootstrap: true,
    maxSteps: "",
    hints: [{ id: 0, content: "" }],
  });

  const [rename, setRename] = useState<{ id: string; title: string } | null>(null);
  const [reopen, setReopen] = useState<{ id: string; title: string } | null>(null);
  const [reopenText, setReopenText] = useState("");
  const [removal, setRemoval] = useState<{ id: string; title: string } | null>(null);
  const [forking, setForking] = useState<{ id: string; title: string } | null>(null);
  const [forkTitle, setForkTitle] = useState("");
  const [forkingBusy, setForkingBusy] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const nextHintId = useRef(1);
  const [snapshot, setSnapshot] = useState<{ id: string; title: string; text: string } | null>(null);

  async function load() {
    try {
      setProjects(await api<ProjectSummary[]>("GET", "/projects"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
    try {
      const list = await api<Schedule[]>("GET", "/schedules");
      setSchedules(Object.fromEntries(list.map((schedule) => [schedule.id, schedule.name])));
    } catch {
      /* schedules are optional decoration for grouping */
    }
  }

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 5000);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return projects.filter((project) => {
      if (status !== "all" && project.status !== status) return false;
      if (!needle) return true;
      return (
        project.title.toLowerCase().includes(needle) || project.id.toLowerCase().includes(needle)
      );
    });
  }, [projects, query, status]);

  const counts = useMemo(
    () => ({
      all: projects.length,
      active: projects.filter((project) => project.status === "active").length,
      stopped: projects.filter((project) => project.status === "stopped").length,
      blocked: projects.filter((project) => project.status === "blocked").length,
      completed: projects.filter((project) => project.status === "completed").length,
    }),
    [projects],
  );

  type Row =
    | { kind: "project"; project: ProjectSummary }
    | { kind: "group"; scheduleId: string; name: string; projects: ProjectSummary[] };

  // Projects started by a schedule collapse under that schedule, so a cron job cannot
  // flood the list; everything else stays a flat row.
  const rows = useMemo<Row[]>(() => {
    const buckets = new Map<string, ProjectSummary[]>();
    const flat: Row[] = [];
    for (const project of filtered) {
      const scheduleId = project.schedule_id;
      if (scheduleId && schedules[scheduleId]) {
        const bucket = buckets.get(scheduleId);
        if (bucket) bucket.push(project);
        else buckets.set(scheduleId, [project]);
      } else {
        flat.push({ kind: "project", project });
      }
    }
    for (const [scheduleId, members] of buckets) {
      flat.push({ kind: "group", scheduleId, name: schedules[scheduleId], projects: members });
    }
    return flat;
  }, [filtered, schedules]);

  function toggleGroup(scheduleId: string) {
    setExpandedGroups((current) => {
      const next = new Set(current);
      if (next.has(scheduleId)) next.delete(scheduleId);
      else next.add(scheduleId);
      return next;
    });
  }

  async function createProject() {
    try {
      const hints = newForm.hints.map((hint) => hint.content.trim()).filter(Boolean);
      const created = await api<ProjectDetail>("POST", "/projects", {
        title: newForm.title,
        origin: newForm.origin,
        goal: newForm.goal,
        bootstrap_enabled: newForm.bootstrap,
        max_steps: newForm.maxSteps.trim() ? Number(newForm.maxSteps) : null,
        hints: hints.length > 0 ? hints.map((content) => ({ content, creator: actorName })) : undefined,
      });
      setNewOpen(false);
      setNewForm({
        title: "",
        origin: "",
        goal: "",
        bootstrap: true,
        maxSteps: "",
        hints: [{ id: nextHintId.current++, content: "" }],
      });
      await load();
      toast.show(t("projects.toast.created"));
      navigate(`/projects/${created.project.id}`);
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function setProjectStatus(projectId: string, next: ProjectStatus) {
    try {
      await api("PUT", `/projects/${projectId}/status`, { status: next });
      await load();
      toast.show(t(next === "stopped" ? "projects.toast.stopped" : "projects.toast.resumed"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function stopAll() {
    const active = projects.filter((project) => project.status === "active");
    if (active.length === 0 || stopping) return;
    setStopping(true);
    try {
      const results = await Promise.allSettled(
        active.map((project) => api("PUT", `/projects/${project.id}/status`, { status: "stopped" })),
      );
      const failures = results.filter((result) => result.status === "rejected").length;
      await load();
      toast.show(
        failures === 0
          ? t("projects.toast.stoppedAll", { count: active.length })
          : t("projects.toast.stoppedSome", { failed: failures }),
        failures === 0 ? "info" : "error",
      );
    } finally {
      setStopping(false);
    }
  }

  async function renameProject() {
    if (!rename) return;
    try {
      await api("PUT", `/projects/${rename.id}/title`, { title: rename.title });
      setRename(null);
      await load();
      toast.show(t("projects.toast.renamed"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function reopenProject() {
    if (!reopen) return;
    try {
      await api("POST", `/projects/${reopen.id}/reopen`, {
        description: reopenText,
        creator: actorName,
      });
      setReopen(null);
      setReopenText("");
      await load();
      toast.show(t("projects.toast.reopened"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function forkProject() {
    if (!forking || forkingBusy) return;
    setForkingBusy(true);
    try {
      const forked = await api<ProjectDetail>("POST", `/projects/${forking.id}/fork`, {
        title: forkTitle.trim() || null,
      });
      setForking(null);
      await load();
      toast.show(t("projects.toast.forked", { id: forked.project.id }));
      navigate(`/projects/${forked.project.id}`);
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setForkingBusy(false);
    }
  }

  async function deleteProject() {
    if (!removal || deleting) return;
    setDeleting(true);
    try {
      await api("DELETE", `/projects/${removal.id}`);
      setRemoval(null);
      await load();
      toast.show(t("projects.toast.deleted", { id: removal.id }));
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setDeleting(false);
    }
  }

  async function openSnapshot(project: ProjectSummary) {
    try {
      const text = await fetchText(`/projects/${project.id}/export?format=yaml`);
      setSnapshot({ id: project.id, title: `${project.id} - ${project.title}`, text });
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  return {
    counts,
    createProject,
    deleteProject,
    actorName,
    deleting,
    expandedGroups,
    filtered,
    forkProject,
    forkTitle,
    forking,
    forkingBusy,
    newForm,
    newOpen,
    nextHintId,
    query,
    removal,
    rename,
    renameProject,
    reopen,
    reopenProject,
    reopenText,
    rows,
    setForkTitle,
    setForking,
    setNewForm,
    setNewOpen,
    setQuery,
    setRemoval,
    setRename,
    setReopen,
    setReopenText,
    setSnapshot,
    setProjectStatus,
    openSnapshot,
    snapshot,
    status,
    setStatusFilter: setStatus,
    stopAll,
    stopping,
    toggleGroup,
  };
}

export function ProjectsPage() {
  const {
    counts,
    createProject,
    deleteProject,
    deleting,
    expandedGroups,
    filtered,
    forkProject,
    forkTitle,
    forking,
    forkingBusy,
    newForm,
    newOpen,
    nextHintId,
    query,
    removal,
    rename,
    renameProject,
    reopen,
    reopenProject,
    reopenText,
    rows,
    setForkTitle,
    setForking,
    setNewForm,
    setNewOpen,
    setQuery,
    setRemoval,
    setRename,
    setReopen,
    setReopenText,
    setSnapshot,
    setProjectStatus,
    openSnapshot,
    actorName,
    setStatusFilter,
    snapshot,
    status,
    stopAll,
    stopping,
    toggleGroup,
  } = useProjectsPage();
  const { t } = useTranslation();

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("projects.title")}</h1>
          <p className="text-xs text-muted-foreground">
            {t("projects.summary", {
              active: counts.active,
              stopped: counts.stopped,
              blocked: counts.blocked,
              done: counts.completed,
            })}
          </p>
        </div>
        <div className="flex-1" />
        <Button
          variant="outline"
          size="sm"
          onClick={stopAll}
          disabled={counts.active === 0 || stopping}
        >
          <Square className="size-3.5" />
          {t("projects.stopAll")}
        </Button>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="mx-auto w-full max-w-6xl">
          <div className="rounded-xl border bg-card">
            <div className="flex flex-wrap items-center gap-2 border-b p-3">
              <div className="relative min-w-60 flex-1">
                <Search className="absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t("projects.searchPlaceholder")}
                  className="pl-8"
                />
              </div>
              <Select value={status} onValueChange={(value) => setStatusFilter(value as typeof status)}>
                <SelectTrigger className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("projects.filterAll")}</SelectItem>
                  <SelectItem value="active">{t("common.status.active")}</SelectItem>
                  <SelectItem value="stopped">{t("common.status.stopped")}</SelectItem>
                  <SelectItem value="blocked">{t("common.status.blocked")}</SelectItem>
                  <SelectItem value="completed">{t("common.status.completed")}</SelectItem>
                </SelectContent>
              </Select>
              <span className="text-xs text-muted-foreground">
                {t("projects.count", { shown: filtered.length, total: counts.all })}
              </span>
              <div className="flex-1" />
              <Button size="sm" onClick={() => setNewOpen(true)}>
                <Plus className="size-4" />
                {t("projects.newProject")}
              </Button>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-28">{t("projects.header.id")}</TableHead>
                  <TableHead>{t("projects.header.title")}</TableHead>
                  <TableHead className="w-24">{t("projects.header.status")}</TableHead>
                  <TableHead className="w-20 text-right">{t("projects.header.facts")}</TableHead>
                  <TableHead className="w-20 text-right">{t("projects.header.intents")}</TableHead>
                  <TableHead className="w-20 text-right">{t("projects.header.hints")}</TableHead>
                  <TableHead className="w-40">{t("projects.header.created")}</TableHead>
                  <TableHead className="w-56 text-right">{t("projects.header.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className="py-16 text-center text-muted-foreground">
                      {t("projects.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {rows.map((row) =>
                  row.kind === "project" ? (
                    <ProjectRow
                      key={row.project.id}
                      project={row.project}
                      inGroup={false}
                      onRename={(project) => setRename({ id: project.id, title: project.title })}
                      onFork={(project) => {
                        setForkTitle(`${project.title} (fork)`);
                        setForking({ id: project.id, title: project.title });
                      }}
                      onSnapshot={(project) => void openSnapshot(project)}
                      onStatus={(project, next) => void setProjectStatus(project.id, next)}
                      onDelete={(project) => setRemoval({ id: project.id, title: project.title })}
                    />
                  ) : (
                    <Fragment key={row.scheduleId}>
                      <TableRow className="bg-muted/40 hover:bg-muted/40">
                        <TableCell colSpan={8}>
                          <div className="flex items-center gap-2">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => toggleGroup(row.scheduleId)}
                              title={
                                expandedGroups.has(row.scheduleId)
                                  ? t("projects.group.collapse")
                                  : t("projects.group.expand")
                              }
                            >
                              {expandedGroups.has(row.scheduleId) ? (
                                <ChevronDown className="size-3.5" />
                              ) : (
                                <ChevronRight className="size-3.5" />
                              )}
                            </Button>
                            <CalendarClock className="size-3.5 text-muted-foreground" />
                            <span className="text-xs font-medium">{row.name}</span>
                            <Badge variant="secondary">
                              {t("projects.group.runs", { count: row.projects.length })}
                            </Badge>
                            <Link
                              to="/schedules"
                              className="ml-auto text-[11px] text-muted-foreground hover:underline"
                            >
                              {t("projects.group.openSchedule")}
                            </Link>
                          </div>
                        </TableCell>
                      </TableRow>
                      {expandedGroups.has(row.scheduleId) &&
                        row.projects.map((project) => (
                          <ProjectRow
                            key={project.id}
                            project={project}
                            inGroup
                            onRename={(item) => setRename({ id: item.id, title: item.title })}
                            onFork={(item) => {
                              setForkTitle(`${item.title} (fork)`);
                              setForking({ id: item.id, title: item.title });
                            }}
                            onSnapshot={(item) => void openSnapshot(item)}
                            onStatus={(item, next) => void setProjectStatus(item.id, next)}
                            onDelete={(item) => setRemoval({ id: item.id, title: item.title })}
                          />
                        ))}
                    </Fragment>
                  ),
                )}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between border-t px-3 py-2 text-xs text-muted-foreground">
              <span>{t("projects.perPage")}</span>
              <span>{t("projects.pagination", { shown: filtered.length, total: counts.all })}</span>
            </div>
          </div>
        </div>
      </div>

      <ProjectsDialogs
        newOpen={newOpen}
        setNewOpen={setNewOpen}
        newForm={newForm}
        setNewForm={setNewForm}
        nextHintId={nextHintId}
        actorName={actorName}
        createProject={createProject}
        rename={rename}
        setRename={setRename}
        renameProject={renameProject}
        reopen={reopen}
        setReopen={setReopen}
        reopenText={reopenText}
        setReopenText={setReopenText}
        reopenProject={reopenProject}
        forking={forking}
        setForking={setForking}
        forkTitle={forkTitle}
        setForkTitle={setForkTitle}
        forkProject={forkProject}
        forkingBusy={forkingBusy}
        removal={removal}
        setRemoval={setRemoval}
        deleteProject={deleteProject}
        snapshot={snapshot}
        setSnapshot={setSnapshot}
        deleting={deleting}
      />
    </div>
  );
}
