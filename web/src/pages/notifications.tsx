import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { FlaskConical, Loader2, Pencil, Plus, Trash2 } from "lucide-react";

import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import type {
  NotificationChannel,
  NotificationChannelPayload,
  NotificationEvent,
  NotificationKind,
  NotificationTestResult,
} from "@/lib/types";
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const KINDS: NotificationKind[] = ["discord", "telegram", "webhook"];
const EVENTS: NotificationEvent[] = [
  "project_created",
  "project_stopped",
  "project_resumed",
  "project_completed",
  "project_reopened",
];

const KIND_FIELDS: Record<NotificationKind, string[]> = {
  discord: ["webhook_url"],
  telegram: ["bot_token", "chat_id"],
  webhook: ["url"],
};

const KIND_HINTS: Record<NotificationKind, string> = {
  discord: "notifications.form.discordHint",
  telegram: "notifications.form.telegramHint",
  webhook: "notifications.form.webhookHint",
};

interface ChannelForm {
  name: string;
  kind: NotificationKind;
  enabled: boolean;
  events: NotificationEvent[];
  config: Record<string, string>;
}

function emptyForm(): ChannelForm {
  return {
    name: "",
    kind: "discord",
    enabled: true,
    events: [...EVENTS],
    config: Object.fromEntries(KIND_FIELDS.discord.map((key) => [key, ""])),
  };
}

function toForm(channel: NotificationChannel): ChannelForm {
  return {
    name: channel.name,
    kind: channel.kind,
    enabled: channel.enabled,
    events: [...channel.events],
    config: { ...channel.config },
  };
}

export function NotificationsPage() {
  const toast = useToast();
  const { t } = useTranslation();
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [form, setForm] = useState<ChannelForm | null>(null);
  const [editing, setEditing] = useState<NotificationChannel | null>(null);
  const [removal, setRemoval] = useState<NotificationChannel | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);

  async function load() {
    try {
      setChannels(await api<NotificationChannel[]>("GET", "/notifications"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function openCreate() {
    setEditing(null);
    setForm(emptyForm());
  }

  function openEdit(channel: NotificationChannel) {
    setEditing(channel);
    setForm(toForm(channel));
  }

  async function toggle(channel: NotificationChannel, enabled: boolean) {
    setChannels((current) =>
      current.map((item) => (item.id === channel.id ? { ...item, enabled } : item)),
    );
    try {
      await api("PUT", `/notifications/${channel.id}`, { ...channel, enabled });
      toast.show(t(enabled ? "notifications.toast.enabled" : "notifications.toast.disabled", {
        name: channel.name,
      }));
    } catch (error) {
      toast.show((error as Error).message, "error");
      await load();
    }
  }

  async function save() {
    if (!form || saving) return;
    const payload: NotificationChannelPayload = {
      name: form.name.trim(),
      kind: form.kind,
      enabled: form.enabled,
      events: form.events,
      config: form.config,
    };
    setSaving(true);
    try {
      if (editing) await api("PUT", `/notifications/${editing.id}`, payload);
      else await api("POST", "/notifications", payload);
      setForm(null);
      setEditing(null);
      await load();
      toast.show(t(editing ? "notifications.toast.updated" : "notifications.toast.created"));
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setSaving(false);
    }
  }

  async function test(channel: NotificationChannel) {
    if (testing) return;
    setTesting(channel.id);
    try {
      const result = await api<NotificationTestResult>(
        "POST",
        `/notifications/${channel.id}/test`,
      );
      if (result.ok) toast.show(t("notifications.test.okToast", { name: channel.name }));
      else
        toast.show(
          t("notifications.test.failToast", {
            name: channel.name,
            status: result.status ? ` (${result.status})` : "",
          }),
          "error",
        );
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setTesting(null);
    }
  }

  async function remove() {
    if (!removal) return;
    try {
      await api("DELETE", `/notifications/${removal.id}`);
      setRemoval(null);
      await load();
      toast.show(t("notifications.toast.deleted", { name: removal.name }));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  const valid = Boolean(
    form &&
      form.name.trim() &&
      form.events.length > 0 &&
      KIND_FIELDS[form.kind].every((key) => form.config[key]?.trim()),
  );

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("notifications.title")}</h1>
          <p className="text-xs text-muted-foreground">{t("notifications.subtitle")}</p>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="mx-auto w-full max-w-5xl">
          <div className="rounded-xl border bg-card">
            <div className="flex flex-wrap items-center gap-2 border-b p-3">
              <span className="text-xs text-muted-foreground">
                {t("notifications.count", { count: channels.length })}
              </span>
              <div className="flex-1" />
              <Button size="sm" onClick={openCreate}>
                <Plus className="size-4" />
                {t("notifications.newChannel")}
              </Button>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-56">{t("notifications.header.name")}</TableHead>
                  <TableHead className="w-28">{t("notifications.header.kind")}</TableHead>
                  <TableHead>{t("notifications.header.events")}</TableHead>
                  <TableHead className="w-24">{t("notifications.header.enabled")}</TableHead>
                  <TableHead className="w-32 text-right">{t("notifications.header.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {channels.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="py-16 text-center text-muted-foreground">
                      {t("notifications.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {channels.map((channel) => (
                  <TableRow key={channel.id}>
                    <TableCell className="font-medium">
                      <div className="truncate">{channel.name}</div>
                      <div className="font-mono text-[11px] text-muted-foreground">{channel.id}</div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">{channel.kind}</Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {channel.events.map((event) => (
                          <span
                            key={event}
                            className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground"
                          >
                            {t(`notifications.events.${event}`)}
                          </span>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={channel.enabled}
                        onCheckedChange={(checked) => void toggle(channel, checked)}
                        aria-label={t("notifications.header.enabled")}
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("notifications.test.action")}
                          disabled={testing !== null}
                          onClick={() => void test(channel)}
                        >
                          {testing === channel.id ? (
                            <Loader2 className="size-3.5 animate-spin" />
                          ) : (
                            <FlaskConical className="size-3.5" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          title={t("common.edit")}
                          onClick={() => openEdit(channel)}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          title={t("common.delete")}
                          onClick={() => setRemoval(channel)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="border-t px-3 py-2 text-xs text-muted-foreground">
              {t("notifications.footer")}
            </div>
          </div>
        </div>
      </div>

      <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
        <DialogContent className="max-h-[85vh] max-w-xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {t(form && editing ? "notifications.form.editTitle" : "notifications.form.newTitle")}
            </DialogTitle>
            <DialogDescription>{t("notifications.form.description")}</DialogDescription>
          </DialogHeader>
          {form && (
            <div className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="channel-name">{t("notifications.form.name")}</Label>
                <Input
                  id="channel-name"
                  value={form.name}
                  placeholder={t("notifications.form.namePlaceholder")}
                  onChange={(event) =>
                    setForm((current) =>
                      current ? { ...current, name: event.target.value } : current,
                    )
                  }
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                  <Label>{t("notifications.form.kind")}</Label>
                  <Select
                    value={form.kind}
                    onValueChange={(value) =>
                      setForm((current) => {
                        if (!current) return current;
                        const kind = value as NotificationKind;
                        return {
                          ...current,
                          kind,
                          config: Object.fromEntries(
                            KIND_FIELDS[kind].map((key) => [key, current.config[key] ?? ""]),
                          ),
                        };
                      })
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {KINDS.map((kind) => (
                        <SelectItem key={kind} value={kind}>
                          {t(`notifications.kinds.${kind}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="channel-enabled">{t("notifications.form.enabled")}</Label>
                  <div className="flex h-9 items-center">
                    <Switch
                      id="channel-enabled"
                      checked={form.enabled}
                      onCheckedChange={(checked) =>
                        setForm((current) =>
                          current ? { ...current, enabled: checked } : current,
                        )
                      }
                    />
                  </div>
                </div>
              </div>

              <div className="grid gap-2">
                {KIND_FIELDS[form.kind].map((key) => (
                  <div key={key} className="grid gap-2">
                    <Label htmlFor={`channel-${key}`}>{t(`notifications.fields.${key}`)}</Label>
                    <Input
                      id={`channel-${key}`}
                      value={form.config[key] ?? ""}
                      placeholder={t(`notifications.placeholders.${key}`)}
                      className="font-mono text-xs"
                      onChange={(event) =>
                        setForm((current) =>
                          current
                            ? {
                                ...current,
                                config: { ...current.config, [key]: event.target.value },
                              }
                            : current,
                        )
                      }
                    />
                  </div>
                ))}
                <p className="text-[11px] text-muted-foreground">{t(KIND_HINTS[form.kind])}</p>
              </div>

              <div className="grid gap-2">
                <Label>{t("notifications.form.events")}</Label>
                <div className="flex flex-wrap gap-2">
                  {EVENTS.map((event) => {
                    const checked = form.events.includes(event);
                    return (
                      <Button
                        key={event}
                        type="button"
                        size="sm"
                        variant={checked ? "default" : "outline"}
                        onClick={() =>
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  events: checked
                                    ? current.events.filter((item) => item !== event)
                                    : [...current.events, event],
                                }
                              : current,
                          )
                        }
                      >
                        {t(`notifications.events.${event}`)}
                      </Button>
                    );
                  })}
                </div>
                <p className="text-[11px] text-muted-foreground">{t("notifications.form.eventsNote")}</p>
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
            <DialogTitle>{t("notifications.removal.title")}</DialogTitle>
            <DialogDescription>
              {t("notifications.removal.body", { name: removal?.name ?? "" })}
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
