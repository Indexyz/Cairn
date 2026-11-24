import { useTranslation } from "react-i18next";
import { Copy, Download } from "lucide-react";

import { ConfirmDialog } from "@/components/confirm-dialog";
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
import { Field, FieldDescription } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

export interface ProjectDialogsProps {
  factsLabel: string;

  intentOpen: boolean;
  onIntentOpenChange: (open: boolean) => void;
  intentText: string;
  onIntentText: (value: string) => void;
  onDeclareIntent: () => void;
  onDeclareAndClaim: () => void;

  concludeTargetId: string | null;
  concludeText: string;
  onConcludeText: (value: string) => void;
  onCloseConclude: () => void;
  onConclude: () => void;

  completeOpen: boolean;
  onCompleteOpenChange: (open: boolean) => void;
  completeText: string;
  onCompleteText: (value: string) => void;
  onComplete: () => void;

  hintOpen: boolean;
  onHintOpenChange: (open: boolean) => void;
  hintText: string;
  onHintText: (value: string) => void;
  onAddHint: () => void;

  renameOpen: boolean;
  onRenameOpenChange: (open: boolean) => void;
  renameText: string;
  onRenameText: (value: string) => void;
  onRename: () => void;

  maxStepsOpen: boolean;
  onMaxStepsOpenChange: (open: boolean) => void;
  maxStepsText: string;
  onMaxStepsText: (value: string) => void;
  onSaveMaxSteps: () => void;

  reopenOpen: boolean;
  onReopenOpenChange: (open: boolean) => void;
  reopenText: string;
  onReopenText: (value: string) => void;
  onReopen: () => void;

  deleteOpen: boolean;
  onDeleteOpenChange: (open: boolean) => void;
  onDelete: () => void;

  projectId?: string;
  snapshot: string | null;
  onCloseSnapshot: () => void;
  onDownloadSnapshot: () => void;
  onCopySnapshot: () => void;
}

export function ProjectDialogs(props: ProjectDialogsProps) {
  const { t } = useTranslation();

  return (
    <>
      <Dialog open={props.intentOpen} onOpenChange={props.onIntentOpenChange}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.intent.title")}</DialogTitle>
            <DialogDescription>
              {t("project.dialogs.intent.from", { facts: props.factsLabel })}
            </DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <Textarea
              value={props.intentText}
              onChange={(event) => props.onIntentText(event.target.value)}
              placeholder={t("project.dialogs.intent.placeholder")}
              rows={3}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button
              variant="secondary"
              onClick={props.onDeclareIntent}
              disabled={!props.intentText}
            >
              {t("project.dialogs.intent.declare")}
            </Button>
            <Button onClick={props.onDeclareAndClaim} disabled={!props.intentText}>
              {t("project.dialogs.intent.declareClaim")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog
        open={props.concludeTargetId !== null}
        onOpenChange={(open) => !open && props.onCloseConclude()}
      >
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.conclude.title")}</DialogTitle>
            <DialogDescription>{props.concludeTargetId}</DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <Textarea
              value={props.concludeText}
              onChange={(event) => props.onConcludeText(event.target.value)}
              placeholder={t("project.dialogs.conclude.placeholder")}
              rows={3}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={props.onConclude} disabled={!props.concludeText}>
              {t("project.actions.conclude")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={props.completeOpen} onOpenChange={props.onCompleteOpenChange}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.complete.title")}</DialogTitle>
            <DialogDescription>
              {t("project.dialogs.intent.from", { facts: props.factsLabel })}
            </DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <Textarea
              value={props.completeText}
              onChange={(event) => props.onCompleteText(event.target.value)}
              placeholder={t("project.dialogs.complete.placeholder")}
              rows={2}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={props.onComplete} disabled={!props.completeText}>
              {t("project.actions.complete")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={props.hintOpen} onOpenChange={props.onHintOpenChange}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.hint.title")}</DialogTitle>
          </DialogHeader>
          <DialogPanel>
            <Textarea
              value={props.hintText}
              onChange={(event) => props.onHintText(event.target.value)}
              placeholder={t("project.dialogs.hint.placeholder")}
              rows={3}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={props.onAddHint} disabled={!props.hintText}>
              {t("common.add")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={props.renameOpen} onOpenChange={props.onRenameOpenChange}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.rename.title")}</DialogTitle>
          </DialogHeader>
          <DialogPanel>
            <Input
              value={props.renameText}
              onChange={(event) => props.onRenameText(event.target.value)}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={props.onRename} disabled={!props.renameText}>
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={props.maxStepsOpen} onOpenChange={props.onMaxStepsOpenChange}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.maxSteps.title")}</DialogTitle>
            <DialogDescription>{t("project.maxSteps.description")}</DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <Field>
              <Input
                type="number"
                min={1}
                value={props.maxStepsText}
                placeholder={t("project.maxSteps.placeholder")}
                onChange={(event) => props.onMaxStepsText(event.target.value)}
              />
              <FieldDescription>{t("project.maxSteps.hint")}</FieldDescription>
            </Field>
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={props.onSaveMaxSteps}>{t("common.save")}</Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <Dialog open={props.reopenOpen} onOpenChange={props.onReopenOpenChange}>
        <DialogPopup>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.reopen.title")}</DialogTitle>
          </DialogHeader>
          <DialogPanel>
            <Textarea
              value={props.reopenText}
              onChange={(event) => props.onReopenText(event.target.value)}
              placeholder={t("project.dialogs.reopen.placeholder")}
              rows={3}
            />
          </DialogPanel>
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>{t("common.cancel")}</DialogClose>
            <Button onClick={props.onReopen} disabled={!props.reopenText}>
              {t("project.reopen")}
            </Button>
          </DialogFooter>
        </DialogPopup>
      </Dialog>

      <ConfirmDialog
        open={props.deleteOpen}
        title={t("project.dialogs.remove.title")}
        description={t("project.dialogs.remove.body")}
        onCancel={() => props.onDeleteOpenChange(false)}
        onConfirm={props.onDelete}
      />

      <Dialog
        open={props.snapshot !== null}
        onOpenChange={(open) => !open && props.onCloseSnapshot()}
      >
        <DialogPopup className="sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("project.snapshot")}</DialogTitle>
            <DialogDescription>{props.projectId}</DialogDescription>
          </DialogHeader>
          <DialogPanel>
            <div className="max-h-[60vh] overflow-auto rounded-xl border bg-code p-3.5 text-code-foreground">
              <pre className="font-mono text-xs leading-5">{props.snapshot}</pre>
            </div>
          </DialogPanel>
          <DialogFooter>
            <Button variant="outline" onClick={props.onDownloadSnapshot}>
              <Download className="size-4" />
              {t("common.download")}
            </Button>
            <Button variant="outline" onClick={props.onCopySnapshot}>
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
