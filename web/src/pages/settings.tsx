import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, MoveDown } from "lucide-react";

import { api } from "@/lib/api";
import type { ServerSettings } from "@/lib/types";
import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { LAYOUT_MODES, type LayoutMode } from "@/lib/format";
import { SUPPORTED_LOCALES, currentLocale, setLocale, type Locale } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export function SettingsPage() {
  const { prefs, save } = usePrefs();
  const toast = useToast();
  const { t } = useTranslation();
  const [server, setServer] = useState<ServerSettings>({ intent_timeout: 5, reason_timeout: 5 });
  const [actor, setActor] = useState(prefs.actor_name);
  const [layout, setLayout] = useState<LayoutMode>(prefs.layout_mode);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setActor(prefs.actor_name);
    setLayout(prefs.layout_mode);
  }, [prefs.actor_name, prefs.layout_mode]);

  useEffect(() => {
    api<ServerSettings>("GET", "/settings")
      .then(setServer)
      .catch((error: Error) => toast.show(error.message, "error"));
  }, [toast]);

  const dirty =
    actor.trim() !== prefs.actor_name || layout !== prefs.layout_mode;

  async function persistServer() {
    if (saving) return;
    setSaving(true);
    try {
      await api<ServerSettings>("PUT", "/settings", server);
      toast.show(t("settings.serverSaved"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setSaving(false);
    }
  }

  function persistBrowser(nextLayout: LayoutMode = layout) {
    save({ actor_name: actor, layout_mode: nextLayout });
    toast.show(t("settings.saved"));
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("settings.title")}</h1>
          <p className="text-xs text-muted-foreground">{t("settings.description")}</p>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
          <section className="rounded-xl border bg-card p-5">
            <header className="mb-4">
              <h2 className="text-sm font-semibold">{t("settings.dispatcherSection")}</h2>
              <p className="text-xs text-muted-foreground">{t("settings.dispatcherHint")}</p>
            </header>
            <div className="grid gap-4 sm:grid-cols-2">
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
            </div>
            <div className="mt-4 flex justify-end">
              <Button onClick={persistServer} disabled={saving}>
                <Check className="size-4" />
                {t("settings.saveServer")}
              </Button>
            </div>
          </section>

          <section className="rounded-xl border bg-card p-5">
            <header className="mb-4">
              <h2 className="text-sm font-semibold">{t("settings.browserSection")}</h2>
              <p className="text-xs text-muted-foreground">{t("settings.storedLocally")}</p>
            </header>
            <div className="grid gap-4">
              <div className="grid gap-2">
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
              </div>
            </div>
            <div className="mt-4 flex items-center justify-between gap-3">
              <span className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground">
                {dirty ? <MoveDown className="size-3" /> : null}
                {dirty ? t("settings.unsaved") : ""}
              </span>
              <Button onClick={() => persistBrowser()} disabled={!dirty}>
                {t("settings.saveBrowser")}
              </Button>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
