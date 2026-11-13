import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { FileCode2, GitFork, Pencil, Plus, RotateCcw, Search, Square, Trash2 } from "lucide-react";

import { useFormat } from "@/lib/format";
import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { api, copyText, fetchText } from "@/lib/api";
import type { ProjectDetail, ProjectStatus, ProjectSummary } from "@/lib/types";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const STATUS_BADGE: Record<ProjectStatus, "running" | "paused" | "done"> = {
  active: "running",
  stopped: "paused",
  completed: "done",
};

export function ProjectsPage() {
  const toast = useToast();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const format = useFormat();
  const { actorName } = usePrefs();
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<"all" | ProjectStatus>("all");
  const [stopping, setStopping] = useState(false);

  const [newOpen, setNewOpen] = useState(false);
  const [newForm, setNewForm] = useState({ title: "", origin: "", goal: "", bootstrap: true, hints: [""] });

  const [rename, setRename] = useState<{ id: string; title: string } | null>(null);
  const [reopen, setReopen] = useState<{ id: string; title: string } | null>(null);
  const [reopenText, setReopenText] = useState("");
  const [removal, setRemoval] = useState<{ id: string; title: string } | null>(null);
  const [forking, setForking] = useState<{ id: string; title: string } | null>(null);
  const [forkTitle, setForkTitle] = useState("");
  const [forkingBusy, setForkingBusy] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [snapshot, setSnapshot] = useState<{ title: string; text: string } | null>(null);

  async function load() {
    try {
      setProjects(await api<ProjectSummary[]>("GET", "/projects"));
    } catch (error) {
      toast.show((error as Error).message, "error");
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
      completed: projects.filter((project) => project.status === "completed").length,
    }),
    [projects],
  );

  async function createProject() {
    try {
      const hints = newForm.hints.map((hint) => hint.trim()).filter(Boolean);
      const created = await api<ProjectDetail>("POST", "/projects", {
        title: newForm.title,
        origin: newForm.origin,
        goal: newForm.goal,
        bootstrap_enabled: newForm.bootstrap,
        hints: hints.length > 0 ? hints.map((content) => ({ content, creator: actorName })) : undefined,
      });
      setNewOpen(false);
      setNewForm({ title: "", origin: "", goal: "", bootstrap: true, hints: [""] });
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
      setSnapshot({ title: `${project.id} - ${project.title}`, text });
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("projects.title")}</h1>
          <p className="text-xs text-muted-foreground">
            {t("projects.summary", { active: counts.active, stopped: counts.stopped, done: counts.completed })}
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
              <Select value={status} onValueChange={(value) => setStatus(value as typeof status)}>
                <SelectTrigger className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("projects.filterAll")}</SelectItem>
                  <SelectItem value="active">{t("common.status.active")}</SelectItem>
                  <SelectItem value="stopped">{t("common.status.stopped")}</SelectItem>
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
                {filtered.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className="py-16 text-center text-muted-foreground">
                      {t("projects.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {filtered.map((project) => (
                  <TableRow key={project.id}>
                    <TableCell className="font-mono text-xs text-muted-foreground">
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
                      <Badge variant={STATUS_BADGE[project.status]}>
                        {t(`common.status.${project.status}`)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{project.fact_count}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {project.intent_count}
                      {project.working_intent_count > 0 && (
                        <span className="ml-1 text-amber-600">+{project.working_intent_count}</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{project.hint_count}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {format.date(project.created_at)}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="outline" size="sm" onClick={() => void openSnapshot(project)}>
                          <FileCode2 className="size-3.5" />
                          {t("projects.snapshot")}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("common.edit")}
                          onClick={() => setRename({ id: project.id, title: project.title })}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        {project.status === "active" && (
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            title={t("project.stop")}
                            onClick={() => void setProjectStatus(project.id, "stopped")}
                          >
                            <Square className="size-3.5" />
                          </Button>
                        )}
                        {project.status === "stopped" && (
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            title={t("project.resume")}
                            onClick={() => void setProjectStatus(project.id, "active")}
                          >
                            <RotateCcw className="size-3.5" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("projects.fork.action")}
                          onClick={() => {
                            setForkTitle(`${project.title} (fork)`);
                            setForking({ id: project.id, title: project.title });
                          }}
                        >
                          <GitFork className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          title={t("common.delete")}
                          onClick={() => setRemoval({ id: project.id, title: project.title })}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between border-t px-3 py-2 text-xs text-muted-foreground">
              <span>{t("projects.perPage")}</span>
              <span>{t("projects.pagination", { shown: filtered.length, total: counts.all })}</span>
            </div>
          </div>
        </div>
      </div>

      <Dialog open={newOpen} onOpenChange={setNewOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("projects.new.title")}</DialogTitle>
            <DialogDescription>{t("projects.new.description")}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <Input
              value={newForm.title}
              onChange={(event) => setNewForm((form) => ({ ...form, title: event.target.value }))}
              placeholder={t("projects.new.projectTitle")}
            />
            <Textarea
              value={newForm.origin}
              onChange={(event) => setNewForm((form) => ({ ...form, origin: event.target.value }))}
              placeholder={t("projects.new.origin")}
              rows={2}
            />
            <Textarea
              value={newForm.goal}
              onChange={(event) => setNewForm((form) => ({ ...form, goal: event.target.value }))}
              placeholder={t("projects.new.goal")}
              rows={2}
            />
            <div className="flex items-center gap-2">
              <Switch
                checked={newForm.bootstrap}
                onCheckedChange={(checked) => setNewForm((form) => ({ ...form, bootstrap: checked }))}
              />
              <Label>{t("projects.new.bootstrap")}</Label>
            </div>
            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">{t("projects.new.hints")}</Label>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => setNewForm((form) => ({ ...form, hints: [...form.hints, ""] }))}
                >
                  + {t("common.add")}
                </Button>
              </div>
              {newForm.hints.map((hint, index) => (
                <Input
                  key={index}
                  value={hint}
                  placeholder={t("projects.new.hintPlaceholder")}
                  onChange={(event) =>
                    setNewForm((form) => ({
                      ...form,
                      hints: form.hints.map((item, itemIndex) =>
                        itemIndex === index ? event.target.value : item,
                      ),
                    }))
                  }
                />
              ))}
              <p className="text-[11px] text-muted-foreground">
                {t("projects.new.hintCreator", { actor: actorName })}
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              onClick={createProject}
              disabled={!newForm.title || !newForm.origin || !newForm.goal}
            >
              {t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!rename} onOpenChange={(open) => !open && setRename(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.rename.title")}</DialogTitle>
            <DialogDescription>{rename ? `${rename.id} - ${rename.title}` : ""}</DialogDescription>
          </DialogHeader>
          <Input
            value={rename?.title ?? ""}
            onChange={(event) =>
              setRename((current) => (current ? { ...current, title: event.target.value } : current))
            }
          />
          <p className="text-[11px] text-muted-foreground">{t("projects.rename.hint")}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRename(null)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={renameProject} disabled={!rename?.title}>
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!reopen} onOpenChange={(open) => !open && setReopen(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.reopen.title")}</DialogTitle>
            <DialogDescription>{reopen ? `${reopen.id} - ${reopen.title}` : ""}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={reopenText}
            onChange={(event) => setReopenText(event.target.value)}
            placeholder={t("projects.reopen.placeholder")}
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setReopen(null)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={reopenProject} disabled={!reopenText}>
              {t("project.reopen")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!forking} onOpenChange={(open) => !open && setForking(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.fork.title")}</DialogTitle>
            <DialogDescription>
              {t("projects.fork.description", { label: forking ? `${forking.id} - ${forking.title}` : "" })}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="fork-title">{t("projects.fork.titleLabel")}</Label>
            <Input id="fork-title" value={forkTitle} onChange={(event) => setForkTitle(event.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setForking(null)} disabled={forkingBusy}>
              {t("common.cancel")}
            </Button>
            <Button onClick={forkProject} disabled={forkingBusy}>
              <GitFork className="size-4" />
              {forkingBusy ? t("common.saving") : t("projects.fork.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!removal} onOpenChange={(open) => !open && setRemoval(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.removal.title")}</DialogTitle>
            <DialogDescription>
              {t("projects.removal.body", { label: removal ? `${removal.id} - ${removal.title}` : "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoval(null)} disabled={deleting}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={deleteProject} disabled={deleting}>
              {deleting ? t("common.deleting") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!snapshot} onOpenChange={(open) => !open && setSnapshot(null)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("projects.snapshotTitle")}</DialogTitle>
            <DialogDescription>{snapshot?.title}</DialogDescription>
          </DialogHeader>
          <div className={cn("max-h-[60vh] overflow-auto rounded-lg border bg-muted/40 p-3")}>
            <pre className="text-xs leading-5">{snapshot?.text}</pre>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() =>
                snapshot &&
                copyText(snapshot.text)
                  .then(() => toast.show(t("common.copied")))
                  .catch(() => toast.show(t("common.copyFailed"), "error"))
              }
            >
              {t("common.copy")}
            </Button>
            <Button onClick={() => setSnapshot(null)}>{t("common.close")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
