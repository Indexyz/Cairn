import type { Dispatch, SetStateAction } from "react";
import { useTranslation } from "react-i18next";

import type { Schedule } from "@/lib/types";
import type { ScheduleForm } from "@/pages/schedules";

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
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";


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
        <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {t(form && editing ? "schedules.form.editTitle" : "schedules.form.newTitle")}
            </DialogTitle>
            <DialogDescription>{t("schedules.form.description")}</DialogDescription>
          </DialogHeader>
          {form && (
            <div className="grid gap-4">
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="schedule-name">{t("schedules.form.name")}</Label>
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
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="schedule-enabled">{t("schedules.form.enabled")}</Label>
                  <div className="flex h-9 items-center">
                    <Switch
                      id="schedule-enabled"
                      checked={form.enabled}
                      onCheckedChange={(checked) =>
                        setForm((current) => (current ? { ...current, enabled: checked } : current))
                      }
                    />
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="schedule-cron">{t("schedules.form.cron")}</Label>
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
                  <p className="text-[11px] text-muted-foreground">{t("schedules.form.cronHint")}</p>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="schedule-timezone">{t("schedules.form.timezone")}</Label>
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
                  <p className="text-[11px] text-muted-foreground">
                    {t("schedules.form.timezoneHint")}
                  </p>
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="schedule-title">{t("schedules.form.title")}</Label>
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
              </div>

              <div className="grid gap-2">
                <Label htmlFor="schedule-origin">{t("schedules.form.origin")}</Label>
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
              </div>

              <div className="grid gap-2">
                <Label htmlFor="schedule-goal">{t("schedules.form.goal")}</Label>
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
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="schedule-bootstrap">{t("schedules.form.bootstrap")}</Label>
                  <div className="flex h-9 items-center">
                    <Switch
                      id="schedule-bootstrap"
                      checked={form.bootstrap}
                      onCheckedChange={(checked) =>
                        setForm((current) => (current ? { ...current, bootstrap: checked } : current))
                      }
                    />
                  </div>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="schedule-max-steps">{t("schedules.form.maxSteps")}</Label>
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
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="schedule-hints">{t("schedules.form.hints")}</Label>
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
                <p className="text-[11px] text-muted-foreground">{t("schedules.form.hintsHint")}</p>
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
