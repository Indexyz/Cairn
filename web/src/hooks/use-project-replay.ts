import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";
import {
  clone,
  EMPTY_REPLAY,
  replayEventWeight,
  type ReplayState,
} from "@/lib/replay";
import type { ProjectDetail, TimelineEvent } from "@/lib/types";

interface Options {
  project: ProjectDetail | null;
  projectId: string;
  getTimeline: () => TimelineEvent[];
  onStart: () => void;
  onExit: () => void;
  onFrame: (project: ProjectDetail) => void;
}

export function useProjectReplay({ project, projectId, getTimeline, onStart, onExit, onFrame }: Options) {
  const { t } = useTranslation();
  const toast = useToast();
  const [replay, setReplay] = useState<ReplayState>(EMPTY_REPLAY);
  const [replayVersion, setReplayVersion] = useState(0);
  const timer = useRef<number | null>(null);
  const stateRef = useRef(replay);
  const handlers = useRef({ onStart, onExit, onFrame });

  useEffect(() => {
    stateRef.current = replay;
  });
  useEffect(() => {
    handlers.current = { onStart, onExit, onFrame };
  });

  function stopReplayTimer() {
    if (timer.current !== null) {
      window.clearTimeout(timer.current);
      timer.current = null;
    }
  }

  const scheduleReplayTick = useCallback((state: ReplayState) => {
    if (!state.active || !state.playing) return;
    if (state.frameIndex >= state.frames.length - 1) {
      const stopped = { ...state, playing: false };
      stateRef.current = stopped;
      setReplay(stopped);
      return;
    }
    stopReplayTimer();
    const frame = state.frames[state.frameIndex];
    const delay = Math.round((Number(state.stepMs) || 1100) * replayEventWeight(frame.event));
    // The ref keeps the timer free of updater side effects.
    timer.current = window.setTimeout(() => {
      const current = stateRef.current;
      if (!current.active) return;
      const nextIndex = current.frameIndex + 1;
      const next =
        nextIndex >= current.frames.length
          ? { ...current, playing: false }
          : { ...current, frameIndex: nextIndex };
      stateRef.current = next;
      setReplay(next);
      scheduleReplayTick(next);
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

    for (const event of getTimeline()) {
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
    handlers.current.onStart();
    try {
      const source = await api<ProjectDetail>("GET", `/projects/${projectId}`);
      const frames = buildReplayFrames(source);
      if (frames.length === 0) {
        toast.show(t("project.replay.empty"), "error");
        return;
      }
      stopReplayTimer();
      const next: ReplayState = {
        active: true,
        playing: true,
        stepMs: replay.stepMs,
        frameIndex: 0,
        frames,
        selectedProject: source,
      };
      stateRef.current = next;
      setReplay(next);
      handlers.current.onFrame(clone(frames[0].project));
      setReplayVersion((value) => value + 1);
      scheduleReplayTick(next);
    } catch (error) {
      toast.show((error as Error).message, "error");
    }
  }

  // Keep the displayed graph in sync with the replay frame.
  useEffect(() => {
    if (!replay.active) return;
    const frame = replay.frames[replay.frameIndex];
    if (frame) handlers.current.onFrame(clone(frame.project));
  }, [replay.active, replay.frameIndex, replay.frames]);

  useEffect(() => {
    if (replay.active && replay.playing) scheduleReplayTick(replay);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [replay.stepMs]);

  useEffect(() => () => stopReplayTimer(), []);

  async function exitReplay() {
    stopReplayTimer();
    stateRef.current = EMPTY_REPLAY;
    setReplay(EMPTY_REPLAY);
    setReplayVersion((value) => value + 1);
    await handlers.current.onExit();
  }

  function toggleReplayPlayback() {
    if (replay.playing) {
      stopReplayTimer();
      const next = { ...replay, playing: false };
      stateRef.current = next;
      setReplay(next);
      return;
    }
    if (replay.frameIndex >= replay.frames.length - 1) {
      restartReplay();
      return;
    }
    const next = { ...replay, playing: true };
    stateRef.current = next;
    setReplay(next);
    scheduleReplayTick(next);
  }

  function restartReplay() {
    if (!replay.active || replay.frames.length === 0) return;
    stopReplayTimer();
    handlers.current.onStart();
    const next = { ...replay, playing: true, frameIndex: 0 };
    stateRef.current = next;
    setReplay(next);
    setReplayVersion((value) => value + 1);
    scheduleReplayTick(next);
  }

  function setReplaySpeed(stepMs: string) {
    setReplay((current) => ({ ...current, stepMs }));
  }

  function resetReplay() {
    stopReplayTimer();
    stateRef.current = EMPTY_REPLAY;
    setReplay(EMPTY_REPLAY);
    setReplayVersion((value) => value + 1);
  }

  return {
    replay,
    replayVersion,
    startReplay,
    exitReplay,
    toggleReplayPlayback,
    restartReplay,
    setReplaySpeed,
    resetReplay,
  };
}

export type { ReplayState };
