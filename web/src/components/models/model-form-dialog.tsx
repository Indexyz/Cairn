import type { Dispatch, SetStateAction } from "react";
import { useTranslation } from "react-i18next";
import { Trash2 } from "lucide-react";

import type { WorkerType } from "@/lib/types";
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
  nextEnvId,
  TASK_TYPES,
  TYPE_ENV_KEYS,
  WORKER_TYPES,
  type EnvRow,
  type ModelForm,
} from "@/components/models/model-form";

interface ModelFormDialogProps {
  form: ModelForm | null;
  setForm: Dispatch<SetStateAction<ModelForm | null>>;
  originalName: string | null;
  saving: boolean;
  valid: boolean;
  updateEnv: (id: number, patch: Partial<EnvRow>) => void;
  onSave: () => void;
}

export function ModelFormDialog({
  form,
  setForm,
  originalName,
  saving,
  valid,
  updateEnv,
  onSave,
}: ModelFormDialogProps) {
  const { t } = useTranslation();

  return (
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
                              ? preset.map((key) => ({ id: nextEnvId(), key, value: existing.get(key) ?? "" }))
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
                        current
                          ? { ...current, env: [...current.env, { id: nextEnvId(), key: "", value: "" }] }
                          : current,
                      )
                    }
                  >
                    {t("models.form.addEnv")}
                  </Button>
                </div>
                {form.env.length === 0 && (
                  <p className="text-xs text-muted-foreground">{t("models.form.noEnv")}</p>
                )}
                {form.env.map((row) => (
                  <div key={row.id} className="flex gap-2">
                    <Input
                      value={row.key}
                      placeholder="KEY"
                      className="font-mono text-xs"
                      onChange={(event) => updateEnv(row.id, { key: event.target.value })}
                    />
                    <Input
                      value={row.value}
                      placeholder="value"
                      className="font-mono text-xs"
                      onChange={(event) => updateEnv(row.id, { value: event.target.value })}
                    />
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() =>
                        setForm((current) =>
                          current
                            ? { ...current, env: current.env.filter((item) => item.id !== row.id) }
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
            <Button onClick={onSave} disabled={!valid || saving}>
              {saving ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

  );
}
