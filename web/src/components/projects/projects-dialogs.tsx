import type { Dispatch, SetStateAction } from "react";
import { Copy, Download, GitFork, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";
import { copyText } from "@/lib/api";
import { downloadText, snapshotFilename } from "@/lib/download";
import type { Hint } from "@/lib/types";

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
        <DialogPopup className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("projects.new.title")}</DialogTitle>
            <DialogDescription>{t("projects.new.description")}</DialogDescription>
          </DialogHeader>
          <DialogPanel className="grid gap-4">
            <Field>
              <FieldLabel htmlFor="new-project-title">{t("projects.new.projectTitle")}</FieldLabel>
              <Input
                id="new-project-title"
                value={newForm.title}
                onChange={(event) =>
                  setNewForm((form) => ({ ...form, title: event.target.value }))
                }
                placeholder={t("projects.new.projectTitle")}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="new-project-origin">{t("projects.new.origin")}</FieldLabel>
              <Textarea
                id="new-project-origin"
                value={newForm.origin}
                onChange={(event) =>
                  setNewForm((form) => ({ ...form, origin: event.target.value }))
                }
                placeholder={t("projects.new.origin")}
                rows={2}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="new-project-goal">{t("projects.new.goal")}</FieldLabel>
              <Textarea
                id="new-project-goal"
                value={newForm.goal}
                onChange={(event) =>
                  setNewForm((form) => ({ ...form, goal: event.target.value }))
                }
                placeholder={t("projects.new.goal")}
                rows={2}
              />
            </Field>
            <label className="flex cursor-pointer items-center justify-between gap-3 rounded-lg border bg-muted/40 px-3 py-2.5">
              <span className="text-sm font-medium text-foreground">
                {t("projects.new.bootstrap")}
              </span>
              <Switch
                checked={newForm.bootstrap}
                onCheckedChange={(checked) =>
                  setNewForm((form) => ({ ...form, bootstrap: checked }))
                }
              />
            </label>
            <Field>
              <FieldLabel htmlFor="new-max-steps">{t("projects.new.maxSteps")}</FieldLabel>
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
              <FieldDescription>{t("projects.new.maxStepsHint")}</FieldDescription>
            </Field>
            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <Label>{t("projects.new.hints")}</Label>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => {
                    const id = nextHintId.current++;
                    setNewForm((form) => ({
                      ...form,
                      hints: [...form.hints, { id, content: "" }],
                    }));
                  }}
                >
                  <Plus className="size-3.5" />
                  {t("common.add")}
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
              <p className="text-xs text-muted-foreground">
                {t("projects.new.hintCreator", { actor: actorName })}
              </p>
            </div>
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button
              onClick={createProject}
              disabled={!newForm.title || !newForm.origin || !newForm.goal}
            >
              {t("common.create")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={!!rename} onOpenChange={(open) => !open && setRename(null)}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("projects.rename.title")}</DialogTitle>
            <DialogDescription>
              {rename ? `${rename.id} - ${rename.title}` : ""}
            </DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <Field>
              <Input
                value={rename?.title ?? ""}
                onChange={(event) =>
                  setRename((current) =>
                    current ? { ...current, title: event.target.value } : current,
                  )
                }
              />
              <FieldDescription>{t("projects.rename.hint")}</FieldDescription>
            </Field>
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={renameProject} disabled={!rename?.title}>
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={!!reopen} onOpenChange={(open) => !open && setReopen(null)}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("projects.reopen.title")}</DialogTitle>
            <DialogDescription>
              {reopen ? `${reopen.id} - ${reopen.title}` : ""}
            </DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <Textarea
              value={reopenText}
              onChange={(event) => setReopenText(event.target.value)}
              placeholder={t("projects.reopen.placeholder")}
              rows={3}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={reopenProject} disabled={!reopenText}>
              {t("project.reopen")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={!!forking} onOpenChange={(open) => !open && setForking(null)}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("projects.fork.title")}</DialogTitle>
            <DialogDescription>
              {t("projects.fork.description", {
                label: forking ? `${forking.id} - ${forking.title}` : "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogPanel className="grid gap-4">
            <Field>
              <FieldLabel htmlFor="fork-title">{t("projects.fork.titleLabel")}</FieldLabel>
              <Input
                id="fork-title"
                value={forkTitle}
                onChange={(event) => setForkTitle(event.target.value)}
              />
            </Field>
            <div className="grid gap-2">
              <div className="flex items-center justify-between gap-2">
                <Label>{t("projects.fork.hintsLabel")}</Label>
                <div className="flex items-center gap-2 text-xs">
                  <Button
                    variant="link"
                    size="xs"
                    disabled={forkHintsLoading || forkSelected.size === forkHints.length}
                    onClick={() => setForkSelected(new Set(forkHints.map((hint) => hint.id)))}
                  >
                    {t("projects.fork.selectAll")}
                  </Button>
                  <Button
                    variant="link"
                    size="xs"
                    disabled={forkHintsLoading || forkSelected.size === 0}
                    onClick={() => setForkSelected(new Set())}
                  >
                    {t("projects.fork.selectNone")}
                  </Button>
                </div>
              </div>
              <div className="max-h-56 overflow-y-auto rounded-lg border bg-muted/32 p-1.5">
                {forkHintsLoading ? (
                  <p className="p-2 text-xs text-muted-foreground">{t("common.loading")}</p>
                ) : forkHintsFailed ? (
                  <p className="p-2 text-xs text-muted-foreground">
                    {t("projects.fork.hintsFailed")}
                  </p>
                ) : forkHints.length === 0 ? (
                  <p className="p-2 text-xs text-muted-foreground">{t("projects.fork.noHints")}</p>
                ) : (
                  forkHints.map((hint) => (
                    <label
                      key={hint.id}
                      className="flex cursor-pointer items-start gap-2.5 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-accent"
                    >
                      <Checkbox
                        className="mt-0.5"
                        checked={forkSelected.has(hint.id)}
                        onCheckedChange={() => toggleForkHint(hint.id)}
                      />
                      <span className="grid gap-0.5">
                        <span className="text-foreground">{hint.content}</span>
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
                  : t("projects.fork.hintsSelected", {
                      selected: forkSelected.size,
                      total: forkHints.length,
                    })}
              </p>
            </div>
          </DialogPanel>
          <DialogFooter>
            <Button variant="outline" onClick={() => setForking(null)} disabled={forkingBusy}>
              {t("common.cancel")}
            </Button>
            <Button
              onClick={forkProject}
              disabled={forkingBusy || forkHintsLoading}
              loading={forkingBusy}
            >
              <GitFork className="size-4" />
              {forkingBusy ? t("common.saving") : t("projects.fork.confirm")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <ConfirmDialog
        open={!!removal}
        title={t("projects.removal.title")}
        description={t("projects.removal.body", {
          label: removal ? `${removal.id} - ${removal.title}` : "",
        })}
        onCancel={() => setRemoval(null)}
        onConfirm={deleteProject}
        confirmDisabled={deleting}
      />

      <Dialog open={!!snapshot} onOpenChange={(open) => !open && setSnapshot(null)}>
        <DialogPopup className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("projects.snapshotTitle")}</DialogTitle>
            <DialogDescription>{snapshot?.title}</DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <div className="max-h-[60vh] overflow-auto rounded-xl border bg-code p-3.5 text-code-foreground">
              <pre className="font-mono text-xs leading-5">{snapshot?.text}</pre>
            </div>
          </DialogPanel>
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
                  .then(() => toast.show(t("common.copied"), "success"))
                  .catch(() => toast.show(t("common.copyFailed"), "error"))
              }
            >
              <Copy className="size-4" />
              {t("common.copy")}
            </Button>
            <DialogClose render={<Button />}>{t("common.close")}</DialogClose>
          </DialogFooter>
        </DialogPopup>
      </Dialog>
    </>
  );
}
