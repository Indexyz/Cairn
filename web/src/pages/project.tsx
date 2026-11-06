import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { NodeSingular } from "cytoscape";
import {
  ChevronLeft,
  CircleCheck,
  Copy,
  FileCode2,
  Lightbulb,
  Maximize2,
  Pause,
  Pencil,
  Play,
  RotateCcw,
  Sparkles,
  Square,
  Trash2,
} from "lucide-react";

import { readPanelWidth, savePanelWidth } from "@/hooks/use-local-prefs";
import { useCytoscape } from "@/hooks/use-cytoscape";
import { usePrefs } from "@/hooks/use-prefs";
import { useToast } from "@/hooks/use-toast";
import { api, copyText, fetchText } from "@/lib/api";
import { formatDurationMs, formatTime, formatTimelineDate, LAYOUT_MODES, type LayoutMode } from "@/lib/format";
import { buildTimeline, isBootstrapIntent, timelineEventBadge, timelineEventBadgeClass, timelineEventDotClass, timelineEventIsInteractive, timelineEventTriggersGraphFocus } from "@/lib/timeline";
import type { Intent, ProjectDetail, ProjectStatus, Selection, TimelineEvent } from "@/lib/types";
import { cn } from "@/lib/utils";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

const STATUS_LABEL: Record<ProjectStatus, string> = {
  active: "运行中",
  stopped: "已暂停",
  completed: "已完成",
};

const STATUS_BADGE: Record<ProjectStatus, "running" | "paused" | "done"> = {
  active: "running",
  stopped: "paused",
  completed: "done",
};

const REPLAY_SPEEDS = [
  { value: "700", label: "Fast" },
  { value: "1100", label: "Normal" },
  { value: "1600", label: "Slow" },
];

interface ReplayState {
  active: boolean;
  playing: boolean;
  stepMs: string;
  frameIndex: number;
  frames: { event: TimelineEvent; project: ProjectDetail }[];
  selectedProject: ProjectDetail | null;
}

const EMPTY_REPLAY: ReplayState = {
  active: false,
  playing: false,
  stepMs: "1100",
  frameIndex: -1,
  frames: [],
  selectedProject: null,
};

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function replayEventWeight(event: TimelineEvent): number {
  switch (event.type) {
    case "reason_started":
      return 1.6;
    case "intent_declared":
      return 1.05;
    case "intent_running":
      return 0.75;
    case "intent_concluded":
    case "project_completed":
      return 1.6;
    case "project_created":
    case "hint_added":
      return 0.8;
    default:
      return 1;
  }
}

export function ProjectPage() {
  const { projectId = "" } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const { actorName, prefs, save: savePrefs } = usePrefs();

  const [project, setProject] = useState<ProjectDetail | null>(null);
  const [selectedNode, setSelectedNode] = useState<Selection | null>(null);
  const [selectedFacts, setSelectedFacts] = useState<string[]>([]);
  const [selectedTimelineEntryId, setSelectedTimelineEntryId] = useState<string | null>(null);
  const [sideTab, setSideTab] = useState<"detail" | "hints" | "log">("detail");
  const [layoutMode, setLayoutMode] = useState<LayoutMode>(prefs.layout_mode);
  const [panelWidth, setPanelWidth] = useState(() => readPanelWidth() ?? 360);
  const [replay, setReplay] = useState<ReplayState>(EMPTY_REPLAY);
  const [replayVersion, setReplayVersion] = useState(0);

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
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [snapshot, setSnapshot] = useState<string | null>(null);

  const graphRef = useRef<HTMLDivElement | null>(null);
  const logRef = useRef<HTMLDivElement | null>(null);
  const replayTimer = useRef<number | null>(null);

  const isActive = project?.project.status === "active" && !replay.active;
  const canWriteHints = !replay.active && Boolean(project);

  const load = useCallback(
    async (id: string) => {
      try {
        setProject(await api<ProjectDetail>("GET", `/projects/${id}`));
      } catch (error) {
        toast.show((error as Error).message, "error");
      }
    },
    [toast],
  );

  useEffect(() => {
    setSelectedNode(null);
    setSelectedFacts([]);
    setSelectedTimelineEntryId(null);
    setReplay(EMPTY_REPLAY);
    void load(projectId);
  }, [projectId, load]);

  useEffect(() => {
    if (replay.active) return;
    const timer = window.setInterval(() => void load(projectId), 5000);
    return () => window.clearInterval(timer);
  }, [projectId, replay.active, load]);

  const timeline = useMemo(() => {
    if (replay.active) {
      return replay.frames
        .slice(0, replay.frameIndex + 1)
        .map((frame) => frame.event)
        .filter((event) => event.type !== "reason_started")
        .map((event, index, list) => ({ ...event, isLast: index === list.length - 1 }));
    }
    return buildTimeline(project);
  }, [project, replay]);

  const { fit, centerOnFact, centerOnIntent } = useCytoscape({
    containerRef: graphRef,
    resetKey: replayVersion,
    project,
    layoutMode,
    selectedNode,
    selectedFacts,
    onNodeTap: (node: NodeSingular, shiftKey: boolean) => {
      const nodeType = node.data("nodeType") as string;
      const id = node.data("id") as string;
      if (["in_progress", "unclaimed", "bootstrap_pending", "bootstrap_running"].includes(nodeType)) {
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
    onEdgeTap: (intentId: string) => {
      selectIntent(intentId);
      if (sideTab === "log") scrollToSelection();
      else setSideTab("detail");
    },
    onBackgroundTap: () => clearSelection(),
  });

  const selectedFactRecord = useMemo(() => {
    if (!project || selectedNode?.type !== "fact") return null;
    return project.facts.find((fact) => fact.id === selectedNode.id) ?? null;
  }, [project, selectedNode]);

  const selectedIntentRecord = useMemo(() => {
    if (!project || selectedNode?.type !== "intent") return null;
    return project.intents.find((intent) => intent.id === selectedNode.id) ?? null;
  }, [project, selectedNode]);

  const openIntent = selectedIntentRecord && !selectedIntentRecord.to && isActive ? selectedIntentRecord : null;
  const actionableIntent =
    openIntent && (!openIntent.worker || openIntent.worker === actorName) ? openIntent : null;
  const releasableIntent =
    openIntent && openIntent.worker === actorName ? openIntent : null;

  const canActOnFacts = isActive && selectedFacts.length > 0 && !selectedFacts.includes("goal");

  function clearSelection() {
    setSelectedNode(null);
    setSelectedFacts([]);
    setSelectedTimelineEntryId(null);
  }

  function selectIntent(intentId: string) {
    setSelectedFacts([]);
    setSelectedTimelineEntryId(null);
    setSelectedNode({ type: "intent", id: intentId });
  }

  function selectFact(factId: string) {
    setSelectedFacts((current) => (current.includes(factId) ? current : [factId]));
    setSelectedTimelineEntryId(null);
    setSelectedNode({ type: "fact", id: factId });
  }

  function toggleFact(factId: string) {
    setSelectedTimelineEntryId(null);
    setSelectedFacts((current) => {
      const next = current.includes(factId)
        ? current.filter((id) => id !== factId)
        : [...current, factId];
      setSelectedNode(next.length > 0 ? { type: "fact", id: next[next.length - 1] } : null);
      return next;
    });
  }

  function scrollToEntry(entryId: string | null, flash = true) {
    if (!entryId) return;
    window.requestAnimationFrame(() => {
      const element = document.getElementById(`timeline-entry-${entryId}`);
      const panel = logRef.current;
      if (!element || !panel) return;
      const top = Math.max(0, element.offsetTop - panel.clientHeight / 2 + element.clientHeight / 2);
      panel.scrollTo({ top, behavior: "smooth" });
      if (!flash) return;
      window.setTimeout(() => {
        element.classList.remove("timeline-flash");
        void element.offsetWidth;
        element.classList.add("timeline-flash");
        window.setTimeout(() => element.classList.remove("timeline-flash"), 1000);
      }, 160);
    });
  }

  function scrollToSelection() {
    if (sideTab !== "log") return;
    const target =
      selectedNode?.type === "intent"
        ? timeline.find((entry) => entry.type === "intent_declared" && entry.intentId === selectedNode.id)
        : selectedNode?.type === "fact"
          ? timeline.find((entry) => entry.targetType === "fact" && entry.targetId === selectedNode.id)
          : null;
    scrollToEntry(target?.id ?? null);
  }

  function openTimelineEntry(entry: TimelineEvent) {
    if (!timelineEventIsInteractive(entry)) return;
    if (!timelineEventTriggersGraphFocus(entry)) {
      setSelectedTimelineEntryId(entry.id);
      setSelectedNode(null);
      setSelectedFacts([]);
      return;
    }
    if (entry.targetType === "intent") {
      selectIntent(entry.targetId);
      setSelectedTimelineEntryId(entry.id);
      centerOnIntent(entry.targetId);
      return;
    }
    setSelectedFacts([entry.targetId]);
    selectFact(entry.targetId);
    setSelectedTimelineEntryId(entry.id);
    centerOnFact(entry.targetId);
  }

  async function setStatus(next: ProjectStatus) {
    try {
      await api("PUT", `/projects/${projectId}/status`, { status: next });
      await load(projectId);
      toast.show(next === "stopped" ? "Project stopped" : "Project resumed");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function createIntent(claim: boolean) {
    try {
      await api("POST", `/projects/${projectId}/intents`, {
        from: selectedFacts,
        description: intentText,
        creator: actorName,
        worker: claim ? actorName : null,
      });
      setIntentOpen(false);
      setIntentText("");
      await load(projectId);
      toast.show(claim ? "Intent declared and claimed" : "Intent declared");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function heartbeatSelected() {
    if (!actionableIntent) return;
    try {
      const claiming = !actionableIntent.worker;
      await api("POST", `/projects/${projectId}/intents/${actionableIntent.id}/heartbeat`, {
        worker: actorName,
      });
      await load(projectId);
      toast.show(claiming ? "Intent claimed" : "Heartbeat sent");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function releaseSelected() {
    if (!releasableIntent) return;
    try {
      await api("POST", `/projects/${projectId}/intents/${releasableIntent.id}/release`, {
        worker: actorName,
      });
      await load(projectId);
      toast.show("Intent released");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function concludeIntent() {
    if (!concludeTarget) return;
    try {
      await api("POST", `/projects/${projectId}/intents/${concludeTarget.id}/conclude`, {
        worker: actorName,
        description: concludeText,
      });
      setConcludeTarget(null);
      setConcludeText("");
      setSelectedNode(null);
      await load(projectId);
      toast.show("Intent concluded");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function completeProject() {
    try {
      await api("POST", `/projects/${projectId}/complete`, {
        from: selectedFacts,
        description: completeText,
        worker: actorName,
      });
      setCompleteOpen(false);
      setCompleteText("");
      await load(projectId);
      toast.show("Project completed");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function addHint() {
    try {
      await api("POST", `/projects/${projectId}/hints`, { content: hintText, creator: actorName });
      setHintOpen(false);
      setHintText("");
      await load(projectId);
      setSideTab("hints");
      toast.show("Hint added");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function renameProject() {
    try {
      await api("PUT", `/projects/${projectId}/title`, { title: renameText });
      setRenameOpen(false);
      await load(projectId);
      toast.show("Project renamed");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function reopenProject() {
    try {
      await api("POST", `/projects/${projectId}/reopen`, {
        description: reopenText,
        creator: actorName,
      });
      setReopenOpen(false);
      setReopenText("");
      await load(projectId);
      toast.show("Project reopened");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function deleteProject() {
    try {
      await api("DELETE", `/projects/${projectId}`);
      toast.show(`Deleted ${projectId}`);
      navigate("/");
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  async function openSnapshot() {
    try {
      setSnapshot(await fetchText(`/projects/${projectId}/export?format=yaml`));
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  function stopReplayTimer() {
    if (replayTimer.current !== null) {
      window.clearTimeout(replayTimer.current);
      replayTimer.current = null;
    }
  }

  const scheduleReplayTick = useCallback((state: ReplayState) => {
    if (!state.active || !state.playing) return;
    if (state.frameIndex >= state.frames.length - 1) {
      setReplay((current) => ({ ...current, playing: false }));
      return;
    }
    stopReplayTimer();
    const frame = state.frames[state.frameIndex];
    const delay = Math.round((Number(state.stepMs) || 1100) * replayEventWeight(frame.event));
    replayTimer.current = window.setTimeout(() => {
      setReplay((current) => {
        if (!current.active) return current;
        const nextIndex = current.frameIndex + 1;
        if (nextIndex >= current.frames.length) return { ...current, playing: false };
        const next: ReplayState = { ...current, frameIndex: nextIndex };
        scheduleReplayTick(next);
        return next;
      });
    }, delay);
  }, []);

  function buildReplayFrames(source: ProjectDetail): ReplayState["frames"] {
    const origin = source.facts.find((fact) => fact.id === "origin");
    const goal = source.facts.find((fact) => fact.id === "goal");
    const working: ProjectDetail = {
      project: { ...clone(source.project), status: "active", reason: null },
      facts: [origin, goal].filter((fact): fact is NonNullable<typeof fact> => Boolean(fact)).map(clone),
      intents: [],
      hints: [],
    };

    const sourceIntents = new Map(source.intents.map((intent) => [intent.id, intent]));
    const events: TimelineEvent[] = [];

    for (const event of timeline) {
      if (event.type !== "intent_declared" || !event.intentId) {
        events.push(clone(event));
        continue;
      }
      const intent = sourceIntents.get(event.intentId);
      events.push({
        ...clone(event),
        id: `reason-started-${event.intentId}`,
        type: "reason_started",
        timestamp: intent?.created_at ?? event.timestamp,
        actor: intent?.creator ?? event.actor,
        targetType: "reason",
        order: `${event.order}.reason`,
      });
      events.push(clone(event));
      if (!intent?.worker) continue;
      events.push({
        ...clone(event),
        id: `intent-running-${intent.id}`,
        type: "intent_running",
        timestamp: intent.last_heartbeat_at ?? intent.created_at,
        actor: intent.worker,
        order: `${event.order}.run`,
      });
    }

    const frames: ReplayState["frames"] = [];
    for (const event of events) {
      applyReplayEvent(working, source, event);
      frames.push({ event: clone(event), project: clone(working) });
    }
    if (frames.length > 0) {
      const last = frames[frames.length - 1].project;
      last.project.status = source.project.status;
      last.project.reason = clone(source.project.reason);
    }
    return frames;
  }

  function applyReplayEvent(target: ProjectDetail, source: ProjectDetail, event: TimelineEvent) {
    const intent = event.intentId ? source.intents.find((item) => item.id === event.intentId) : null;
    if (event.type === "project_created") {
      target.project.title = source.project.title;
      target.project.status = "active";
      return;
    }
    if (event.type === "hint_added") {
      const hint = source.hints.find((item) => item.id === event.targetId);
      if (hint && !target.hints.some((item) => item.id === hint.id)) target.hints.push(clone(hint));
      return;
    }
    if (event.type === "reason_started") {
      target.project.reason = {
        worker: event.actor,
        trigger: "new_facts",
        started_at: event.timestamp,
        last_heartbeat_at: event.timestamp,
      };
      return;
    }
    if (event.type === "intent_declared") {
      if (!intent) return;
      target.project.reason = null;
      if (!target.intents.some((item) => item.id === intent.id)) {
        target.intents.push({
          id: intent.id,
          from: [...intent.from],
          to: null,
          description: intent.description,
          creator: intent.creator,
          worker: null,
          last_heartbeat_at: null,
          created_at: intent.created_at,
          concluded_at: null,
        });
      }
      return;
    }
    if (event.type === "intent_running") {
      if (!intent) return;
      const replayIntent = target.intents.find((item) => item.id === intent.id);
      if (!replayIntent) return;
      replayIntent.worker = intent.worker ?? intent.creator;
      replayIntent.last_heartbeat_at = intent.last_heartbeat_at ?? intent.created_at;
      return;
    }
    if (event.type !== "intent_concluded" && event.type !== "project_completed") return;
    if (!intent) return;
    const replayIntent = target.intents.find((item) => item.id === intent.id);
    if (replayIntent) {
      replayIntent.to = intent.to;
      replayIntent.worker = intent.worker ?? intent.creator;
      replayIntent.last_heartbeat_at = intent.last_heartbeat_at;
      replayIntent.concluded_at = intent.concluded_at;
    }
    if (event.type === "project_completed") {
      target.project.status = "completed";
      return;
    }
    const produced = source.facts.find((fact) => fact.id === intent.to);
    if (produced && !target.facts.some((fact) => fact.id === produced.id)) {
      target.facts.push(clone(produced));
    }
  }

  async function startReplay() {
    if (!project || replay.active) return;
    try {
      const source = await api<ProjectDetail>("GET", `/projects/${projectId}`);
      const frames = buildReplayFrames(source);
      if (frames.length === 0) {
        toast.show("No timeline to replay", "error");
        return;
      }
      stopReplayTimer();
      setSelectedNode(null);
      setSelectedFacts([]);
      setSelectedTimelineEntryId(null);
      setSideTab("log");
      const next: ReplayState = {
        active: true,
        playing: true,
        stepMs: replay.stepMs,
        frameIndex: 0,
        frames,
        selectedProject: source,
      };
      setReplay(next);
      setProject(clone(frames[0].project));
      setReplayVersion((value) => value + 1);
      scheduleReplayTick(next);
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  // keep the displayed graph in sync with the replay frame
  useEffect(() => {
    if (!replay.active) return;
    const frame = replay.frames[replay.frameIndex];
    if (frame) setProject(clone(frame.project));
  }, [replay.active, replay.frameIndex, replay.frames]);

  useEffect(() => {
    if (replay.active && replay.playing) scheduleReplayTick(replay);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [replay.stepMs]);

  useEffect(() => () => stopReplayTimer(), []);

  async function exitReplay() {
    stopReplayTimer();
    setReplay(EMPTY_REPLAY);
    await load(projectId);
    setReplayVersion((value) => value + 1);
  }

  function toggleReplayPlayback() {
    if (replay.playing) {
      stopReplayTimer();
      setReplay((current) => ({ ...current, playing: false }));
      return;
    }
    if (replay.frameIndex >= replay.frames.length - 1) {
      restartReplay();
      return;
    }
    const next = { ...replay, playing: true };
    setReplay(next);
    scheduleReplayTick(next);
  }

  function restartReplay() {
    if (!replay.active || replay.frames.length === 0) return;
    stopReplayTimer();
    setSelectedNode(null);
    setSelectedFacts([]);
    setSelectedTimelineEntryId(null);
    const next = { ...replay, playing: true, frameIndex: 0 };
    setReplay(next);
    setReplayVersion((value) => value + 1);
    scheduleReplayTick(next);
  }

  useEffect(() => {
    if (replay.active) scrollToEntry(timeline[timeline.length - 1]?.id ?? null, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [replay.frameIndex, replay.active]);

  const selectedTimelineSummary = useMemo(() => {
    const targetId =
      selectedTimelineEntryId ??
      (selectedNode?.type === "intent"
        ? (timeline.find((entry) => entry.type === "intent_declared" && entry.intentId === selectedNode.id)?.id ?? null)
        : selectedNode?.type === "fact"
          ? (timeline.find((entry) => entry.targetType === "fact" && entry.targetId === selectedNode.id)?.id ?? null)
          : null);
    if (!targetId) return null;
    const index = timeline.findIndex((entry) => entry.id === targetId);
    if (index < 0) return null;
    const total = timeline.length;
    const sequencePercent = total <= 1 ? 100 : Math.round((index / (total - 1)) * 100);
    const totalDuration = Math.max(
      0,
      Date.parse(timeline[total - 1].timestamp) - Date.parse(timeline[0].timestamp),
    );
    const elapsed = Math.max(0, Date.parse(timeline[index].timestamp) - Date.parse(timeline[0].timestamp));
    const timePercent = totalDuration === 0 ? 100 : Math.round((elapsed / totalDuration) * 100);
    return {
      sequencePercent,
      sequenceLabel: `${index + 1} / ${total} · ${sequencePercent}%`,
      timePercent,
      timeLabel: `${formatDurationMs(elapsed)} / ${formatDurationMs(totalDuration)} · ${timePercent}%`,
    };
  }, [timeline, selectedNode, selectedTimelineEntryId]);

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
      setPanelWidth((width) => {
        savePanelWidth(width);
        return width;
      });
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  }

  if (!project) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        Loading project...
      </div>
    );
  }

  const producingIntent = project.intents.find((intent) => intent.to === (selectedNode?.id ?? ""));
  const selectedFactIntent = selectedFactRecord
    ? project.intents.find((intent) => intent.to === selectedFactRecord.id)
    : null;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex h-14 shrink-0 items-center gap-2 border-b bg-background px-3">
        <Button variant="ghost" size="icon-sm" asChild>
          <Link to="/">
            <ChevronLeft className="size-4" />
          </Link>
        </Button>
        <span className="font-mono text-[11px] text-muted-foreground">{project.project.id}</span>
        <h1 className="max-w-80 truncate text-sm font-semibold">{project.project.title}</h1>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => {
            setRenameText(project.project.title);
            setRenameOpen(true);
          }}
          disabled={replay.active}
        >
          <Pencil className="size-3.5" />
        </Button>
        <Badge variant={STATUS_BADGE[project.project.status]}>
          {STATUS_LABEL[project.project.status]}
        </Badge>
        <span className="ml-2 text-xs text-muted-foreground">
          {project.facts.length} facts · {project.intents.length} intents
        </span>
        <div className="flex-1" />

        {!replay.active && (
          <Button variant="outline" size="sm" onClick={startReplay}>
            <Play className="size-3.5" />
            Replay
          </Button>
        )}
        {replay.active && (
          <div className="flex items-center gap-1 rounded-lg border border-violet-200 bg-violet-50 px-2 py-1 text-xs text-violet-700">
            <span className="font-medium">
              Replay {Math.min(replay.frameIndex + 1, replay.frames.length)} / {replay.frames.length}
            </span>
            <Button variant="ghost" size="icon-sm" onClick={toggleReplayPlayback}>
              {replay.playing ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
            </Button>
            <Button variant="ghost" size="icon-sm" onClick={restartReplay}>
              <RotateCcw className="size-3.5" />
            </Button>
            <Select
              value={replay.stepMs}
              onValueChange={(value) => setReplay((current) => ({ ...current, stepMs: value }))}
            >
              <SelectTrigger className="h-6 w-24 border-violet-200 bg-white text-[11px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {REPLAY_SPEEDS.map((speed) => (
                  <SelectItem key={speed.value} value={speed.value}>
                    {speed.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="ghost" size="sm" onClick={exitReplay}>
              Exit
            </Button>
          </div>
        )}

        <Button variant="outline" size="sm" onClick={openSnapshot}>
          <FileCode2 className="size-3.5" />
          Snapshot
        </Button>
        {!replay.active && project.project.status !== "completed" && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => void setStatus(project.project.status === "active" ? "stopped" : "active")}
          >
            {project.project.status === "active" ? <Square className="size-3.5" /> : <Play className="size-3.5" />}
            {project.project.status === "active" ? "Stop" : "Resume"}
          </Button>
        )}
        {!replay.active && project.project.status === "completed" && (
          <Button variant="outline" size="sm" onClick={() => setReopenOpen(true)}>
            <RotateCcw className="size-3.5" />
            Reopen
          </Button>
        )}
        {!replay.active && (
          <Button variant="outline" size="sm" className="text-destructive" onClick={() => setDeleteOpen(true)}>
            <Trash2 className="size-3.5" />
            Delete
          </Button>
        )}
      </header>

      <div className="flex min-h-0 flex-1">
        <div className="relative min-w-0 flex-1">
          <div className="absolute top-3 left-3 z-10 flex items-center gap-1.5">
            <Button variant="outline" size="icon-sm" className="bg-background/90" onClick={fit}>
              <Maximize2 className="size-4" />
            </Button>
            <Select
              value={layoutMode}
              onValueChange={(value) => {
                const mode = value as LayoutMode;
                setLayoutMode(mode);
                savePrefs({ ...prefs, layout_mode: mode });
              }}
            >
              <SelectTrigger className="h-8 w-28 bg-background/90 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LAYOUT_MODES.map((mode) => (
                  <SelectItem key={mode} value={mode}>
                    {mode}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {project.project.reason && (
            <div className="absolute top-3 right-3 z-10 w-72 rounded-xl border border-sky-200 bg-background/95 p-3 shadow-lg">
              <div className="flex items-start gap-2">
                <span className="pulse-soft mt-1 size-2 rounded-full bg-sky-500" />
                <div className="min-w-0">
                  <p className="text-[10px] font-semibold tracking-widest text-sky-600 uppercase">
                    Reason running
                  </p>
                  <p className="truncate text-sm font-semibold">{project.project.reason.worker}</p>
                  <p className="text-[11px] text-muted-foreground">
                    Heartbeat {formatTime(project.project.reason.last_heartbeat_at)}
                  </p>
                </div>
              </div>
            </div>
          )}

          <div className="absolute top-14 left-3 z-10 flex flex-col gap-2">
            <div className="flex gap-1.5">
              {isActive && (
                <>
                  <Button
                    size="sm"
                    variant="outline"
                    className="bg-background/90"
                    disabled={!canActOnFacts}
                    onClick={() => {
                      setIntentText("");
                      setIntentOpen(true);
                    }}
                  >
                    <Sparkles className="size-3.5" />
                    Intent
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="bg-background/90"
                    disabled={!canActOnFacts}
                    onClick={() => {
                      setCompleteText("");
                      setCompleteOpen(true);
                    }}
                  >
                    <CircleCheck className="size-3.5" />
                    Complete
                  </Button>
                </>
              )}
              {canWriteHints && (
                <Button
                  size="sm"
                  variant="outline"
                  className="bg-background/90"
                  onClick={() => {
                    setHintText("");
                    setHintOpen(true);
                  }}
                >
                  <Lightbulb className="size-3.5" />
                  Hint
                </Button>
              )}
            </div>
            {isActive && (
              <div className="flex gap-1.5">
                <Button
                  size="sm"
                  variant="outline"
                  className="bg-background/90"
                  disabled={!actionableIntent}
                  onClick={heartbeatSelected}
                >
                  {openIntent?.worker ? "Heartbeat" : "Claim"}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="bg-background/90"
                  disabled={!releasableIntent}
                  onClick={releaseSelected}
                >
                  Release
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="bg-background/90"
                  disabled={!actionableIntent}
                  onClick={() => {
                    if (!actionableIntent) return;
                    setConcludeTarget(actionableIntent);
                    setConcludeText("");
                  }}
                >
                  Conclude
                </Button>
              </div>
            )}
          </div>

          <div key={replayVersion} ref={graphRef} className="h-full w-full" />        </div>

        <div
          onPointerDown={startPanelResize}
          className="w-1.5 shrink-0 cursor-col-resize bg-border/60 hover:bg-border"
        />

        <aside
          className="flex shrink-0 flex-col overflow-hidden border-l bg-background"
          style={{ width: panelWidth }}
        >
          <div className="flex shrink-0 border-b">
            {(["detail", "hints", "log"] as const).map((tab) => (
              <button
                key={tab}
                type="button"
                onClick={() => setSideTab(tab)}
                className={cn(
                  "flex-1 px-3 py-2.5 text-xs font-medium capitalize transition-colors",
                  sideTab === tab
                    ? "border-b-2 border-primary text-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {tab}
                {tab === "hints" && <span className="ml-1 opacity-60">{project.hints.length}</span>}
              </button>
            ))}
          </div>

          <div className="flex-1 overflow-y-auto p-3">
            {sideTab === "detail" && (
              <div className="space-y-3">
                {!selectedNode && (
                  <div className="rounded-xl border border-dashed px-4 py-10 text-center">
                    <p className="text-sm text-muted-foreground">Click a node or edge</p>
                    <p className="text-xs text-muted-foreground/70">Shift+click for multi-select</p>
                  </div>
                )}
                {selectedNode?.type === "fact" && selectedFacts.length > 1 && (
                  <div className="space-y-2">
                    {selectedFacts.map((factId) => {
                      const fact = project.facts.find((item) => item.id === factId);
                      if (!fact) return null;
                      return (
                        <article key={factId} className="rounded-xl border p-3">
                          <div className="mb-2 flex items-center justify-between">
                            <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">
                              {fact.id}
                            </span>
                            <Button
                              variant="link"
                              size="sm"
                              onClick={() => toggleFact(fact.id)}
                            >
                              Remove
                            </Button>
                          </div>
                          <p className="text-sm break-words whitespace-pre-wrap">{fact.description}</p>
                        </article>
                      );
                    })}
                  </div>
                )}
                {selectedNode?.type === "fact" && selectedFacts.length <= 1 && selectedFactRecord && (
                  <article className="rounded-xl border p-3">
                    <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">
                      {selectedFactRecord.id}
                    </span>
                    <p className="mt-2 text-sm break-words whitespace-pre-wrap">
                      {selectedFactRecord.description}
                    </p>
                    {selectedFactIntent && (
                      <dl className="mt-3 grid grid-cols-2 gap-1 border-t pt-3 text-xs">
                        <dt className="text-muted-foreground">Produced by</dt>
                        <dd className="text-right font-mono">{selectedFactIntent.id}</dd>
                        <dt className="text-muted-foreground">From</dt>
                        <dd className="text-right font-mono">{selectedFactIntent.from.join(", ")}</dd>
                        <dt className="text-muted-foreground">Worker</dt>
                        <dd className="text-right">{selectedFactIntent.worker ?? "—"}</dd>
                      </dl>
                    )}
                  </article>
                )}
                {selectedNode?.type === "intent" && selectedIntentRecord && (
                  <article className="rounded-xl border p-3">
                    <div className="mb-2 flex items-center gap-2">
                      <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">
                        {selectedIntentRecord.id}
                      </span>
                      <Badge variant="secondary">
                        {selectedIntentRecord.to
                          ? "Concluded"
                          : isBootstrapIntent(selectedIntentRecord)
                            ? selectedIntentRecord.worker
                              ? "Bootstrap running"
                              : "Bootstrap pending"
                            : selectedIntentRecord.worker
                              ? "In progress"
                              : "Unclaimed"}
                      </Badge>
                    </div>
                    <p className="text-sm break-words whitespace-pre-wrap">
                      {selectedIntentRecord.description}
                    </p>
                    <dl className="mt-3 grid grid-cols-2 gap-1 border-t pt-3 text-xs">
                      <dt className="text-muted-foreground">From</dt>
                      <dd className="text-right font-mono">{selectedIntentRecord.from.join(", ")}</dd>
                      <dt className="text-muted-foreground">To</dt>
                      <dd className="text-right font-mono">{selectedIntentRecord.to ?? "—"}</dd>
                      <dt className="text-muted-foreground">Worker</dt>
                      <dd className="text-right">{selectedIntentRecord.worker ?? "—"}</dd>
                      <dt className="text-muted-foreground">Created</dt>
                      <dd className="text-right">{formatTime(selectedIntentRecord.created_at)}</dd>
                    </dl>
                  </article>
                )}
                {producingIntent && selectedNode?.type === "fact" && (
                  <p className="text-xs text-muted-foreground">
                    Produced by {producingIntent.id} from {producingIntent.from.join(", ")}
                  </p>
                )}
              </div>
            )}

            {sideTab === "hints" && (
              <div className="space-y-2">
                {project.hints.length === 0 && (
                  <p className="py-10 text-center text-sm text-muted-foreground">No hints yet</p>
                )}
                {project.hints.map((hint) => (
                  <button
                    key={hint.id}
                    type="button"
                    onClick={() => {
                      const entry = timeline.find(
                        (item) => item.type === "hint_added" && item.targetId === hint.id,
                      );
                      if (!entry) return;
                      setSideTab("log");
                      openTimelineEntry(entry);
                      scrollToEntry(entry.id);
                    }}
                    className="w-full rounded-xl border bg-card p-3 text-left hover:bg-muted/40"
                  >
                    <div className="mb-2 flex items-center justify-between text-[11px] text-muted-foreground">
                      <span className="truncate font-medium">{hint.creator}</span>
                      <span>{formatTime(hint.created_at)}</span>
                    </div>
                    <p className="text-sm break-words whitespace-pre-wrap">{hint.content}</p>
                  </button>
                ))}
              </div>
            )}

            {sideTab === "log" && (
              <div ref={logRef} className="space-y-3">
                {selectedTimelineSummary && (
                  <div className="sticky top-0 z-10 rounded-xl border bg-background/95 p-3 shadow-sm">
                    {(["sequence", "time"] as const).map((kind) => {
                      const percent =
                        kind === "sequence"
                          ? selectedTimelineSummary.sequencePercent
                          : selectedTimelineSummary.timePercent;
                      const label =
                        kind === "sequence"
                          ? selectedTimelineSummary.sequenceLabel
                          : selectedTimelineSummary.timeLabel;
                      return (
                        <div key={kind} className="space-y-1 py-1">
                          <div className="flex justify-between text-[11px] text-muted-foreground">
                            <span className="capitalize">{kind}</span>
                            <span className="font-mono">{label}</span>
                          </div>
                          <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                            <div className="h-full rounded-full bg-primary/70" style={{ width: `${percent}%` }} />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
                {timeline.length === 0 && (
                  <p className="py-10 text-center text-sm text-muted-foreground">No activity yet</p>
                )}
                {timeline.map((entry) => (
                  <div key={entry.id} id={`timeline-entry-${entry.id}`} className="flex gap-2">
                    <div className="flex w-2.5 flex-col items-center">
                      <span className={cn("mt-1.5 size-2.5 rounded-full", timelineEventDotClass(entry))} />
                      {!entry.isLast && <span className="mt-1 w-px flex-1 bg-border" />}
                    </div>
                    <button
                      type="button"
                      onClick={() => openTimelineEntry(entry)}
                      className={cn(
                        "min-w-0 flex-1 rounded-lg px-1 py-1 text-left",
                        timelineEventIsInteractive(entry) ? "cursor-pointer hover:bg-muted/50" : "cursor-default",
                        selectedTimelineEntryId === entry.id && "bg-accent/60 ring-1 ring-border",
                      )}
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-[11px] text-muted-foreground">
                          {formatTime(entry.timestamp)}
                        </span>
                        <span
                          className={cn(
                            "rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase",
                            timelineEventBadgeClass(entry),
                          )}
                        >
                          {timelineEventBadge(entry)}
                        </span>
                        <span className="text-[11px] text-muted-foreground/70">
                          {formatTimelineDate(entry.timestamp)}
                        </span>
                      </div>
                      <p className="mt-1 text-sm break-words">{entry.title}</p>
                      {(entry.actor || entry.meta.length > 0) && (
                        <p className="mt-0.5 text-[11px] break-words text-muted-foreground">
                          {[entry.actor, ...entry.meta].filter(Boolean).join(" · ")}
                        </p>
                      )}
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </aside>
      </div>

      <Dialog open={intentOpen} onOpenChange={setIntentOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New intent</DialogTitle>
            <DialogDescription>From facts: {selectedFacts.join(", ")}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={intentText}
            onChange={(event) => setIntentText(event.target.value)}
            placeholder="What will you explore?"
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setIntentOpen(false)}>
              Cancel
            </Button>
            <Button variant="secondary" onClick={() => createIntent(false)} disabled={!intentText}>
              Declare
            </Button>
            <Button onClick={() => createIntent(true)} disabled={!intentText}>
              Declare &amp; Claim
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!concludeTarget} onOpenChange={(open) => !open && setConcludeTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Conclude intent</DialogTitle>
            <DialogDescription>{concludeTarget?.id}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={concludeText}
            onChange={(event) => setConcludeText(event.target.value)}
            placeholder="What did you find? (new fact)"
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setConcludeTarget(null)}>
              Cancel
            </Button>
            <Button onClick={concludeIntent} disabled={!concludeText}>
              Conclude
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={completeOpen} onOpenChange={setCompleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Complete project</DialogTitle>
            <DialogDescription>From facts: {selectedFacts.join(", ")}</DialogDescription>
          </DialogHeader>
          <Textarea
            value={completeText}
            onChange={(event) => setCompleteText(event.target.value)}
            placeholder="Why is the goal met?"
            rows={2}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setCompleteOpen(false)}>
              Cancel
            </Button>
            <Button onClick={completeProject} disabled={!completeText}>
              Complete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={hintOpen} onOpenChange={setHintOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add hint</DialogTitle>
          </DialogHeader>
          <Textarea
            value={hintText}
            onChange={(event) => setHintText(event.target.value)}
            placeholder="Strategy advice or note..."
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setHintOpen(false)}>
              Cancel
            </Button>
            <Button onClick={addHint} disabled={!hintText}>
              Add
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename project</DialogTitle>
          </DialogHeader>
          <Input value={renameText} onChange={(event) => setRenameText(event.target.value)} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameOpen(false)}>
              Cancel
            </Button>
            <Button onClick={renameProject} disabled={!renameText}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={reopenOpen} onOpenChange={setReopenOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reopen project</DialogTitle>
          </DialogHeader>
          <Textarea
            value={reopenText}
            onChange={(event) => setReopenText(event.target.value)}
            placeholder="Why is this project not actually complete?"
            rows={3}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setReopenOpen(false)}>
              Cancel
            </Button>
            <Button onClick={reopenProject} disabled={!reopenText}>
              Reopen
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete project</DialogTitle>
            <DialogDescription>This cannot be undone.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={deleteProject}>
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!snapshot} onOpenChange={(open) => !open && setSnapshot(null)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle>Snapshot</DialogTitle>
            <DialogDescription>{project.project.id}</DialogDescription>
          </DialogHeader>
          <div className="max-h-[60vh] overflow-auto rounded-lg border bg-muted/40 p-3">
            <pre className="text-xs leading-5">{snapshot}</pre>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => snapshot && copyText(snapshot).then(() => toast.show("Copied"))}
            >
              <Copy className="size-3.5" />
              Copy
            </Button>
            <Button onClick={() => setSnapshot(null)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
