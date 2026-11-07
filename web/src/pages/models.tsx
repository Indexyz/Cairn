import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { FlaskConical, Loader2, Pencil, Plus, Search, Trash2 } from "lucide-react";

import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import type { ModelTestResult, TaskType, WorkerModel, WorkerType } from "@/lib/types";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const WORKER_TYPES: WorkerType[] = ["claudecode", "codex", "pi", "mock"];
const TASK_TYPES: TaskType[] = ["bootstrap", "reason", "explore"];

const TYPE_ENV_KEYS: Record<WorkerType, string[]> = {
  claudecode: ["ANTHROPIC_MODEL", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN"],
  codex: ["CODEX_MODEL", "CODEX_BASE_URL", "OPENAI_API_KEY"],
  pi: ["PI_MODEL", "PI_BASE_URL", "PI_API_KEY", "PI_PROVIDER_API", "PI_MODEL_CONTEXT_WINDOW"],
  mock: [],
};

interface EnvRow {
  key: string;
  value: string;
}

interface ModelForm {
  name: string;
  type: WorkerType;
  task_types: TaskType[];
  max_running: number;
  priority: number;
  env: EnvRow[];
}

function emptyForm(): ModelForm {
  return {
    name: "",
    type: "claudecode",
    task_types: ["bootstrap", "reason", "explore"],
    max_running: 1,
    priority: 0,
    env: TYPE_ENV_KEYS.claudecode.map((key) => ({ key, value: "" })),
  };
}

function toForm(model: WorkerModel): ModelForm {
  return {
    name: model.name,
    type: model.type,
    task_types: [...model.task_types],
    max_running: model.max_running,
    priority: model.priority,
    env: Object.entries(model.env).map(([key, value]) => ({ key, value })),
  };
}

function toPayload(form: ModelForm): WorkerModel {
  const env: Record<string, string> = {};
  for (const row of form.env) {
    const key = row.key.trim();
    if (key) env[key] = row.value;
  }
  return {
    name: form.name.trim(),
    type: form.type,
    task_types: form.task_types,
    max_running: form.max_running,
    priority: form.priority,
    env,
  };
}

export function ModelsPage() {
  const toast = useToast();
  const { t } = useTranslation();
  const [models, setModels] = useState<WorkerModel[]>([]);
  const [query, setQuery] = useState("");
  const [type, setType] = useState<"all" | WorkerType>("all");
  const [form, setForm] = useState<ModelForm | null>(null);
  const [originalName, setOriginalName] = useState<string | null>(null);
  const [removal, setRemoval] = useState<WorkerModel | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [results, setResults] = useState<Record<string, ModelTestResult>>({});

  async function load() {
    try {
      setModels(await api<WorkerModel[]>("GET", "/models"));
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
    return models.filter((model) => {
      if (type !== "all" && model.type !== type) return false;
      if (!needle) return true;
      return (
        model.name.toLowerCase().includes(needle) ||
        model.task_types.join(",").includes(needle) ||
        Object.keys(model.env).some((key) => key.toLowerCase().includes(needle))
      );
    });
  }, [models, query, type]);

  function openCreate() {
    setOriginalName(null);
    setForm(emptyForm());
  }

  function openEdit(model: WorkerModel) {
    setOriginalName(model.name);
    setForm(toForm(model));
  }

  function updateEnv(index: number, patch: Partial<EnvRow>) {
    setForm((current) =>
      current
        ? {
            ...current,
            env: current.env.map((row, rowIndex) =>
              rowIndex === index ? { ...row, ...patch } : row,
            ),
          }
        : current,
    );
  }

  async function save() {
    if (!form || saving) return;
    const payload = toPayload(form);
    setSaving(true);
    try {
      if (originalName === null) await api("POST", "/models", payload);
      else await api("PUT", `/models/${encodeURIComponent(originalName)}`, payload);
      setForm(null);
      setOriginalName(null);
      await load();
      toast.show(t(originalName === null ? "models.toast.created" : "models.toast.updated"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setSaving(false);
    }
  }

  async function testModel(model: WorkerModel) {
    if (testing) return;
    setTesting(model.name);
    try {
      const result = await api<ModelTestResult>("POST", `/models/${encodeURIComponent(model.name)}/test`);
      setResults((current) => ({ ...current, [model.name]: result }));
      if (result.ok) {
        toast.show(t("models.test.okToast", { name: model.name, ms: result.duration_ms }));
      } else {
        toast.show(
          t("models.test.failToast", {
            name: model.name,
            status: result.status ? ` (${result.status})` : "",
          }),
          "error",
        );
      }
    } catch (error) {
      setResults((current) => {
        const next = { ...current };
        delete next[model.name];
        return next;
      });
      toast.show((error as Error).message, "error");
    } finally {
      setTesting(null);
    }
  }

  async function remove() {
    if (!removal) return;
    try {
      await api("DELETE", `/models/${encodeURIComponent(removal.name)}`);
      setRemoval(null);
      await load();
      toast.show(t("models.toast.deleted", { name: removal.name }));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  const valid = Boolean(
    form &&
      form.name.trim() &&
      form.task_types.length > 0 &&
      form.max_running > 0 &&
      form.priority >= 0,
  );

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("models.title")}</h1>
          <p className="text-xs text-muted-foreground">{t("models.subtitle")}</p>
        </div>
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
                  placeholder={t("models.searchPlaceholder")}
                  className="pl-8"
                />
              </div>
              <Select value={type} onValueChange={(value) => setType(value as typeof type)}>
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("models.filterAll")}</SelectItem>
                  {WORKER_TYPES.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <span className="text-xs text-muted-foreground">
                {t("models.count", { shown: filtered.length, total: models.length })}
              </span>
              <div className="flex-1" />
              <Button size="sm" onClick={openCreate}>
                <Plus className="size-4" />
                {t("models.newModel")}
              </Button>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-56">{t("models.header.name")}</TableHead>
                  <TableHead className="w-32">{t("models.header.type")}</TableHead>
                  <TableHead>{t("models.header.taskTypes")}</TableHead>
                  <TableHead className="w-24 text-right">{t("models.header.priority")}</TableHead>
                  <TableHead className="w-24 text-right">{t("models.header.concurrency")}</TableHead>
                  <TableHead className="w-24 text-right">{t("models.header.env")}</TableHead>
                  <TableHead className="w-32">{t("models.header.test")}</TableHead>
                  <TableHead className="w-32 text-right">{t("models.header.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={8} className="py-16 text-center text-muted-foreground">
                      {t("models.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {filtered.map((model) => (
                  <TableRow key={model.name}>
                    <TableCell className="font-mono text-xs">
                      <span className="rounded bg-muted px-1.5 py-0.5">{model.name}</span>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">{model.type}</Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {model.task_types.join(", ")}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{model.priority}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {model.max_running > 0 && (
                        <span
                          className={cn(
                            "font-medium",
                            (model.used ?? 0) >= model.max_running
                              ? "text-amber-600"
                              : (model.used ?? 0) > 0
                                ? "text-foreground"
                                : "text-muted-foreground",
                          )}
                          title={t("models.header.concurrencyHint", {
                            used: model.used ?? 0,
                            total: model.max_running,
                          })}
                        >
                          {model.used ?? 0}
                        </span>
                      )}
                      <span className="text-muted-foreground"> / {model.max_running}</span>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {Object.keys(model.env).length}
                    </TableCell>
                    <TableCell>
                      {results[model.name] ? (
                        results[model.name].ok ? (
                          <Badge
                            variant="done"
                            title={`${results[model.name].endpoint} · ${results[model.name].duration_ms}ms`}
                          >
                            {t("models.test.ok", { ms: results[model.name].duration_ms })}
                          </Badge>
                        ) : (
                          <Badge
                            variant="destructive"
                            title={`HTTP ${results[model.name].status ?? "-"} · ${results[model.name].detail || "-"}`}
                          >
                            {t("models.test.fail", { status: results[model.name].status ?? "-" })}
                          </Badge>
                        )
                      ) : (
                        <span className="text-xs text-muted-foreground">{t("models.test.idle")}</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("models.test.action")}
                          disabled={testing !== null}
                          onClick={() => void testModel(model)}
                        >
                          {testing === model.name ? (
                            <Loader2 className="size-3.5 animate-spin" />
                          ) : (
                            <FlaskConical className="size-3.5" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("common.edit")}
                          onClick={() => openEdit(model)}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          title={t("common.delete")}
                          onClick={() => setRemoval(model)}
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
              <span>{t("models.footer.order")}</span>
              <span>{t("models.footer.total", { count: models.length })}</span>
            </div>
          </div>
        </div>
      </div>

      <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
        <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {t(originalName ? "models.form.editTitle" : "models.form.newTitle")}
            </DialogTitle>
            <DialogDescription>{t("models.form.description")}</DialogDescription>
          </DialogHeader>
          {form && (
            <div className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="model-name">{t("models.form.name")}</Label>
                <Input
                  id="model-name"
                  value={form.name}
                  placeholder={t("models.form.namePlaceholder")}
                  onChange={(event) =>
                    setForm((current) => (current ? { ...current, name: event.target.value } : current))
                  }
                />
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div className="grid gap-2">
                  <Label>{t("models.form.type")}</Label>
                  <Select
                    value={form.type}
                    onValueChange={(value) =>
                      setForm((current) => {
                        if (!current) return current;
                        const nextType = value as WorkerType;
                        const preset = TYPE_ENV_KEYS[nextType];
                        const existing = new Map(current.env.map((row) => [row.key, row.value]));
                        return {
                          ...current,
                          type: nextType,
                          env:
                            preset.length > 0
                              ? preset.map((key) => ({ key, value: existing.get(key) ?? "" }))
                              : current.env,
                        };
                      })
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {WORKER_TYPES.map((item) => (
                        <SelectItem key={item} value={item}>
                          {item}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="model-priority">{t("models.form.priority")}</Label>
                  <Input
                    id="model-priority"
                    type="number"
                    min={0}
                    value={form.priority}
                    onChange={(event) =>
                      setForm((current) =>
                        current ? { ...current, priority: Number(event.target.value) } : current,
                      )
                    }
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="model-max">{t("models.form.concurrency")}</Label>
                  <Input
                    id="model-max"
                    type="number"
                    min={1}
                    value={form.max_running}
                    onChange={(event) =>
                      setForm((current) =>
                        current ? { ...current, max_running: Number(event.target.value) } : current,
                      )
                    }
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <Label>{t("models.form.taskTypes")}</Label>
                <div className="flex flex-wrap gap-2">
                  {TASK_TYPES.map((taskType) => {
                    const checked = form.task_types.includes(taskType);
                    return (
                      <Button
                        key={taskType}
                        type="button"
                        size="sm"
                        variant={checked ? "default" : "outline"}
                        onClick={() =>
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  task_types: checked
                                    ? current.task_types.filter((item) => item !== taskType)
                                    : [...current.task_types, taskType],
                                }
                              : current,
                          )
                        }
                      >
                        {taskType}
                      </Button>
                    );
                  })}
                </div>
              </div>

              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <Label>{t("models.form.env")}</Label>
                  <Button
                    variant="link"
                    size="sm"
                    onClick={() =>
                      setForm((current) =>
                        current ? { ...current, env: [...current.env, { key: "", value: "" }] } : current,
                      )
                    }
                  >
                    {t("models.form.addEnv")}
                  </Button>
                </div>
                {form.env.length === 0 && (
                  <p className="text-xs text-muted-foreground">{t("models.form.noEnv")}</p>
                )}
                {form.env.map((row, index) => (
                  <div key={index} className="flex gap-2">
                    <Input
                      value={row.key}
                      placeholder="KEY"
                      className="font-mono text-xs"
                      onChange={(event) => updateEnv(index, { key: event.target.value })}
                    />
                    <Input
                      value={row.value}
                      placeholder="value"
                      className="font-mono text-xs"
                      onChange={(event) => updateEnv(index, { value: event.target.value })}
                    />
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() =>
                        setForm((current) =>
                          current
                            ? { ...current, env: current.env.filter((_, rowIndex) => rowIndex !== index) }
                            : current,
                        )
                      }
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                ))}
                <p className="text-[11px] text-muted-foreground">{t("models.form.envNote")}</p>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setForm(null)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={save} disabled={!valid || saving}>
              {saving ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!removal} onOpenChange={(open) => !open && setRemoval(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("models.removal.title")}</DialogTitle>
            <DialogDescription>
              {t("models.removal.body", { name: removal?.name ?? "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoval(null)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={remove}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
