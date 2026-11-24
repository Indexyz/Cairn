import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pencil, Plus, Search, Trash2 } from "lucide-react";

import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import type { MCPServer, MCPTransport } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
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
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export const MCP_TRANSPORTS: MCPTransport[] = ["stdio", "http", "sse"];

interface PairRow {
  id: number;
  key: string;
  value: string;
}

interface MCPForm {
  name: string;
  type: MCPTransport;
  command: string;
  args: string;
  url: string;
  env: PairRow[];
  headers: PairRow[];
}

const NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_.-]*$/;

let nextPairId = 1;

function emptyForm(): MCPForm {
  return { name: "", type: "stdio", command: "", args: "", url: "", env: [], headers: [] };
}

function pairsOf(values: Record<string, string>): PairRow[] {
  return Object.entries(values).map(([key, value]) => ({ id: nextPairId++, key, value }));
}

function toForm(server: MCPServer): MCPForm {
  return {
    name: server.name,
    type: server.type,
    command: server.command,
    args: server.args.join("\n"),
    url: server.url,
    env: pairsOf(server.env),
    headers: pairsOf(server.headers),
  };
}

function toRecord(rows: PairRow[]): Record<string, string> {
  const record: Record<string, string> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (key) record[key] = row.value;
  }
  return record;
}

function payloadOf(form: MCPForm) {
  const stdio = form.type === "stdio";
  return {
    name: form.name.trim(),
    type: form.type,
    command: stdio ? form.command.trim() : "",
    args: stdio
      ? form.args
          .split("\n")
          .map((line) => line.trim())
          .filter(Boolean)
      : [],
    url: stdio ? "" : form.url.trim(),
    env: toRecord(form.env),
    headers: stdio ? {} : toRecord(form.headers),
  };
}

export function useMCPPage() {
  const toast = useToast();
  const { t } = useTranslation();
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [query, setQuery] = useState("");
  const [form, setForm] = useState<MCPForm | null>(null);
  const [originalName, setOriginalName] = useState<string | null>(null);
  const [removal, setRemoval] = useState<MCPServer | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    try {
      setServers(await api<MCPServer[]>("GET", "/mcp"));
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
    if (!needle) return servers;
    return servers.filter(
      (server) =>
        server.name.toLowerCase().includes(needle) ||
        server.command.toLowerCase().includes(needle) ||
        server.url.toLowerCase().includes(needle),
    );
  }, [servers, query]);

  function openCreate() {
    setOriginalName(null);
    setForm(emptyForm());
  }

  function openEdit(server: MCPServer) {
    setOriginalName(server.name);
    setForm(toForm(server));
  }

  function updatePair(list: "env" | "headers", id: number, patch: Partial<PairRow>) {
    setForm((current) =>
      current ? { ...current, [list]: current[list].map((row) => (row.id === id ? { ...row, ...patch } : row)) } : current,
    );
  }

  function addPair(list: "env" | "headers") {
    setForm((current) => (current ? { ...current, [list]: [...current[list], { id: nextPairId++, key: "", value: "" }] } : current));
  }

  function removePair(list: "env" | "headers", id: number) {
    setForm((current) => (current ? { ...current, [list]: current[list].filter((row) => row.id !== id) } : current));
  }

  async function save() {
    if (!form || saving) return;
    const payload = payloadOf(form);
    setSaving(true);
    try {
      if (originalName === null) await api("POST", "/mcp", payload);
      else await api("PUT", `/mcp/${encodeURIComponent(originalName)}`, payload);
      toast.show(t(originalName === null ? "mcp.toast.created" : "mcp.toast.updated"));
      setForm(null);
      setOriginalName(null);
      await load();
    } catch (error) {
      toast.show((error as Error).message, "error");
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    if (!removal) return;
    try {
      await api("DELETE", `/mcp/${encodeURIComponent(removal.name)}`);
      toast.show(t("mcp.toast.deleted", { name: removal.name }));
      setRemoval(null);
      await load();
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  const valid = Boolean(
    form &&
      NAME_PATTERN.test(form.name.trim()) &&
      (form.type === "stdio" ? form.command.trim() : /^https?:\/\/.+/.test(form.url.trim())),
  );

  return {
    addPair,
    filtered,
    form,
    load,
    openCreate,
    openEdit,
    originalName,
    query,
    removal,
    remove,
    removePair,
    save,
    saving,
    servers,
    setForm,
    setQuery,
    setRemoval,
    updatePair,
    valid,
  };
}

export function MCPPage() {
  const {
    addPair,
    filtered,
    form,
    openCreate,
    openEdit,
    originalName,
    query,
    removal,
    remove,
    removePair,
    save,
    saving,
    servers,
    setForm,
    setQuery,
    setRemoval,
    updatePair,
    valid,
  } = useMCPPage();
  const { t } = useTranslation();

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-3 border-b bg-background px-5">
        <div>
          <h1 className="text-sm font-semibold">{t("mcp.title")}</h1>
          <p className="text-xs text-muted-foreground">{t("mcp.subtitle")}</p>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="mx-auto w-full max-w-5xl">
          <div className="rounded-xl border bg-card">
            <div className="flex flex-wrap items-center gap-2 border-b p-3">
              <div className="relative min-w-60 flex-1">
                <Search className="absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t("mcp.searchPlaceholder")}
                  className="pl-8"
                />
              </div>
              <span className="text-xs text-muted-foreground">
                {t("mcp.count", { shown: filtered.length, total: servers.length })}
              </span>
              <div className="flex-1" />
              <Button size="sm" onClick={openCreate}>
                <Plus className="size-4" />
                {t("mcp.newServer")}
              </Button>
            </div>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-48">{t("mcp.header.name")}</TableHead>
                  <TableHead className="w-24">{t("mcp.header.type")}</TableHead>
                  <TableHead>{t("mcp.header.target")}</TableHead>
                  <TableHead className="w-20 text-right">{t("mcp.header.env")}</TableHead>
                  <TableHead className="w-20 text-right">{t("mcp.header.headers")}</TableHead>
                  <TableHead className="w-24 text-right">{t("mcp.header.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="py-16 text-center text-muted-foreground">
                      {t("mcp.empty")}
                    </TableCell>
                  </TableRow>
                )}
                {filtered.map((server) => (
                  <TableRow key={server.name}>
                    <TableCell className="font-mono text-xs">
                      <span className="rounded bg-muted px-1.5 py-0.5">{server.name}</span>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">{server.type}</Badge>
                    </TableCell>
                    <TableCell className="truncate font-mono text-xs text-muted-foreground">
                      {server.type === "stdio" ? [server.command, ...server.args].join(" ") : server.url}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{Object.keys(server.env).length}</TableCell>
                    <TableCell className="text-right tabular-nums">{Object.keys(server.headers).length}</TableCell>
                    <TableCell>
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="icon-sm" title={t("common.edit")} onClick={() => openEdit(server)}>
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          className="text-destructive hover:text-destructive"
                          title={t("common.delete")}
                          onClick={() => setRemoval(server)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="border-t px-3 py-2 text-xs text-muted-foreground">{t("mcp.footer")}</div>
          </div>
        </div>
      </div>

      <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
        <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t(originalName ? "mcp.form.editTitle" : "mcp.form.newTitle")}</DialogTitle>
            <DialogDescription>{t("mcp.form.description")}</DialogDescription>
          </DialogHeader>

          {form && (
            <div className="grid gap-4">
              <div className="grid gap-2 sm:grid-cols-2">
                <div className="grid gap-2">
                  <Label htmlFor="mcp-name">{t("mcp.form.name")}</Label>
                  <Input
                    id="mcp-name"
                    value={form.name}
                    placeholder="filesystem"
                    onChange={(event) => setForm((current) => (current ? { ...current, name: event.target.value } : current))}
                  />
                  {form.name.trim() !== "" && !NAME_PATTERN.test(form.name.trim()) && (
                    <p className="text-[11px] text-destructive">{t("mcp.form.nameHint")}</p>
                  )}
                </div>
                <div className="grid gap-2">
                  <Label>{t("mcp.form.type")}</Label>
                  <Select
                    value={form.type}
                    onValueChange={(value) =>
                      setForm((current) => (current ? { ...current, type: value as MCPTransport } : current))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MCP_TRANSPORTS.map((item) => (
                        <SelectItem key={item} value={item}>
                          {item}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              {form.type === "stdio" ? (
                <>
                  <div className="grid gap-2">
                    <Label htmlFor="mcp-command">{t("mcp.form.command")}</Label>
                    <Input
                      id="mcp-command"
                      value={form.command}
                      placeholder="npx"
                      onChange={(event) =>
                        setForm((current) => (current ? { ...current, command: event.target.value } : current))
                      }
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="mcp-args">{t("mcp.form.args")}</Label>
                    <Textarea
                      id="mcp-args"
                      value={form.args}
                      placeholder={t("mcp.form.argsPlaceholder")}
                      className="min-h-20 font-mono text-xs"
                      onChange={(event) => setForm((current) => (current ? { ...current, args: event.target.value } : current))}
                    />
                  </div>
                </>
              ) : (
                <div className="grid gap-2">
                  <Label htmlFor="mcp-url">{t("mcp.form.url")}</Label>
                  <Input
                    id="mcp-url"
                    value={form.url}
                    placeholder="https://mcp.example.com/sse"
                    onChange={(event) => setForm((current) => (current ? { ...current, url: event.target.value } : current))}
                  />
                </div>
              )}

              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <Label>{t("mcp.form.env")}</Label>
                  <Button variant="link" size="sm" onClick={() => addPair("env")}>
                    + {t("common.add")}
                  </Button>
                </div>
                {form.env.map((row) => (
                  <div key={row.id} className="flex items-center gap-2">
                    <Input
                      value={row.key}
                      placeholder={t("mcp.form.keyPlaceholder")}
                      className="font-mono text-xs"
                      onChange={(event) => updatePair("env", row.id, { key: event.target.value })}
                    />
                    <Input
                      value={row.value}
                      placeholder={t("mcp.form.valuePlaceholder")}
                      className="font-mono text-xs"
                      onChange={(event) => updatePair("env", row.id, { value: event.target.value })}
                    />
                    <Button variant="ghost" size="icon-sm" onClick={() => removePair("env", row.id)}>
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                ))}
              </div>

              {form.type !== "stdio" && (
                <div className="grid gap-2">
                  <div className="flex items-center justify-between">
                    <Label>{t("mcp.form.headers")}</Label>
                    <Button variant="link" size="sm" onClick={() => addPair("headers")}>
                      + {t("common.add")}
                    </Button>
                  </div>
                  {form.headers.map((row) => (
                    <div key={row.id} className="flex items-center gap-2">
                      <Input
                        value={row.key}
                        placeholder={t("mcp.form.keyPlaceholder")}
                        className="font-mono text-xs"
                        onChange={(event) => updatePair("headers", row.id, { key: event.target.value })}
                      />
                      <Input
                        value={row.value}
                        placeholder={t("mcp.form.valuePlaceholder")}
                        className="font-mono text-xs"
                        onChange={(event) => updatePair("headers", row.id, { value: event.target.value })}
                      />
                      <Button variant="ghost" size="icon-sm" onClick={() => removePair("headers", row.id)}>
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => setForm(null)} disabled={saving}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => void save()} disabled={!valid || saving}>
              {saving ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!removal}
        title={t("mcp.removal.title")}
        description={t("mcp.removal.body", { name: removal?.name ?? "" })}
        onCancel={() => setRemoval(null)}
        onConfirm={() => void remove()}
      />
    </div>
  );
}
