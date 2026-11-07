import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { api } from "@/lib/api";
import type { ServerSettings } from "@/lib/types";
import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { LAYOUT_MODES, type LayoutMode } from "@/lib/format";
import { SUPPORTED_LOCALES, currentLocale, setLocale, type Locale } from "@/lib/i18n";
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

export function SettingsDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { prefs, save } = usePrefs();
  const toast = useToast();
  const { t } = useTranslation();
  const [server, setServer] = useState<ServerSettings>({ intent_timeout: 5, reason_timeout: 5 });
  const [actor, setActor] = useState(prefs.actor_name);
  const [layout, setLayout] = useState<LayoutMode>(prefs.layout_mode);

  useEffect(() => {
    if (!open) return;
    setActor(prefs.actor_name);
    setLayout(prefs.layout_mode);
    api<ServerSettings>("GET", "/settings")
      .then(setServer)
      .catch((error: Error) => toast.show(error.message, "error"));
  }, [open, prefs.actor_name, prefs.layout_mode, toast]);

  async function persist() {
    try {
      await api<ServerSettings>("PUT", "/settings", server);
      save({ actor_name: actor, layout_mode: layout });
      onOpenChange(false);
      toast.show(t("settings.saved"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("settings.title")}</DialogTitle>
          <DialogDescription>{t("settings.description")}</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="intent-timeout">{t("settings.intentTimeout")}</Label>
            <Input
              id="intent-timeout"
              type="number"
              min={5}
              value={server.intent_timeout}
              onChange={(event) =>
                setServer((current) => ({ ...current, intent_timeout: Number(event.target.value) }))
              }
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="reason-timeout">{t("settings.reasonTimeout")}</Label>
            <Input
              id="reason-timeout"
              type="number"
              min={5}
              value={server.reason_timeout}
              onChange={(event) =>
                setServer((current) => ({ ...current, reason_timeout: Number(event.target.value) }))
              }
            />
          </div>
          <div className="grid gap-2 border-t pt-4">
            <Label htmlFor="actor">{t("settings.actor")}</Label>
            <Input
              id="actor"
              value={actor}
              placeholder={t("settings.actorPlaceholder")}
              onChange={(event) => setActor(event.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label>{t("settings.language")}</Label>
            <Select value={currentLocale()} onValueChange={(value) => setLocale(value as Locale)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SUPPORTED_LOCALES.map((locale) => (
                  <SelectItem key={locale} value={locale}>
                    {t(`language.${locale}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-2">
            <Label>{t("settings.defaultLayout")}</Label>
            <Select value={layout} onValueChange={(value) => setLayout(value as LayoutMode)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LAYOUT_MODES.map((mode) => (
                  <SelectItem key={mode} value={mode}>
                    {t(`layouts.${mode}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-[11px] text-muted-foreground">{t("settings.storedLocally")}</p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={persist}>{t("common.save")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
