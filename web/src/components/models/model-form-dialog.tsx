import { useMemo, type Dispatch, type SetStateAction } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";

import {
  TASK_TYPES,
  TYPE_ENV_KEYS,
  WORKER_TYPES,
  nextEnvId,
  type EnvRow,
  type ModelForm,
} from "@/components/models/model-form";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogPanel,
  DialogPopup,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { WorkerType } from "@/lib/types";

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

  const workerTypeItems = useMemo(
    () => WORKER_TYPES.map((item) => ({ value: item, label: item })),
    [],
  );

  return (
    <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
      <DialogPopup className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {t(originalName ? "models.form.editTitle" : "models.form.newTitle")}
          </DialogTitle>
          <DialogDescription>{t("models.form.description")}</DialogDescription>
        </DialogHeader>
        {form && (
          <DialogPanel className="grid gap-4">
            <Field>
              <FieldLabel htmlFor="model-name">{t("models.form.name")}</FieldLabel>
              <Input
                id="model-name"
                value={form.name}
                placeholder={t("models.form.namePlaceholder")}
                onChange={(event) =>
                  setForm((current) =>
                    current ? { ...current, name: event.target.value } : current,
                  )
                }
              />
            </Field>

            <div className="grid gap-3 sm:grid-cols-3">
              <Field>
                <FieldLabel>{t("models.form.type")}</FieldLabel>
                <Select
                  items={workerTypeItems}
                  value={form.type}
                  onValueChange={(value) => {
                    if (!value) return;
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
                            ? preset.map((key) => ({
                                id: nextEnvId(),
                                key,
                                value: existing.get(key) ?? "",
                              }))
                            : current.env,
                      };
                    });
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectPopup>
                    {workerTypeItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="model-priority">{t("models.form.priority")}</FieldLabel>
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
              </Field>
              <Field>
                <FieldLabel htmlFor="model-max">{t("models.form.concurrency")}</FieldLabel>
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
              </Field>
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
                      aria-pressed={checked}
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
                  variant="ghost"
                  size="xs"
                  onClick={() =>
                    setForm((current) =>
                      current
                        ? {
                            ...current,
                            env: [...current.env, { id: nextEnvId(), key: "", value: "" }],
                          }
                        : current,
                    )
                  }
                >
                  <Plus className="size-3.5" />
                  {t("models.form.addEnv")}
                </Button>
              </div>
              {form.env.length === 0 && (
                <p className="text-xs text-muted-foreground">{t("models.form.noEnv")}</p>
              )}
              {form.env.map((row) => (
                <div key={row.id} className="flex items-center gap-2">
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
                    size="icon-sm"
                    aria-label={t("common.remove")}
                    onClick={() =>
                      setForm((current) =>
                        current
                          ? { ...current, env: current.env.filter((item) => item.id !== row.id) }
                          : current,
                      )
                    }
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              ))}
              <p className="text-xs text-muted-foreground">{t("models.form.envNote")}</p>
            </div>
          </DialogPanel>
        )}
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
          <Button onClick={onSave} disabled={!valid || saving} loading={saving}>
            {saving ? t("common.saving") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogPopup>
    </Dialog>
  );
}
