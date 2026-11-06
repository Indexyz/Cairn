import { useEffect, useState } from "react";

import { api } from "@/lib/api";
import type { ServerSettings } from "@/lib/types";
import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { LAYOUT_MODES, type LayoutMode } from "@/lib/format";
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

const LAYOUT_LABELS: Record<LayoutMode, string> = {
  dagre_tb: "Dagre ↓",
  dagre_lr: "Dagre →",
  klay_tb: "Klay ↓",
  klay_lr: "Klay →",
  elk_tb: "ELK ↓",
  elk_lr: "ELK →",
};

export function SettingsDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { prefs, save } = usePrefs();
  const toast = useToast();
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
      toast.show("Settings saved");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription>
            Dispatcher timeouts are shared; actor and layout stay in this browser.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="intent-timeout">Intent timeout (seconds)</Label>
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
            <Label htmlFor="reason-timeout">Reason timeout (seconds)</Label>
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
            <Label htmlFor="actor">Actor</Label>
            <Input
              id="actor"
              value={actor}
              placeholder="Your name"
              onChange={(event) => setActor(event.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label>Default graph layout</Label>
            <Select value={layout} onValueChange={(value) => setLayout(value as LayoutMode)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LAYOUT_MODES.map((mode) => (
                  <SelectItem key={mode} value={mode}>
                    {LAYOUT_LABELS[mode]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={persist}>Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
