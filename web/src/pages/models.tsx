import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { FlaskConical, Pencil, Plus, Search, Sparkles, Trash2 } from "lucide-react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import {
  WORKER_TYPES,
  emptyForm,
  nextEnvId,
  toForm,
  toPayload,
  type EnvRow,
  type ModelForm,
} from "@/components/models/model-form";
import { ModelFormDialog } from "@/components/models/model-form-dialog";
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
import { Spinner } from "@/components/ui/spinner";
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
import type { ModelTestResult, WorkerModel, WorkerType } from "@/lib/types";
import { cn } from "@/lib/utils";

export function useModelsPage() {
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
      toast.show(
        t(originalName === null ? "models.toast.created" : "models.toast.updated"),
        "success",
      );
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
      const result = await api<ModelTestResult>(
        "POST",
        `/models/${encodeURIComponent(model.name)}/test`,
      );
      setResults((current) => ({ ...current, [model.name]: result }));
      if (result.ok) {
        toast.show(
          t("models.test.okToast", { name: model.name, ms: result.duration_ms }),
          "success",
        );
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
      toast.show(t("models.toast.deleted", { name: removal.name }), "success");
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

  const typeItems = useMemo(
    () => [
      { value: "all", label: t("models.filterAll") },
      ...WORKER_TYPES.map((item) => ({ value: item, label: item })),
    ],
    [t],
  );

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b bg-background px-6 py-4">
        <div className="min-w-0">
          <h1 className="font-heading text-lg font-semibold tracking-tight text-foreground">
            {t("models.title")}
          </h1>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("models.subtitle")}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="size-4" />
          {t("models.newModel")}
        </Button>
      </header>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex w-full max-w-6xl flex-col gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <InputGroup className="w-full max-w-xs">
              <InputGroupAddon align="inline-start">
                <Search className="size-4" />
              </InputGroupAddon>
              <InputGroupInput
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t("models.searchPlaceholder")}
              />
            </InputGroup>

            <div className="w-36">
              <Select
                items={typeItems}
                value={type}
                onValueChange={(value) => {
                  if (value) setType(value as typeof type);
                }}
              >
                <SelectTrigger aria-label={t("models.header.type")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectPopup>
                  {typeItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectPopup>
              </Select>
            </div>

            <Badge variant="outline" className="ml-auto tabular-nums">
              {t("models.count", { shown: filtered.length, total: models.length })}
            </Badge>
          </div>

          <Frame>
            <FramePanel className="overflow-hidden p-0">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-56">{t("models.header.name")}</TableHead>
                    <TableHead className="w-32">{t("models.header.type")}</TableHead>
                    <TableHead>{t("models.header.taskTypes")}</TableHead>
                    <TableHead className="w-24 text-right">{t("models.header.priority")}</TableHead>
                    <TableHead className="w-28 text-right">
                      {t("models.header.concurrency")}
                    </TableHead>
                    <TableHead className="w-20 text-right">{t("models.header.env")}</TableHead>
                    <TableHead className="w-32">{t("models.header.test")}</TableHead>
                    <TableHead className="w-32 text-right">{t("models.header.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={8} className="p-0">
                        <Empty className="py-12">
                          <EmptyHeader>
                            <EmptyMedia variant="icon">
                              <Sparkles />
                            </EmptyMedia>
                            <EmptyTitle>{t("models.empty")}</EmptyTitle>
                          </EmptyHeader>
                        </Empty>
                      </TableCell>
                    </TableRow>
                  ) : (
                    filtered.map((model) => (
                      <TableRow key={model.name}>
                        <TableCell className="font-mono text-xs font-medium text-foreground">
                          <span className="rounded-md border bg-muted/60 px-1.5 py-0.5">
                            {model.name}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{model.type}</Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {model.task_types.map((task) => (
                              <Badge key={task} variant="outline" size="sm">
                                {task}
                              </Badge>
                            ))}
                          </div>
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{model.priority}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {model.max_running > 0 && (
                            <span
                              className={cn(
                                "font-medium",
                                (model.used ?? 0) >= model.max_running
                                  ? "text-warning-foreground"
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
                                variant="success"
                                title={`${results[model.name].endpoint} · ${results[model.name].duration_ms}ms`}
                              >
                                {t("models.test.ok", { ms: results[model.name].duration_ms })}
                              </Badge>
                            ) : (
                              <Badge
                                variant="error"
                                title={`HTTP ${results[model.name].status ?? "-"} · ${results[model.name].detail || "-"}`}
                              >
                                {t("models.test.fail", {
                                  status: results[model.name].status ?? "-",
                                })}
                              </Badge>
                            )
                          ) : (
                            <span className="text-xs text-muted-foreground">
                              {t("models.test.idle")}
                            </span>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center justify-end gap-1">
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    title={t("models.test.action")}
                                    aria-label={t("models.test.action")}
                                    disabled={testing !== null}
                                    onClick={() => void testModel(model)}
                                  />
                                }
                              >
                                {testing === model.name ? (
                                  <Spinner className="size-3.5" />
                                ) : (
                                  <FlaskConical className="size-3.5" />
                                )}
                              </TooltipTrigger>
                              <TooltipPopup>{t("models.test.action")}</TooltipPopup>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    title={t("common.edit")}
                                    aria-label={t("common.edit")}
                                    onClick={() => openEdit(model)}
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
                                    onClick={() => setRemoval(model)}
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
            <FrameFooter className="flex items-center justify-between py-2.5 text-xs text-muted-foreground">
              <span>{t("models.footer.order")}</span>
              <span className="tabular-nums">
                {t("models.footer.total", { count: models.length })}
              </span>
            </FrameFooter>
          </Frame>
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
