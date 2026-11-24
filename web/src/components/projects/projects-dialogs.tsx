import type { Dispatch, SetStateAction } from "react";
import { Download, GitFork } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useToast } from "@/hooks/use-toast";
import { copyText } from "@/lib/api";
import { downloadText, snapshotFilename } from "@/lib/download";
import type { Hint } from "@/lib/types";
import { cn } from "@/lib/utils";

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

interface ProjectsDialogsProps {
  newOpen: boolean;
  setNewOpen: Dispatch<SetStateAction<boolean>>;
  newForm: {
    title: string;
    origin: string;
    goal: string;
    bootstrap: boolean;
    maxSteps: string;
    hints: { id: number; content: string }[];
  };
  setNewForm: Dispatch<
    SetStateAction<{
      title: string;
      origin: string;
      goal: string;
      bootstrap: boolean;
      maxSteps: string;
      hints: { id: number; content: string }[];
    }>
  >;
  nextHintId: { current: number };
  actorName: string;
  createProject: () => void;
  rename: { id: string; title: string } | null;
  setRename: Dispatch<SetStateAction<{ id: string; title: string } | null>>;
  renameProject: () => void;
  reopen: { id: string; title: string } | null;
  setReopen: Dispatch<SetStateAction<{ id: string; title: string } | null>>;
  reopenText: string;
  setReopenText: Dispatch<SetStateAction<string>>;
  reopenProject: () => void;
  forking: { id: string; title: string } | null;
  setForking: Dispatch<SetStateAction<{ id: string; title: string } | null>>;
  forkTitle: string;
  setForkTitle: Dispatch<SetStateAction<string>>;
  forkHints: Hint[];
  forkSelected: Set<string>;
  setForkSelected: Dispatch<SetStateAction<Set<string>>>;
  forkHintsLoading: boolean;
  forkHintsFailed: boolean;
  toggleForkHint: (id: string) => void;
  forkProject: () => void;
  forkingBusy: boolean;
  removal: { id: string; title: string } | null;
  setRemoval: Dispatch<SetStateAction<{ id: string; title: string } | null>>;
  deleteProject: () => void;
  snapshot: { id: string; title: string; text: string } | null;
  setSnapshot: Dispatch<SetStateAction<{ id: string; title: string; text: string } | null>>;
  deleting: boolean;
}

export function ProjectsDialogs(props: ProjectsDialogsProps) {
  const {
    newOpen,
    setNewOpen,
    newForm,
    setNewForm,
    nextHintId,
    actorName,
    createProject,
    rename,
    setRename,
    renameProject,
    reopen,
    setReopen,
    reopenText,
    setReopenText,
    reopenProject,
    forking,
    setForking,
    forkTitle,
    setForkTitle,
    forkHints,
    forkSelected,
    setForkSelected,
    forkHintsLoading,
    forkHintsFailed,
    toggleForkHint,
    forkProject,
    forkingBusy,
    removal,
    setRemoval,
    deleteProject,
    snapshot,
    setSnapshot,
    deleting,
  } = props;
  const { t } = useTranslation();
  const toast = useToast();

  return (
    <>
      <Dialog open={newOpen} onOpenChange={setNewOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("projects.new.title")}</DialogTitle>
            <DialogDescription>{t("projects.new.description")}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <Input
              value={newForm.title}
              onChange={(event) => setNewForm((form) => ({ ...form, title: event.target.value }))}
              placeholder={t("projects.new.projectTitle")}
            />
            <Textarea
              value={newForm.origin}
              onChange={(event) => setNewForm((form) => ({ ...form, origin: event.target.value }))}
              placeholder={t("projects.new.origin")}
              rows={2}
            />
            <Textarea
              value={newForm.goal}
              onChange={(event) => setNewForm((form) => ({ ...form, goal: event.target.value }))}
              placeholder={t("projects.new.goal")}
              rows={2}
            />
            <div className="flex items-center gap-2">
              <Switch
                checked={newForm.bootstrap}
                onCheckedChange={(checked) => setNewForm((form) => ({ ...form, bootstrap: checked }))}
              />
              <Label>{t("projects.new.bootstrap")}</Label>
            </div>
            <div className="grid gap-1">
              <Label htmlFor="new-max-steps" className="text-xs text-muted-foreground">
                {t("projects.new.maxSteps")}
              </Label>
              <Input
                id="new-max-steps"
                type="number"
                min={1}
                value={newForm.maxSteps}
                placeholder={t("projects.new.maxStepsPlaceholder")}
                onChange={(event) =>
                  setNewForm((form) => ({ ...form, maxSteps: event.target.value }))
                }
              />
              <p className="text-[11px] text-muted-foreground">{t("projects.new.maxStepsHint")}</p>
            </div>
            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <Label className="text-xs text-muted-foreground">{t("projects.new.hints")}</Label>
                <Button
                  variant="link"
                  size="sm"
                  onClick={() => {
                    const id = nextHintId.current++;
                    setNewForm((form) => ({
                      ...form,
                      hints: [...form.hints, { id, content: "" }],
                    }));
                  }}
                >
                  + {t("common.add")}
                </Button>
              </div>
              {newForm.hints.map((hint) => (
                <Input
                  key={hint.id}
                  value={hint.content}
                  placeholder={t("projects.new.hintPlaceholder")}
                  onChange={(event) =>
                    setNewForm((form) => ({
                      ...form,
                      hints: form.hints.map((item) =>
                        item.id === hint.id ? { ...item, content: event.target.value } : item,
                      ),
                    }))
                  }
                />
              ))}
              <p className="text-[11px] text-muted-foreground">
                {t("projects.new.hintCreator", { actor: actorName })}
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              onClick={createProject}
              disabled={!newForm.title || !newForm.origin || !newForm.goal}
            >
              {t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!rename} onOpenChange={(open) => !open && setRename(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.rename.title")}</DialogTitle>
            <DialogDescription>{rename ? `${rename.id} - ${rename.title}` : ""}</DialogDescription>
          </DialogHeader>
          <Input
            value={rename?.title ?? ""}
            onChange={(event) =>
              setRename((current) => (current ? { ...current, title: event.target.value } : current))
            }
          />
          <p className="text-[11px] text-muted-foreground">{t("projects.rename.hint")}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRename(null)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={renameProject} disabled={!rename?.title}>
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!reopen} onOpenChange={(open) => !open && setReopen(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.reopen.title")}</DialogTitle>
            <DialogDescription>{reopen ? `${reopen.id} - ${reopen.title}` : ""}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={reopenText}
            onChange={(event) => setReopenText(event.target.value)}
            placeholder={t("projects.reopen.placeholder")}
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setReopen(null)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={reopenProject} disabled={!reopenText}>
              {t("project.reopen")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!forking} onOpenChange={(open) => !open && setForking(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.fork.title")}</DialogTitle>
            <DialogDescription>
              {t("projects.fork.description", { label: forking ? `${forking.id} - ${forking.title}` : "" })}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="fork-title">{t("projects.fork.titleLabel")}</Label>
            <Input id="fork-title" value={forkTitle} onChange={(event) => setForkTitle(event.target.value)} />
          </div>
          <div className="grid gap-2">
            <div className="flex items-center justify-between gap-2">
              <Label>{t("projects.fork.hintsLabel")}</Label>
              <div className="flex items-center gap-3 text-xs">
                <button
                  type="button"
                  className="text-muted-foreground underline-offset-2 hover:underline disabled:opacity-50"
                  disabled={forkHintsLoading || forkSelected.size === forkHints.length}
                  onClick={() => setForkSelected(new Set(forkHints.map((hint) => hint.id)))}
                >
                  {t("projects.fork.selectAll")}
                </button>
                <button
                  type="button"
                  className="text-muted-foreground underline-offset-2 hover:underline disabled:opacity-50"
                  disabled={forkHintsLoading || forkSelected.size === 0}
                  onClick={() => setForkSelected(new Set())}
                >
                  {t("projects.fork.selectNone")}
                </button>
              </div>
            </div>
            <div className="max-h-56 overflow-y-auto rounded-md border p-1.5">
              {forkHintsLoading ? (
                <p className="p-2 text-xs text-muted-foreground">{t("common.loading")}</p>
              ) : forkHintsFailed ? (
                <p className="p-2 text-xs text-muted-foreground">{t("projects.fork.hintsFailed")}</p>
              ) : forkHints.length === 0 ? (
                <p className="p-2 text-xs text-muted-foreground">{t("projects.fork.noHints")}</p>
              ) : (
                forkHints.map((hint) => (
                  <label
                    key={hint.id}
                    className="flex cursor-pointer items-start gap-2 rounded px-1.5 py-1 text-xs hover:bg-muted"
                  >
                    <input
                      type="checkbox"
                      className="mt-0.5 size-3.5 shrink-0 accent-primary"
                      checked={forkSelected.has(hint.id)}
                      onChange={() => toggleForkHint(hint.id)}
                    />
                    <span className="grid gap-0.5">
                      <span>{hint.content}</span>
                      <span className="text-muted-foreground">
                        {hint.creator} · {hint.created_at}
                      </span>
                    </span>
                  </label>
                ))
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {forkHintsFailed
                ? t("projects.fork.hintsAllCopied")
                : t("projects.fork.hintsSelected", { selected: forkSelected.size, total: forkHints.length })}
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setForking(null)} disabled={forkingBusy}>
              {t("common.cancel")}
            </Button>
            <Button onClick={forkProject} disabled={forkingBusy || forkHintsLoading}>
              <GitFork className="size-4" />
              {forkingBusy ? t("common.saving") : t("projects.fork.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!removal} onOpenChange={(open) => !open && setRemoval(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.removal.title")}</DialogTitle>
            <DialogDescription>
              {t("projects.removal.body", { label: removal ? `${removal.id} - ${removal.title}` : "" })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoval(null)} disabled={deleting}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={deleteProject} disabled={deleting}>
              {deleting ? t("common.deleting") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!snapshot} onOpenChange={(open) => !open && setSnapshot(null)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("projects.snapshotTitle")}</DialogTitle>
            <DialogDescription>{snapshot?.title}</DialogDescription>
          </DialogHeader>
          <div className={cn("max-h-[60vh] overflow-auto rounded-lg border bg-muted/40 p-3")}>
            <pre className="text-xs leading-5">{snapshot?.text}</pre>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => snapshot && downloadText(snapshotFilename(snapshot.id), snapshot.text)}
            >
              <Download className="size-4" />
              {t("common.download")}
            </Button>
            <Button
              variant="outline"
              onClick={() =>
                snapshot &&
                copyText(snapshot.text)
                  .then(() => toast.show(t("common.copied")))
                  .catch(() => toast.show(t("common.copyFailed"), "error"))
              }
            >
              {t("common.copy")}
            </Button>
            <Button onClick={() => setSnapshot(null)}>{t("common.close")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
