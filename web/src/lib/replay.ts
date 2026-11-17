import type { ProjectDetail, TimelineEvent } from "@/lib/types";

export interface ReplayState {
  active: boolean;
  playing: boolean;
  stepMs: string;
  frameIndex: number;
  frames: { event: TimelineEvent; project: ProjectDetail }[];
  selectedProject: ProjectDetail | null;
}

export const EMPTY_REPLAY: ReplayState = {
  active: false,
  playing: false,
  stepMs: "1100",
  frameIndex: -1,
  frames: [],
  selectedProject: null,
};

export function clone<T>(value: T): T {
  return structuredClone(value);
}

export function replayEventWeight(event: TimelineEvent): number {
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

