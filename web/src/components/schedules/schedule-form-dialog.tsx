import type { Dispatch, SetStateAction } from "react";
import { useTranslation } from "react-i18next";

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
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { Schedule } from "@/lib/types";
import type { ScheduleForm } from "@/pages/schedules";

interface ScheduleFormDialogProps {
  form: ScheduleForm | null;
  editing: Schedule | null;
  setForm: Dispatch<SetStateAction<ScheduleForm | null>>;
  saving: boolean;
  valid: boolean;
  onSave: () => void;
}

export function ScheduleFormDialog({
  form,
  editing,
  setForm,
  saving,
  valid,
  onSave,
}: ScheduleFormDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
      <DialogPopup className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {t(form && editing ? "schedules.form.editTitle" : "schedules.form.newTitle")}
          </DialogTitle>
          <DialogDescription>{t("schedules.form.description")}</DialogDescription>
        </DialogHeader>
        {form && (
          <DialogPanel className="grid gap-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="schedule-name">{t("schedules.form.name")}</FieldLabel>
                <Input
                  id="schedule-name"
                  value={form.name}
                  placeholder={t("schedules.form.namePlaceholder")}
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, name: event.target.value } : current,
                    )
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="schedule-enabled">{t("schedules.form.enabled")}</FieldLabel>
                <div className="flex h-8.5 items-center sm:h-8">
                  <Switch
                    id="schedule-enabled"
                    checked={form.enabled}
                    onCheckedChange={(checked) =>
                      setForm((current) => (current ? { ...current, enabled: checked } : current))
                    }
                  />
                </div>
              </Field>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="schedule-cron">{t("schedules.form.cron")}</FieldLabel>
                <Input
                  id="schedule-cron"
                  value={form.cron}
                  placeholder="0 3 * * *"
                  className="font-mono text-xs"
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, cron: event.target.value } : current,
                    )
                  }
                />
                <FieldDescription>{t("schedules.form.cronHint")}</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="schedule-timezone">{t("schedules.form.timezone")}</FieldLabel>
                <Input
                  id="schedule-timezone"
                  value={form.timezone}
                  placeholder="UTC"
                  className="font-mono text-xs"
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, timezone: event.target.value } : current,
                    )
                  }
                />
                <FieldDescription>{t("schedules.form.timezoneHint")}</FieldDescription>
              </Field>
            </div>

            <Field>
              <FieldLabel htmlFor="schedule-title">{t("schedules.form.title")}</FieldLabel>
              <Input
                id="schedule-title"
                value={form.title}
                placeholder={t("schedules.form.titlePlaceholder")}
                onChange={(event) =>
                  setForm((current) =>
                    current ? { ...current, title: event.target.value } : current,
                  )
                }
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="schedule-origin">{t("schedules.form.origin")}</FieldLabel>
              <Textarea
                id="schedule-origin"
                value={form.origin}
                rows={2}
                onChange={(event) =>
                  setForm((current) =>
                    current ? { ...current, origin: event.target.value } : current,
                  )
                }
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="schedule-goal">{t("schedules.form.goal")}</FieldLabel>
              <Textarea
                id="schedule-goal"
                value={form.goal}
                rows={2}
                onChange={(event) =>
                  setForm((current) =>
                    current ? { ...current, goal: event.target.value } : current,
                  )
                }
              />
            </Field>

            <div className="grid gap-3 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="schedule-bootstrap">
                  {t("schedules.form.bootstrap")}
                </FieldLabel>
                <div className="flex h-8.5 items-center sm:h-8">
                  <Switch
                    id="schedule-bootstrap"
                    checked={form.bootstrap}
                    onCheckedChange={(checked) =>
                      setForm((current) =>
                        current ? { ...current, bootstrap: checked } : current,
                      )
                    }
                  />
                </div>
              </Field>
              <Field>
                <FieldLabel htmlFor="schedule-max-steps">{t("schedules.form.maxSteps")}</FieldLabel>
                <Input
                  id="schedule-max-steps"
                  value={form.maxSteps}
                  placeholder={t("schedules.form.maxStepsPlaceholder")}
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, maxSteps: event.target.value } : current,
                    )
                  }
                />
              </Field>
            </div>

            <Field>
              <FieldLabel htmlFor="schedule-hints">{t("schedules.form.hints")}</FieldLabel>
              <Textarea
                id="schedule-hints"
                value={form.hints}
                rows={3}
                className="font-mono text-xs"
                placeholder={t("schedules.form.hintsPlaceholder")}
                onChange={(event) =>
                  setForm((current) =>
                    current ? { ...current, hints: event.target.value } : current,
                  )
                }
              />
              <FieldDescription>{t("schedules.form.hintsHint")}</FieldDescription>
            </Field>
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
