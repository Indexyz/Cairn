import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  CalendarClock,
  ChevronDown,
  ChevronRight,
  FolderKanban,
  Plus,
  Search,
  Square,
} from "lucide-react";

import { ProjectRow } from "@/components/projects/project-row";
import { ProjectsDialogs } from "@/components/projects/projects-dialogs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Frame, FrameFooter, FramePanel } from "@/components/ui/frame";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { api, fetchText } from "@/lib/api";
import type { Hint, ProjectDetail, ProjectStatus, ProjectSummary, Schedule } from "@/lib/types";

export function useProjectsPage() {
  const toast = useToast();
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
  const [forkHints, setForkHints] = useState<Hint[]>([]);
  const [forkSelected, setForkSelected] = useState<Set<string>>(new Set());
  const [forkHintsLoading, setForkHintsLoading] = useState(false);
  const [forkHintsFailed, setForkHintsFailed] = useState(false);
  const [forkingBusy, setForkingBusy] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const nextHintId = useRef(1);
  const [snapshot, setSnapshot] = useState<{ id: string; title: string; text: string } | null>(
    null,
  );

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
        hints:
          hints.length > 0 ? hints.map((content) => ({ content, creator: actorName })) : undefined,
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
      toast.show(t("projects.toast.created"), "success");
      navigate(`/projects/${created.project.id}`);
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function setProjectStatus(projectId: string, next: ProjectStatus) {
    try {
      await api("PUT", `/projects/${projectId}/status`, { status: next });
      await load();
      toast.show(
        t(next === "stopped" ? "projects.toast.stopped" : "projects.toast.resumed"),
        "success",
      );
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
        active.map((project) =>
          api("PUT", `/projects/${project.id}/status`, { status: "stopped" }),
        ),
      );
      const failures = results.filter((result) => result.status === "rejected").length;
      await load();
      toast.show(
        failures === 0
          ? t("projects.toast.stoppedAll", { count: active.length })
          : t("projects.toast.stoppedSome", { failed: failures }),
        failures === 0 ? "success" : "error",
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
      toast.show(t("projects.toast.renamed"), "success");
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
      toast.show(t("projects.toast.reopened"), "success");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function openFork(project: ProjectSummary) {
    setForkTitle(`${project.title} (fork)`);
    setForking({ id: project.id, title: project.title });
    setForkHints([]);
    setForkSelected(new Set());
    setForkHintsFailed(false);
    setForkHintsLoading(true);
    try {
      const detail = await api<ProjectDetail>("GET", `/projects/${project.id}`);
      setForkHints(detail.hints);
      setForkSelected(new Set(detail.hints.map((hint) => hint.id)));
    } catch (error) {
      setForkHintsFailed(true);
      toast.show((error as Error).message, "error");
    } finally {
      setForkHintsLoading(false);
    }
  }

  function toggleForkHint(id: string) {
    setForkSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function forkProject() {
    if (!forking || forkingBusy || forkHintsLoading) return;
    setForkingBusy(true);
    try {
      const forked = await api<ProjectDetail>("POST", `/projects/${forking.id}/fork`, {
        title: forkTitle.trim() || null,
        hint_ids: forkHintsFailed ? undefined : [...forkSelected],
      });
      setForking(null);
      await load();
      toast.show(t("projects.toast.forked", { id: forked.project.id }), "success");
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
      toast.show(t("projects.toast.deleted", { id: removal.id }), "success");
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
    forkHints,
    forkSelected,
    setForkSelected,
    forkHintsLoading,
    forkHintsFailed,
    forking,
    forkingBusy,
    openFork,
    toggleForkHint,
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
    forkHints,
    forkSelected,
    setForkSelected,
    forkHintsLoading,
    forkHintsFailed,
    forking,
    forkingBusy,
    openFork,
    toggleForkHint,
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
    setStatusFilter,
    stopAll,
    stopping,
    toggleGroup,
    actorName,
  } = useProjectsPage();
  const { t } = useTranslation();

  const statusItems = useMemo(
    () => [
      { value: "all", label: t("projects.filterAll") },
      { value: "active", label: t("common.status.active") },
      { value: "stopped", label: t("common.status.stopped") },
      { value: "blocked", label: t("common.status.blocked") },
      { value: "completed", label: t("common.status.completed") },
    ],
    [t],
  );

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b bg-background px-6 py-4">
        <div className="min-w-0">
          <h1 className="font-heading text-lg font-semibold tracking-tight text-foreground">
            {t("projects.title")}
          </h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("projects.summary", {
              active: counts.active,
              stopped: counts.stopped,
              blocked: counts.blocked,
              done: counts.completed,
            })}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {counts.active > 0 && (
            <Button
              variant="outline"
              onClick={() => void stopAll()}
              disabled={stopping}
              loading={stopping}
            >
              <Square className="size-3.5" />
              {t("projects.stopAll")}
            </Button>
          )}
          <Button onClick={() => setNewOpen(true)}>
            <Plus className="size-4" />
            {t("projects.newProject")}
          </Button>
        </div>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="mx-auto flex max-w-6xl flex-col gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <InputGroup className="w-full max-w-xs">
              <InputGroupAddon align="inline-start">
                <Search className="size-4" />
              </InputGroupAddon>
              <InputGroupInput
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t("projects.searchPlaceholder")}
              />
            </InputGroup>

            <div className="w-40">
              <Select
                items={statusItems}
                value={status}
                onValueChange={(value) => {
                  if (value) setStatusFilter(value as "all" | ProjectStatus);
                }}
              >
                <SelectTrigger aria-label={t("projects.header.status")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectPopup>
                  {statusItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectPopup>
              </Select>
            </div>

            <Badge variant="outline" className="ml-auto tabular-nums">
              {t("projects.count", { shown: filtered.length, total: counts.all })}
            </Badge>
          </div>

          <Frame>
            <FramePanel className="overflow-hidden p-0">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{t("projects.header.id")}</TableHead>
                    <TableHead>{t("projects.header.title")}</TableHead>
                    <TableHead>{t("projects.header.status")}</TableHead>
                    <TableHead className="text-right">{t("projects.header.facts")}</TableHead>
                    <TableHead className="text-right">{t("projects.header.intents")}</TableHead>
                    <TableHead className="text-right">{t("projects.header.hints")}</TableHead>
                    <TableHead>{t("projects.header.created")}</TableHead>
                    <TableHead className="text-right">{t("projects.header.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={8} className="p-0">
                        <Empty className="py-12">
                          <EmptyHeader>
                            <EmptyMedia variant="icon">
                              <FolderKanban />
                            </EmptyMedia>
                            <EmptyTitle>{t("projects.empty")}</EmptyTitle>
                          </EmptyHeader>
                        </Empty>
                      </TableCell>
                    </TableRow>
                  ) : (
                    rows.map((row) =>
                      row.kind === "project" ? (
                        <ProjectRow
                          key={row.project.id}
                          project={row.project}
                          inGroup={false}
                          onRename={(project) =>
                            setRename({ id: project.id, title: project.title })
                          }
                          onFork={(project) => void openFork(project)}
                          onSnapshot={(project) => void openSnapshot(project)}
                          onStatus={(project, next) => void setProjectStatus(project.id, next)}
                          onDelete={(project) =>
                            setRemoval({ id: project.id, title: project.title })
                          }
                        />
                      ) : (
                        <Fragment key={row.scheduleId}>
                          <TableRow className="bg-muted/48 hover:bg-muted/64">
                            <TableCell colSpan={8}>
                              <div className="flex items-center gap-2">
                                <Button
                                  variant="ghost"
                                  size="icon-xs"
                                  onClick={() => toggleGroup(row.scheduleId)}
                                  title={
                                    expandedGroups.has(row.scheduleId)
                                      ? t("projects.group.collapse")
                                      : t("projects.group.expand")
                                  }
                                  aria-label={
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
                                <span className="text-xs font-medium text-foreground">
                                  {row.name}
                                </span>
                                <Badge variant="secondary">
                                  {t("projects.group.runs", { count: row.projects.length })}
                                </Badge>
                                <Link
                                  to="/schedules"
                                  className="ml-auto text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
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
                                onFork={(item) => void openFork(item)}
                                onSnapshot={(item) => void openSnapshot(item)}
                                onStatus={(item, next) => void setProjectStatus(item.id, next)}
                                onDelete={(item) => setRemoval({ id: item.id, title: item.title })}
                              />
                            ))}
                        </Fragment>
                      ),
                    )
                  )}
                </TableBody>
              </Table>
            </FramePanel>
            <FrameFooter className="flex items-center justify-between py-2.5 text-xs text-muted-foreground">
              <span>{t("projects.perPage")}</span>
              <span className="tabular-nums">
                {t("projects.pagination", { shown: filtered.length, total: counts.all })}
              </span>
            </FrameFooter>
          </Frame>
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
        forkHints={forkHints}
        forkSelected={forkSelected}
        setForkSelected={setForkSelected}
        forkHintsLoading={forkHintsLoading}
        forkHintsFailed={forkHintsFailed}
        toggleForkHint={toggleForkHint}
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
