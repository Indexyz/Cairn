import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Bell, FlaskConical, Pencil, Plus, Trash2 } from "lucide-react";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { Badge } from "@/components/ui/badge";
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
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Field, FieldLabel } from "@/components/ui/field";
import { Frame, FrameFooter, FramePanel } from "@/components/ui/frame";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
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
import type {
  NotificationChannel,
  NotificationChannelPayload,
  NotificationEvent,
  NotificationKind,
  NotificationTestResult,
} from "@/lib/types";

const KINDS: NotificationKind[] = ["discord", "telegram", "webhook"];
const EVENTS: NotificationEvent[] = [
  "project_created",
  "project_stopped",
  "project_resumed",
  "project_blocked",
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

export function useNotificationsPage() {
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
      toast.show(
        t(enabled ? "notifications.toast.enabled" : "notifications.toast.disabled", {
          name: channel.name,
        }),
        "success",
      );
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
      toast.show(
        t(editing ? "notifications.toast.updated" : "notifications.toast.created"),
        "success",
      );
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
      if (result.ok) toast.show(t("notifications.test.okToast", { name: channel.name }), "success");
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
      toast.show(t("notifications.toast.deleted", { name: removal.name }), "success");
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

  return {
    channels,
    editing,
    form,
    openCreate,
    openEdit,
    removal,
    remove,
    save,
    saving,
    setForm,
    setRemoval,
    test,
    testing,
    toggle,
    valid,
  };
}

export function NotificationsPage() {
  const {
    channels,
    editing,
    form,
    openCreate,
    openEdit,
    removal,
    remove,
    save,
    saving,
    setForm,
    setRemoval,
    test,
    testing,
    toggle,
    valid,
  } = useNotificationsPage();
  const { t } = useTranslation();

  const kindItems = useMemo(
    () => KINDS.map((kind) => ({ value: kind, label: t(`notifications.kinds.${kind}`) })),
    [t],
  );

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b bg-background px-6 py-4">
        <div className="min-w-0">
          <h1 className="font-heading text-lg font-semibold tracking-tight text-foreground">
            {t("notifications.title")}
          </h1>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("notifications.subtitle")}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="size-4" />
          {t("notifications.newChannel")}
        </Button>
      </header>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex w-full max-w-5xl flex-col gap-4">
          <Frame>
            <FramePanel className="overflow-hidden p-0">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-56">{t("notifications.header.name")}</TableHead>
                    <TableHead className="w-28">{t("notifications.header.kind")}</TableHead>
                    <TableHead>{t("notifications.header.events")}</TableHead>
                    <TableHead className="w-24">{t("notifications.header.enabled")}</TableHead>
                    <TableHead className="w-32 text-right">
                      {t("notifications.header.actions")}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {channels.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={5} className="p-0">
                        <Empty className="py-12">
                          <EmptyHeader>
                            <EmptyMedia variant="icon">
                              <Bell />
                            </EmptyMedia>
                            <EmptyTitle>{t("notifications.empty")}</EmptyTitle>
                          </EmptyHeader>
                        </Empty>
                      </TableCell>
                    </TableRow>
                  ) : (
                    channels.map((channel) => (
                      <TableRow key={channel.id}>
                        <TableCell className="font-medium">
                          <div className="truncate text-foreground">{channel.name}</div>
                          <div className="font-mono text-[11px] text-muted-foreground">
                            {channel.id}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{channel.kind}</Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {channel.events.map((event) => (
                              <Badge key={event} variant="outline" size="sm">
                                {t(`notifications.events.${event}`)}
                              </Badge>
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
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    title={t("notifications.test.action")}
                                    aria-label={t("notifications.test.action")}
                                    disabled={testing !== null}
                                    onClick={() => void test(channel)}
                                  />
                                }
                              >
                                {testing === channel.id ? (
                                  <Spinner className="size-3.5" />
                                ) : (
                                  <FlaskConical className="size-3.5" />
                                )}
                              </TooltipTrigger>
                              <TooltipPopup>{t("notifications.test.action")}</TooltipPopup>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <Button
                                    variant="ghost"
                                    size="icon-xs"
                                    title={t("common.edit")}
                                    aria-label={t("common.edit")}
                                    onClick={() => openEdit(channel)}
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
                                    onClick={() => setRemoval(channel)}
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
            <FrameFooter className="py-2.5 text-xs text-muted-foreground">
              {t("notifications.count", { count: channels.length })}
            </FrameFooter>
          </Frame>
        </div>
      </div>

      <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
        <DialogPopup className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {t(editing ? "notifications.form.editTitle" : "notifications.form.newTitle")}
            </DialogTitle>
            <DialogDescription>{t("notifications.form.description")}</DialogDescription>
          </DialogHeader>
          {form && (
            <DialogPanel className="grid gap-4">
              <Field>
                <FieldLabel htmlFor="channel-name">{t("notifications.form.name")}</FieldLabel>
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
              </Field>

              <div className="grid gap-3 sm:grid-cols-2">
                <Field>
                  <FieldLabel>{t("notifications.form.kind")}</FieldLabel>
                  <Select
                    items={kindItems}
                    value={form.kind}
                    onValueChange={(value) => {
                      if (!value) return;
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
                      });
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectPopup>
                      {kindItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectPopup>
                  </Select>
                </Field>
                <Field>
                  <FieldLabel htmlFor="channel-enabled">{t("notifications.form.enabled")}</FieldLabel>
                  <div className="flex h-8.5 items-center sm:h-8">
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
                </Field>
              </div>

              <div className="grid gap-3">
                {KIND_FIELDS[form.kind].map((key) => (
                  <Field key={key}>
                    <FieldLabel htmlFor={`channel-${key}`}>
                      {t(`notifications.fields.${key}`)}
                    </FieldLabel>
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
                  </Field>
                ))}
                <p className="text-xs text-muted-foreground">{t(KIND_HINTS[form.kind])}</p>
              </div>

              <div className="grid gap-2">
                <Label>{t("notifications.form.events")}</Label>
                <div className="flex flex-wrap gap-1.5">
                  {EVENTS.map((event) => {
                    const checked = form.events.includes(event);
                    return (
                      <Button
                        key={event}
                        type="button"
                        size="sm"
                        variant={checked ? "default" : "outline"}
                        aria-pressed={checked}
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
                <p className="text-xs text-muted-foreground">{t("notifications.form.eventsNote")}</p>
              </div>
            </DialogPanel>
          )}
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={save} disabled={!valid || saving} loading={saving}>
              {saving ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <ConfirmDialog
        open={!!removal}
        title={t("notifications.removal.title")}
        description={t("notifications.removal.body", { name: removal?.name ?? "" })}
        onCancel={() => setRemoval(null)}
        onConfirm={remove}
      />
    </div>
  );
}
