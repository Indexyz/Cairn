import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import type { NodeSingular } from "cytoscape";

import { readPanelWidth, savePanelWidth } from "@/hooks/use-local-prefs";
import { useProjectActions } from "@/hooks/use-project-actions";
import { useProjectDerivations, useProjectFlags } from "@/hooks/use-project-flags";
import { useProjectSelection } from "@/hooks/use-project-selection";
import {
  useDialogsProps,
  useHeaderProps,
  usePanelProps,
  useToolbarProps,
  type ViewContext,
} from "@/hooks/use-project-view-props";
import { useProjectReplay } from "@/hooks/use-project-replay";
import { useTranslation } from "react-i18next";
import { useCytoscape } from "@/hooks/use-cytoscape";
import { usePrefs } from "@/hooks/use-prefs";
import { useTheme } from "@/hooks/use-theme";
import { useToast } from "@/hooks/use-toast";
import { Spinner } from "@/components/ui/spinner";
import { api } from "@/lib/api";
import { useFormat, type LayoutMode } from "@/lib/format";
import { groupNodeId } from "@/lib/graph";



import { ProjectWorkspace } from "@/components/project/project-workspace";
import { buildTimeline } from "@/lib/timeline";
import type { Intent, ProjectDetail, TimelineEvent } from "@/lib/types";

function useProjectPage() {
  const { projectId = "" } = useParams();
  const toast = useToast();
  const format = useFormat();
  const { actorName, prefs, save: savePrefs } = usePrefs();
  const { resolvedTheme } = useTheme();

  const [project, setProject] = useState<ProjectDetail | null>(null);
  const [sideTab, setSideTab] = useState<"detail" | "hints" | "log">("detail");
  const [layoutMode, setLayoutMode] = useState<LayoutMode>(prefs.layout_mode);
  const [panelWidth, setPanelWidth] = useState(() => readPanelWidth() ?? 360);

  const [intentOpen, setIntentOpen] = useState(false);
  const [intentText, setIntentText] = useState("");
  const [concludeTarget, setConcludeTarget] = useState<Intent | null>(null);
  const [concludeText, setConcludeText] = useState("");
  const [completeOpen, setCompleteOpen] = useState(false);
  const [completeText, setCompleteText] = useState("");
  const [hintOpen, setHintOpen] = useState(false);
  const [hintText, setHintText] = useState("");
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameText, setRenameText] = useState("");
  const [reopenOpen, setReopenOpen] = useState(false);
  const [reopenText, setReopenText] = useState("");
  const [maxStepsOpen, setMaxStepsOpen] = useState(false);
  const [maxStepsText, setMaxStepsText] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [snapshot, setSnapshot] = useState<string | null>(null);

  const graphRef = useRef<HTMLDivElement | null>(null);
  const logRef = useRef<HTMLDivElement | null>(null);

  const load = useCallback(
    async (id: string) => {
      try {
        const next = await api<ProjectDetail>("GET", `/projects/${id}`);
        // Polling re-delivers the same JSON; skipping the state write avoids a full re-render.
        const serialized = JSON.stringify(next);
        if (serialized === payloadRef.current) return;
        payloadRef.current = serialized;
        setProject(next);
      } catch (error) {
        toast.show((error as Error).message, "error");
      }
    },
    [toast],
  );

  const payloadRef = useRef<string | null>(null);
  const panelWidthRef = useRef(panelWidth);
  const openEntryRef = useRef<(entry: TimelineEvent) => void>(() => {});
  const handleEntryOpen = useCallback((entry: TimelineEvent) => openEntryRef.current(entry), []);

  const {
    replay,
    replayVersion,
    startReplay,
    exitReplay,
    toggleReplayPlayback,
    restartReplay,
    setReplaySpeed,
    resetReplay,
  } = useProjectReplay({
    project,
    projectId,
    getTimeline: () => timeline,
    onStart: () => {
      clearSelection();
      setSideTab("log");
      payloadRef.current = null;
    },
    onExit: async () => {
      payloadRef.current = null;
      await load(projectId);
    },
    onFrame: setProject,
  });

  const { isActive, isRunnable, canWriteHints } = useProjectFlags(project?.project.status, replay.active);

  useEffect(() => {
    if (replay.active) return;
    const timer = window.setInterval(() => void load(projectId), 5000);
    return () => window.clearInterval(timer);
  }, [projectId, replay.active, load]);

  const timelineSource = project;
  const timeline = useMemo(() => {
    if (replay.active) {
      return replay.frames
        .slice(0, replay.frameIndex + 1)
        .map((frame) => frame.event)
        .filter((event) => event.type !== "reason_started")
        .map((event, index, list) => ({ ...event, isLast: index === list.length - 1 }));
    }
    return buildTimeline(timelineSource);
  }, [timelineSource, replay]);

  const {
    selectedNode,
    selectedFacts,
    selectedTimelineEntryId,
    setSelectedNode,
    setSelectedFacts,
    selectedFactRecord,
    selectedIntentRecord,
    selectedGroup,
    selectedTimelineSummary,
    clearSelection,
    selectGroup,
    selectIntent,
    selectFact,
    toggleFact,
    scrollToEntry,
    scrollToSelection,
    openTimelineEntry,
  } = useProjectSelection({
    project,
    timelineSource,
    timeline,
    sideTab,
    logRef,
    duration: format.duration,
  });

  useEffect(() => {
    clearSelection();
    resetReplay();
    payloadRef.current = null;
    void load(projectId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, load]);

  useEffect(() => {
    panelWidthRef.current = panelWidth;
  }, [panelWidth]);

  const { fit, centerOnFact, centerOnIntent } = useCytoscape({
    containerRef: graphRef,
    resetKey: replayVersion,
    project,
    layoutMode,
    theme: resolvedTheme,
    selectedNode,
    selectedFacts,
    onNodeTap: (node: NodeSingular, shiftKey: boolean) => {
      const nodeType = node.data("nodeType") as string;
      const id = node.data("id") as string;
      if (nodeType === "group") {
        selectGroup(id);
      } else if (["in_progress", "unclaimed", "bootstrap_pending", "bootstrap_running"].includes(nodeType)) {
        selectIntent(node.data("intentId") as string);
      } else if (shiftKey) {
        toggleFact(id);
      } else {
        setSelectedFacts([id]);
        selectFact(id);
      }
      if (sideTab !== "log") setSideTab("detail");
      else scrollToSelection();
    },
    onEdgeTap: (intentId: string | undefined, groupId: string | undefined) => {
      if (intentId) selectIntent(intentId);
      else if (groupId) selectGroup(groupNodeId(groupId));
      if (sideTab === "log") scrollToSelection();
      else setSideTab("detail");
    },
    onBackgroundTap: () => clearSelection(),
  });

  // Keep the latest handler for memoised rows without writing the ref during render.
  useEffect(() => {
    openEntryRef.current = (entry) => openTimelineEntry(entry, { centerOnFact, centerOnIntent });
  });

  const { selectedFactIntent, producingIntent, openIntent, actionableIntent, releasableIntent } =
    useProjectDerivations({
      project,
      selectedNode,
      selectedFactRecord,
      selectedIntentRecord,
      isRunnable,
      actorName,
    });

  const {
    setStatus,
    saveMaxSteps,
    createIntent,
    heartbeatSelected,
    releaseSelected,
    concludeIntent,
    completeProject,
    addHint,
    renameProject,
    reopenProject,
    deleteProject,
    openSnapshot,
  } = useProjectActions({
    projectId,
    actorName,
    load,
    selectedFacts,
    actionableIntent,
    releasableIntent,
    concludeTarget,
    concludeText,
    intentText,
    completeText,
    hintText,
    renameText,
    reopenText,
    maxStepsText,
    setIntentOpen,
    setIntentText,
    setConcludeTarget,
    setConcludeText,
    setSelectedNode,
    setCompleteOpen,
    setCompleteText,
    setHintOpen,
    setHintText,
    setSideTab,
    setRenameOpen,
    setReopenOpen,
    setReopenText,
    setSnapshot,
    setMaxStepsOpen,
  });

  const canActOnFacts = isRunnable && selectedFacts.length > 0 && !selectedFacts.includes("goal");

  useEffect(() => {
    if (replay.active) scrollToEntry(timeline[timeline.length - 1]?.id ?? null, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [replay.frameIndex, replay.active]);

  function startPanelResize(event: React.PointerEvent<HTMLDivElement>) {
    event.currentTarget.setPointerCapture(event.pointerId);
    const move = (moveEvent: PointerEvent) => {
      const rect = graphRef.current?.parentElement?.getBoundingClientRect();
      if (!rect) return;
      setPanelWidth(Math.min(Math.max(rect.right - moveEvent.clientX, 280), rect.width - 280));
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      savePanelWidth(panelWidthRef.current);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  }



  const loaded = project as ProjectDetail;
  const ctx: ViewContext = {
    project: loaded,
    replay,
    format,
    prefsLayoutMode: prefs.layout_mode,
    saveLayoutMode: (mode) => savePrefs({ ...prefs, layout_mode: mode }),
    selectedFacts,
    selectedNode,
    selectedFactRecord,
    selectedFactIntent,
    selectedIntentRecord,
    selectedGroup,
    producingIntent,
    selectedTimelineSummary,
    timeline,
    selectedTimelineEntryId,
    sideTab,
    logRef,
    isRunnable,
    isActive,
    canActOnFacts,
    canWriteHints,
    actionableIntent,
    releasableIntent,
    openIntent,
    concludeTarget,
    concludeText,
    setIntentOpen,
    setIntentText,
    setCompleteOpen,
    setCompleteText,
    setHintOpen,
    setHintText,
    setConcludeTarget,
    setConcludeText,
    setSideTab,
    setRenameOpen,
    setRenameText,
    setMaxStepsOpen,
    setMaxStepsText,
    setReopenOpen,
    setReopenText,
    setDeleteOpen,
    setSnapshot,
    snapshot,
    intentOpen,
    intentText,
    completeOpen,
    completeText,
    hintOpen,
    hintText,
    renameOpen,
    renameText,
    maxStepsOpen,
    maxStepsText,
    reopenOpen,
    reopenText,
    deleteOpen,
    layoutMode,
    setLayoutMode,
    handleEntryOpen,
    centerOnFact,
    centerOnIntent,
    openTimelineEntry,
    scrollToEntry,
    selectIntent,
    toggleFact,
    setStatus,
    startReplay,
    exitReplay,
    toggleReplayPlayback,
    restartReplay,
    setReplaySpeed,
    heartbeatSelected,
    releaseSelected,
    createIntent,
    completeProject,
    addHint,
    renameProject,
    reopenProject,
    deleteProject,
    saveMaxSteps,
    openSnapshot,
    concludeIntent,
    fit,
  };

  if (!project) return null;

  return {
    project,
    ctx,
    graphRef,
    replayVersion,
    panelWidth,
    onPanelResizeStart: startPanelResize,
  };
}

interface ProjectViewProps {
  view: {
    project: ProjectDetail;
    ctx: ViewContext;
    graphRef: React.RefObject<HTMLDivElement | null>;
    replayVersion: number;
    panelWidth: number;
    onPanelResizeStart: (event: React.PointerEvent<HTMLDivElement>) => void;
  };
}

function ProjectView({ view }: ProjectViewProps) {
  const header = useHeaderProps(view.ctx);
  const toolbar = useToolbarProps(view.ctx);
  const panel = usePanelProps(view.ctx);
  const dialogs = useDialogsProps(view.ctx);

  return (
    <ProjectWorkspace
      project={view.project}
      graphRef={view.graphRef}
      replayVersion={view.replayVersion}
      panelWidth={view.panelWidth}
      onPanelResizeStart={view.onPanelResizeStart}
      header={header}
      toolbar={toolbar}
      panel={panel}
      dialogs={dialogs}
    />
  );
}


export function ProjectPage() {
  const view = useProjectPage();
  const { t } = useTranslation();

  if (!view) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground">
        <Spinner className="size-4" />
        <span>{t("common.loading")}</span>
      </div>
    );
  }

  return <ProjectView view={view} />;
}
