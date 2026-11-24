import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Monitor, Moon, Sun } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import {
  Frame,
  FrameDescription,
  FrameFooter,
  FrameHeader,
  FramePanel,
  FrameTitle,
} from "@/components/ui/frame";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { usePrefs } from "@/hooks/use-prefs";
import { THEME_MODES, useTheme, type ThemeMode } from "@/hooks/use-theme";
import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import { LAYOUT_MODES, type LayoutMode } from "@/lib/format";
import { SUPPORTED_LOCALES, currentLocale, setLocale, type Locale } from "@/lib/i18n";
import type { ServerSettings } from "@/lib/types";
import { cn } from "@/lib/utils";

const THEME_ICONS: Record<ThemeMode, typeof Sun> = {
  system: Monitor,
  light: Sun,
  dark: Moon,
};

export function SettingsPage() {
  const { prefs, save } = usePrefs();
  const { theme, setTheme } = useTheme();
  const toast = useToast();
  const { t } = useTranslation();
  const [server, setServer] = useState<ServerSettings>({
    intent_timeout: 5,
    reason_timeout: 5,
    max_workers: 0,
    max_running_projects: 0,
    max_project_workers: 0,
  });
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

  const dirty = actor.trim() !== prefs.actor_name || layout !== prefs.layout_mode;

  const localeItems = useMemo(
    () => SUPPORTED_LOCALES.map((locale) => ({ value: locale, label: t(`language.${locale}`) })),
    [t],
  );

  const layoutItems = useMemo(
    () => LAYOUT_MODES.map((mode) => ({ value: mode, label: t(`layouts.${mode}`) })),
    [t],
  );

  async function persistServer() {
    if (saving) return;
    setSaving(true);
    try {
      await api<ServerSettings>("PUT", "/settings", server);
      toast.show(t("settings.serverSaved"), "success");
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setSaving(false);
    }
  }

  function persistBrowser(nextLayout: LayoutMode = layout) {
    save({ actor_name: actor, layout_mode: nextLayout });
    toast.show(t("settings.saved"), "success");
  }

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex shrink-0 items-center justify-between gap-3 border-b bg-background px-6 py-4">
        <div className="min-w-0">
          <h1 className="font-heading text-lg font-semibold tracking-tight text-foreground">
            {t("settings.title")}
          </h1>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("settings.description")}</p>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex w-full max-w-2xl flex-col gap-6">
          <Frame>
            <FrameHeader>
              <FrameTitle>{t("settings.dispatcherSection")}</FrameTitle>
              <FrameDescription className="text-xs">
                {t("settings.dispatcherHint")}
              </FrameDescription>
            </FrameHeader>
            <FramePanel className="grid gap-5">
              <div className="grid gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="intent-timeout">{t("settings.intentTimeout")}</FieldLabel>
                  <Input
                    id="intent-timeout"
                    type="number"
                    min={5}
                    value={server.intent_timeout}
                    onChange={(event) =>
                      setServer((current) => ({
                        ...current,
                        intent_timeout: Number(event.target.value),
                      }))
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="reason-timeout">{t("settings.reasonTimeout")}</FieldLabel>
                  <Input
                    id="reason-timeout"
                    type="number"
                    min={5}
                    value={server.reason_timeout}
                    onChange={(event) =>
                      setServer((current) => ({
                        ...current,
                        reason_timeout: Number(event.target.value),
                      }))
                    }
                  />
                </Field>
              </div>

              <Separator />

              <div className="grid gap-3">
                <div>
                  <h3 className="text-xs font-semibold text-foreground">
                    {t("settings.capsSection")}
                  </h3>
                  <p className="mt-0.5 text-xs text-muted-foreground">{t("settings.capsHint")}</p>
                </div>
                <div className="grid gap-4 sm:grid-cols-3">
                  <Field>
                    <FieldLabel htmlFor="max-workers">{t("settings.maxWorkers")}</FieldLabel>
                    <Input
                      id="max-workers"
                      type="number"
                      min={0}
                      value={server.max_workers}
                      onChange={(event) =>
                        setServer((current) => ({
                          ...current,
                          max_workers: Number(event.target.value),
                        }))
                      }
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="max-running-projects">
                      {t("settings.maxRunningProjects")}
                    </FieldLabel>
                    <Input
                      id="max-running-projects"
                      type="number"
                      min={0}
                      value={server.max_running_projects}
                      onChange={(event) =>
                        setServer((current) => ({
                          ...current,
                          max_running_projects: Number(event.target.value),
                        }))
                      }
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="max-project-workers">
                      {t("settings.maxProjectWorkers")}
                    </FieldLabel>
                    <Input
                      id="max-project-workers"
                      type="number"
                      min={0}
                      value={server.max_project_workers}
                      onChange={(event) =>
                        setServer((current) => ({
                          ...current,
                          max_project_workers: Number(event.target.value),
                        }))
                      }
                    />
                  </Field>
                </div>
              </div>
            </FramePanel>
            <FrameFooter className="flex justify-end py-3">
              <Button onClick={persistServer} disabled={saving} loading={saving}>
                <Check className="size-4" />
                {t("settings.saveServer")}
              </Button>
            </FrameFooter>
          </Frame>

          <Frame>
            <FrameHeader>
              <FrameTitle>{t("settings.browserSection")}</FrameTitle>
              <FrameDescription className="text-xs">{t("settings.storedLocally")}</FrameDescription>
            </FrameHeader>
            <FramePanel className="grid gap-4">
              <Field>
                <FieldLabel>{t("theme.label")}</FieldLabel>
                <div
                  role="group"
                  aria-label={t("theme.label")}
                  className="grid grid-cols-3 gap-1 rounded-lg bg-muted p-1"
                >
                  {THEME_MODES.map((mode) => {
                    const Icon = THEME_ICONS[mode];
                    const active = theme === mode;
                    return (
                      <button
                        key={mode}
                        type="button"
                        onClick={() => setTheme(mode)}
                        aria-pressed={active}
                        className={cn(
                          "flex h-8 items-center justify-center gap-1.5 rounded-md px-2 text-xs font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
                          active
                            ? "bg-background text-foreground shadow-xs/5"
                            : "text-muted-foreground hover:text-foreground",
                        )}
                      >
                        <Icon className="size-3.5 shrink-0" />
                        <span>{t(`theme.${mode}`)}</span>
                      </button>
                    );
                  })}
                </div>
              </Field>

              <Field>
                <FieldLabel htmlFor="actor">{t("settings.actor")}</FieldLabel>
                <Input
                  id="actor"
                  value={actor}
                  placeholder={t("settings.actorPlaceholder")}
                  onChange={(event) => setActor(event.target.value)}
                />
              </Field>

              <Field>
                <FieldLabel>{t("settings.language")}</FieldLabel>
                <Select
                  items={localeItems}
                  value={currentLocale()}
                  onValueChange={(value) => {
                    if (value) setLocale(value as Locale);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectPopup>
                    {localeItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </Field>

              <Field>
                <FieldLabel>{t("settings.defaultLayout")}</FieldLabel>
                <Select
                  items={layoutItems}
                  value={layout}
                  onValueChange={(value) => {
                    if (value) setLayout(value as LayoutMode);
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectPopup>
                    {layoutItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>
              </Field>
            </FramePanel>
            <FrameFooter className="flex items-center justify-between gap-3 py-3">
              <div>
                {dirty && <Badge variant="warning">{t("settings.unsaved")}</Badge>}
              </div>
              <Button onClick={() => persistBrowser()} disabled={!dirty}>
                {t("settings.saveBrowser")}
              </Button>
            </FrameFooter>
          </Frame>
        </div>
      </div>
    </div>
  );
}
