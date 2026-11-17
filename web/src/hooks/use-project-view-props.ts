import type { Dispatch, SetStateAction } from "react";
import { useTranslation } from "react-i18next";

import { useToast } from "@/hooks/use-toast";
import { copyText } from "@/lib/api";
import { downloadText, snapshotFilename } from "@/lib/download";
import type { UseFormatResult, LayoutMode } from "@/lib/format";
import type { Intent, ProjectDetail, ProjectStatus, Selection, TimelineEvent } from "@/lib/types";
import type { GraphToolbarProps } from "@/components/project/graph-toolbar";
import type { ProjectDialogsProps } from "@/components/project/project-dialogs";
import type { ProjectHeaderProps } from "@/components/project/project-header";
import type { ProjectSidePanelProps, SidePanelSummary } from "@/components/project/project-side-panel";

export interface ViewContext {
  project: ProjectDetail;
  replay: ProjectHeaderProps["replay"];
  format: UseFormatResult;
  prefsLayoutMode: LayoutMode;
  saveLayoutMode: (mode: LayoutMode) => void;
  selectedFacts: string[];
  selectedNode: Selection | null;
  selectedFactRecord: ProjectDetail["facts"][number] | null;
  selectedFactIntent: Intent | undefined;
  selectedIntentRecord: Intent | null;
  selectedGroup: ProjectSidePanelProps["selectedGroup"];
  producingIntent: Intent | undefined;
  selectedTimelineSummary: SidePanelSummary | null;
  timeline: TimelineEvent[];
  selectedTimelineEntryId: string | null;
  sideTab: "detail" | "hints" | "log";
  logRef: ProjectSidePanelProps["logRef"];
  isRunnable: boolean;
  isActive: boolean;
  canActOnFacts: boolean;
  canWriteHints: boolean;
  actionableIntent: Intent | null;
  releasableIntent: Intent | null;
  openIntent: Intent | null;
  concludeTarget: Intent | null;
  concludeText: string;
  setIntentOpen: Dispatch<SetStateAction<boolean>>;
  setIntentText: Dispatch<SetStateAction<string>>;
  setCompleteOpen: Dispatch<SetStateAction<boolean>>;
  setCompleteText: Dispatch<SetStateAction<string>>;
  setHintOpen: Dispatch<SetStateAction<boolean>>;
  setHintText: Dispatch<SetStateAction<string>>;
  setConcludeTarget: Dispatch<SetStateAction<Intent | null>>;
  setConcludeText: Dispatch<SetStateAction<string>>;
  setSideTab: Dispatch<SetStateAction<"detail" | "hints" | "log">>;
  setRenameOpen: Dispatch<SetStateAction<boolean>>;
  setRenameText: Dispatch<SetStateAction<string>>;
  setMaxStepsOpen: Dispatch<SetStateAction<boolean>>;
  setMaxStepsText: Dispatch<SetStateAction<string>>;
  setReopenOpen: Dispatch<SetStateAction<boolean>>;
  setReopenText: Dispatch<SetStateAction<string>>;
  setDeleteOpen: Dispatch<SetStateAction<boolean>>;
  setSnapshot: Dispatch<SetStateAction<string | null>>;
  snapshot: string | null;
  intentOpen: boolean;
  intentText: string;
  completeOpen: boolean;
  completeText: string;
  hintOpen: boolean;
  hintText: string;
  renameOpen: boolean;
  renameText: string;
  maxStepsOpen: boolean;
  maxStepsText: string;
  reopenOpen: boolean;
  reopenText: string;
  deleteOpen: boolean;
  layoutMode: LayoutMode;
  setLayoutMode: Dispatch<SetStateAction<LayoutMode>>;
  handleEntryOpen: (entry: TimelineEvent) => void;
  centerOnFact: (factId: string) => void;
  centerOnIntent: (intentId: string) => void;
  openTimelineEntry: (entry: TimelineEvent, centers: { centerOnFact: (id: string) => void; centerOnIntent: (id: string) => void }) => void;
  scrollToEntry: (entryId: string | null, flash?: boolean) => void;
  selectIntent: (intentId: string) => void;
  toggleFact: (factId: string) => void;
  setStatus: (next: ProjectStatus) => void;
  startReplay: () => void;
  exitReplay: () => void;
  toggleReplayPlayback: () => void;
  restartReplay: () => void;
  setReplaySpeed: (value: string) => void;
  heartbeatSelected: () => void;
  releaseSelected: () => void;
  createIntent: (claim: boolean) => void;
  completeProject: () => void;
  addHint: () => void;
  renameProject: () => void;
  reopenProject: () => void;
  deleteProject: () => void;
  saveMaxSteps: () => void;
  openSnapshot: () => void;
  concludeIntent: () => void;
  fit: () => void;
}

export function useHeaderProps(ctx: ViewContext): ProjectHeaderProps {
  return {
    project: ctx.project,
    replay: ctx.replay,
    onRename: () => {
      ctx.setRenameText(ctx.project.project.title);
      ctx.setRenameOpen(true);
    },
    onEditMaxSteps: () => {
      ctx.setMaxStepsText(ctx.project.project.max_steps?.toString() ?? "");
      ctx.setMaxStepsOpen(true);
    },
    onStartReplay: ctx.startReplay,
    onToggleReplay: ctx.toggleReplayPlayback,
    onRestartReplay: ctx.restartReplay,
    onSpeedChange: ctx.setReplaySpeed,
    onExitReplay: ctx.exitReplay,
    onSnapshot: ctx.openSnapshot,
    onToggleStatus: () => ctx.setStatus(ctx.project.project.status === "active" ? "stopped" : "active"),
    onReopen: () => ctx.setReopenOpen(true),
    onDelete: () => ctx.setDeleteOpen(true),
  };
}

export function useToolbarProps(ctx: ViewContext): GraphToolbarProps {
  return {
    reason: ctx.project.project.reason,
    heartbeatLabel: ctx.format.time(ctx.project.project.reason?.last_heartbeat_at),
    layoutMode: ctx.layoutMode,
    onLayoutChange: (mode) => {
      ctx.setLayoutMode(mode);
      ctx.saveLayoutMode(mode);
    },
    onFit: ctx.fit,
    isRunnable: ctx.isRunnable,
    isActive: ctx.isActive,
    canActOnFacts: ctx.canActOnFacts,
    canWriteHints: ctx.canWriteHints,
    hasActionableIntent: Boolean(ctx.actionableIntent),
    intentClaimed: Boolean(ctx.openIntent?.worker),
    hasReleasableIntent: Boolean(ctx.releasableIntent),
    onIntent: () => {
      ctx.setIntentText("");
      ctx.setIntentOpen(true);
    },
    onComplete: () => {
      ctx.setCompleteText("");
      ctx.setCompleteOpen(true);
    },
    onHint: () => {
      ctx.setHintText("");
      ctx.setHintOpen(true);
    },
    onHeartbeat: ctx.heartbeatSelected,
    onRelease: ctx.releaseSelected,
    onConclude: () => {
      if (!ctx.actionableIntent) return;
      ctx.setConcludeTarget(ctx.actionableIntent);
      ctx.setConcludeText("");
    },
  };
}

export function usePanelProps(ctx: ViewContext): ProjectSidePanelProps {
  return {
    sideTab: ctx.sideTab,
    onTab: ctx.setSideTab,
    width: 0,
    project: ctx.project,
    selectedNode: ctx.selectedNode,
    selectedFacts: ctx.selectedFacts,
    selectedFactRecord: ctx.selectedFactRecord,
    selectedFactIntent: ctx.selectedFactIntent ?? null,
    selectedIntentRecord: ctx.selectedIntentRecord,
    selectedGroup: ctx.selectedGroup,
    producingIntent: ctx.producingIntent ?? null,
    selectedTimelineSummary: ctx.selectedTimelineSummary,
    timeline: ctx.timeline,
    selectedTimelineEntryId: ctx.selectedTimelineEntryId,
    onEntryOpen: ctx.handleEntryOpen,
    onHintOpen: (hintId) => {
      const entry = ctx.timeline.find((item) => item.type === "hint_added" && item.targetId === hintId);
      if (!entry) return;
      ctx.setSideTab("log");
      ctx.openTimelineEntry(entry, {
        centerOnFact: ctx.centerOnFact,
        centerOnIntent: ctx.centerOnIntent,
      });
      ctx.scrollToEntry(entry.id);
    },
    onSelectIntent: ctx.selectIntent,
    onToggleFact: ctx.toggleFact,
    logRef: ctx.logRef,
    format: ctx.format,
  };
}

export function useDialogsProps(ctx: ViewContext): ProjectDialogsProps {
  const toast = useToast();
  const { t } = useTranslation();

  return {
    factsLabel: ctx.selectedFacts.join(", "),
    intentOpen: ctx.intentOpen,
    onIntentOpenChange: ctx.setIntentOpen,
    intentText: ctx.intentText,
    onIntentText: ctx.setIntentText,
    onDeclareIntent: () => ctx.createIntent(false),
    onDeclareAndClaim: () => ctx.createIntent(true),
    concludeTargetId: ctx.concludeTarget?.id ?? null,
    concludeText: ctx.concludeText,
    onConcludeText: ctx.setConcludeText,
    onCloseConclude: () => ctx.setConcludeTarget(null),
    onConclude: ctx.concludeIntent,
    completeOpen: ctx.completeOpen,
    onCompleteOpenChange: ctx.setCompleteOpen,
    completeText: ctx.completeText,
    onCompleteText: ctx.setCompleteText,
    onComplete: ctx.completeProject,
    hintOpen: ctx.hintOpen,
    onHintOpenChange: ctx.setHintOpen,
    hintText: ctx.hintText,
    onHintText: ctx.setHintText,
    onAddHint: ctx.addHint,
    renameOpen: ctx.renameOpen,
    onRenameOpenChange: ctx.setRenameOpen,
    renameText: ctx.renameText,
    onRenameText: ctx.setRenameText,
    onRename: ctx.renameProject,
    maxStepsOpen: ctx.maxStepsOpen,
    onMaxStepsOpenChange: ctx.setMaxStepsOpen,
    maxStepsText: ctx.maxStepsText,
    onMaxStepsText: ctx.setMaxStepsText,
    onSaveMaxSteps: ctx.saveMaxSteps,
    reopenOpen: ctx.reopenOpen,
    onReopenOpenChange: ctx.setReopenOpen,
    reopenText: ctx.reopenText,
    onReopenText: ctx.setReopenText,
    onReopen: ctx.reopenProject,
    deleteOpen: ctx.deleteOpen,
    onDeleteOpenChange: ctx.setDeleteOpen,
    onDelete: ctx.deleteProject,
    snapshot: ctx.snapshot,
    onCloseSnapshot: () => ctx.setSnapshot(null),
    onDownloadSnapshot: () =>
      ctx.snapshot && downloadText(snapshotFilename(ctx.project.project.id), ctx.snapshot),
    onCopySnapshot: () =>
      ctx.snapshot &&
      copyText(ctx.snapshot)
        .then(() => toast.show(t("common.copied")))
        .catch(() => toast.show(t("common.copyFailed"), "error")),
  };
}
