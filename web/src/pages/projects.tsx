import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { FileCode2, Pencil, Plus, RotateCcw, Search, Square, Trash2 } from "lucide-react";

import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { api, copyText, fetchText } from "@/lib/api";
import { formatDate, formatTime } from "@/lib/format";
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

const STATUS_LABEL: Record<ProjectStatus, string> = {
  active: "运行中",
  stopped: "已暂停",
  completed: "已完成",
};

const STATUS_BADGE: Record<ProjectStatus, "running" | "paused" | "done"> = {
  active: "running",
  stopped: "paused",
  completed: "done",
};

export function ProjectsPage() {
  const toast = useToast();
  const navigate = useNavigate();
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
      toast.show("Project created");
      navigate(`/projects/${created.project.id}`);
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function setProjectStatus(projectId: string, next: ProjectStatus) {
    try {
      await api("PUT", `/projects/${projectId}/status`, { status: next });
      await load();
      toast.show(next === "stopped" ? "Project stopped" : "Project resumed");
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
        failures === 0 ? `Stopped ${active.length} project(s)` : `${failures} project(s) failed to stop`,
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
      toast.show("Project renamed");
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
      toast.show("Project reopened");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function deleteProject() {
    if (!removal || deleting) return;
    setDeleting(true);
    try {
      await api("DELETE", `/projects/${removal.id}`);
      setRemoval(null);
      await load();
      toast.show(`Deleted ${removal.id}`);
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
          <h1 className="text-sm font-semibold">Projects</h1>
          <p className="text-xs text-muted-foreground">
            {counts.active} running · {counts.stopped} paused · {counts.completed} done
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
          Stop all
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
                  placeholder="搜索标题 / ID"
                  className="pl-8"
                />
              </div>
              <Select value={status} onValueChange={(value) => setStatus(value as typeof status)}>
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部状态</SelectItem>
                  <SelectItem value="active">运行中</SelectItem>
                  <SelectItem value="stopped">已暂停</SelectItem>
                  <SelectItem value="completed">已完成</SelectItem>
                </SelectContent>
              </Select>
              <span className="text-xs text-muted-foreground">{filtered.length}/{counts.all} 条</span>
              <div className="flex-1" />
              <Button size="sm" onClick={() => setNewOpen(true)}>
                <Plus className="size-4" />
                新建项目
              </Button>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-28">ID</TableHead>
                  <TableHead>标题</TableHead>
                  <TableHead className="w-24">状态</TableHead>
                  <TableHead className="w-20 text-right">事实</TableHead>
                  <TableHead className="w-20 text-right">意图</TableHead>
                  <TableHead className="w-20 text-right">提示</TableHead>
                  <TableHead className="w-40">创建时间</TableHead>
                  <TableHead className="w-56 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className="py-16 text-center text-muted-foreground">
                      没有匹配的项目
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
                          Reasoning · {project.reason.worker} · {formatTime(project.reason.last_heartbeat_at)}
                        </p>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant={STATUS_BADGE[project.status]}>{STATUS_LABEL[project.status]}</Badge>
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
                      {formatDate(project.created_at)}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="outline" size="sm" onClick={() => void openSnapshot(project)}>
                          <FileCode2 className="size-3.5" />
                          Snapshot
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title="重命名"
                          onClick={() => setRename({ id: project.id, title: project.title })}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        {project.status === "active" && (
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            title="暂停"
                            onClick={() => void setProjectStatus(project.id, "stopped")}
                          >
                            <Square className="size-3.5" />
                          </Button>
                        )}
                        {project.status === "stopped" && (
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            title="恢复"
                            onClick={() => void setProjectStatus(project.id, "active")}
                          >
                            <RotateCcw className="size-3.5" />
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          title="删除"
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
              <span>每页 20 条</span>
              <span>
                1-{filtered.length} / 共 {counts.all} 条
              </span>
            </div>
          </div>
        </div>
      </div>

      <Dialog open={newOpen} onOpenChange={setNewOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>New project</DialogTitle>
            <DialogDescription>Define the origin, the goal and optional hints.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <Input
              value={newForm.title}
              onChange={(event) => setNewForm((form) => ({ ...form, title: event.target.value }))}
              placeholder="Project title"
            />
            <Textarea
              value={newForm.origin}
              onChange={(event) => setNewForm((form) => ({ ...form, origin: event.target.value }))}
              placeholder="Origin — starting point"
              rows={2}
            />
            <Textarea
              value={newForm.goal}
              onChange={(event) => setNewForm((form) => ({ ...form, goal: event.target.value }))}
              placeholder="Goal — what to achieve"
              rows={2}
            />
            <div className="flex items-center gap-2">
              <Switch
                checked={newForm.bootstrap}
                onCheckedChange={(checked) => setNewForm((form) => ({ ...form, bootstrap: checked }))}
              />
              <Label>Run bootstrap attempt</Label>
            </div>
            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">Hints (optional)</Label>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => setNewForm((form) => ({ ...form, hints: [...form.hints, ""] }))}
                >
                  + Add
                </Button>
              </div>
              {newForm.hints.map((hint, index) => (
                <Input
                  key={index}
                  value={hint}
                  placeholder="Hint content"
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
              <p className="text-[11px] text-muted-foreground">Hint creator: {actorName}</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={createProject}
              disabled={!newForm.title || !newForm.origin || !newForm.goal}
            >
              Create
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!rename} onOpenChange={(open) => !open && setRename(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename project</DialogTitle>
            <DialogDescription>{rename ? `${rename.id} - ${rename.title}` : ""}</DialogDescription>
          </DialogHeader>
          <Input
            value={rename?.title ?? ""}
            onChange={(event) => setRename((current) => (current ? { ...current, title: event.target.value } : current))}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setRename(null)}>
              Cancel
            </Button>
            <Button onClick={renameProject} disabled={!rename?.title}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!reopen} onOpenChange={(open) => !open && setReopen(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reopen project</DialogTitle>
            <DialogDescription>{reopen ? `${reopen.id} - ${reopen.title}` : ""}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={reopenText}
            onChange={(event) => setReopenText(event.target.value)}
            placeholder="Why is this project not actually complete?"
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setReopen(null)}>
              Cancel
            </Button>
            <Button onClick={reopenProject} disabled={!reopenText}>
              Reopen
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!removal} onOpenChange={(open) => !open && setRemoval(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete project</DialogTitle>
            <DialogDescription>
              Permanently remove {removal ? `${removal.id} - ${removal.title}` : ""}. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoval(null)} disabled={deleting}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={deleteProject} disabled={deleting}>
              {deleting ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!snapshot} onOpenChange={(open) => !open && setSnapshot(null)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>Snapshot</DialogTitle>
            <DialogDescription>{snapshot?.title}</DialogDescription>
          </DialogHeader>
          <div className={cn("max-h-[60vh] overflow-auto rounded-lg border bg-muted/40 p-3")}>
            <pre className="text-xs leading-5">{snapshot?.text}</pre>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => snapshot && copyText(snapshot.text).then(() => toast.show("Copied"))}
            >
              Copy
            </Button>
            <Button onClick={() => setSnapshot(null)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
