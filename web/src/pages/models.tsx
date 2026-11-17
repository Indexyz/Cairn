import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { FlaskConical, Loader2, Pencil, Plus, Search, Trash2 } from "lucide-react";

import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import type { ModelTestResult, WorkerModel, WorkerType } from "@/lib/types";
import type { EnvRow, ModelForm } from "@/components/models/model-form";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { ModelFormDialog } from "@/components/models/model-form-dialog";
import { WORKER_TYPES, emptyForm, nextEnvId, toForm, toPayload } from "@/components/models/model-form";
import { Input } from "@/components/ui/input";
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


export function useModelsPage() {  const toast = useToast();
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
    setForm(emptyForm(nextEnvId));
  }

  function openEdit(model: WorkerModel) {
    setOriginalName(model.name);
    setForm(toForm(model));
  }

  function updateEnv(id: number, patch: Partial<EnvRow>) {
    setForm((current) =>
      current
        ? {
            ...current,
            env: current.env.map((row) => (row.id === id ? { ...row, ...patch } : row)),
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

  return {
    filtered,
    form,
    models,
    openCreate,
    openEdit,
    originalName,
    query,
    removal,
    remove,
    results,
    save,
    saving,
    setForm,
    setQuery,
    setRemoval,
    setType,
    testModel,
    testing,
    type,
    updateEnv,
    valid,
  };
}

export function ModelsPage() {
  const {
    filtered,
    form,
    models,
    openCreate,
    openEdit,
    originalName,
    query,
    removal,
    remove,
    results,
    save,
    saving,
    setForm,
    setQuery,
    setRemoval,
    setType,
    testModel,
    testing,
    type,
    updateEnv,
    valid,
  } = useModelsPage();
  const { t } = useTranslation();

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

      <ModelFormDialog
        form={form}
        setForm={setForm}
        originalName={originalName}
        saving={saving}
        valid={valid}
        updateEnv={updateEnv}
        onSave={save}
      />

      <ConfirmDialog
        open={!!removal}
        title={t("models.removal.title")}
        description={t("models.removal.body", { name: removal?.name ?? "" })}
        onCancel={() => setRemoval(null)}
        onConfirm={remove}
      />
    </div>
  );
}
