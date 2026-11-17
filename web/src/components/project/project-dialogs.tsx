import { useTranslation } from "react-i18next";
import { Copy, Download } from "lucide-react";

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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.intent.title")}</DialogTitle>
            <DialogDescription>
              {t("project.dialogs.intent.from", { facts: props.factsLabel })}
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={props.intentText}
            onChange={(event) => props.onIntentText(event.target.value)}
            placeholder={t("project.dialogs.intent.placeholder")}
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onIntentOpenChange(false)}>
              {t("common.cancel")}
            </Button>
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
        </DialogContent>
      </Dialog>

      <Dialog open={props.concludeTargetId !== null} onOpenChange={(open) => !open && props.onCloseConclude()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.conclude.title")}</DialogTitle>
            <DialogDescription>{props.concludeTargetId}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={props.concludeText}
            onChange={(event) => props.onConcludeText(event.target.value)}
            placeholder={t("project.dialogs.conclude.placeholder")}
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={props.onCloseConclude}>
              {t("common.cancel")}
            </Button>
            <Button onClick={props.onConclude} disabled={!props.concludeText}>
              {t("project.actions.conclude")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.completeOpen} onOpenChange={props.onCompleteOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.complete.title")}</DialogTitle>
            <DialogDescription>
              {t("project.dialogs.intent.from", { facts: props.factsLabel })}
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={props.completeText}
            onChange={(event) => props.onCompleteText(event.target.value)}
            placeholder={t("project.dialogs.complete.placeholder")}
            rows={2}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onCompleteOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={props.onComplete} disabled={!props.completeText}>
              {t("project.actions.complete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.hintOpen} onOpenChange={props.onHintOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.hint.title")}</DialogTitle>
          </DialogHeader>
          <Textarea
            value={props.hintText}
            onChange={(event) => props.onHintText(event.target.value)}
            placeholder={t("project.dialogs.hint.placeholder")}
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onHintOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={props.onAddHint} disabled={!props.hintText}>
              {t("common.add")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.renameOpen} onOpenChange={props.onRenameOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.rename.title")}</DialogTitle>
          </DialogHeader>
          <Input value={props.renameText} onChange={(event) => props.onRenameText(event.target.value)} />
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onRenameOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={props.onRename} disabled={!props.renameText}>
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.maxStepsOpen} onOpenChange={props.onMaxStepsOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.maxSteps.title")}</DialogTitle>
            <DialogDescription>{t("project.maxSteps.description")}</DialogDescription>
          </DialogHeader>
          <Input
            type="number"
            min={1}
            value={props.maxStepsText}
            placeholder={t("project.maxSteps.placeholder")}
            onChange={(event) => props.onMaxStepsText(event.target.value)}
          />
          <p className="text-[11px] text-muted-foreground">{t("project.maxSteps.hint")}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onMaxStepsOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={props.onSaveMaxSteps}>{t("common.save")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.reopenOpen} onOpenChange={props.onReopenOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.reopen.title")}</DialogTitle>
          </DialogHeader>
          <Textarea
            value={props.reopenText}
            onChange={(event) => props.onReopenText(event.target.value)}
            placeholder={t("project.dialogs.reopen.placeholder")}
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onReopenOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={props.onReopen} disabled={!props.reopenText}>
              {t("project.reopen")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.deleteOpen} onOpenChange={props.onDeleteOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("project.dialogs.remove.title")}</DialogTitle>
            <DialogDescription>{t("project.dialogs.remove.body")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => props.onDeleteOpenChange(false)}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={props.onDelete}>
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={props.snapshot !== null} onOpenChange={(open) => !open && props.onCloseSnapshot()}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>{t("project.snapshot")}</DialogTitle>
            <DialogDescription>{props.projectId}</DialogDescription>
          </DialogHeader>
          <div className="max-h-[60vh] overflow-auto rounded-lg border bg-muted/40 p-3">
            <pre className="text-xs leading-5">{props.snapshot}</pre>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={props.onDownloadSnapshot}>
              <Download className="size-3.5" />
              {t("common.download")}
            </Button>
            <Button variant="outline" onClick={props.onCopySnapshot}>
              <Copy className="size-3.5" />
              {t("common.copy")}
            </Button>
            <Button onClick={props.onCloseSnapshot}>{t("common.close")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
