export type ProjectStatus = "active" | "stopped" | "completed";

export type WorkerType = "claudecode" | "codex" | "pi" | "mock";
export type TaskType = "reason" | "explore" | "bootstrap";

export interface WorkerModel {
  name: string;
  type: WorkerType;
  task_types: TaskType[];
  max_running: number;
  priority: number;
  env: Record<string, string>;
  /** Live running count reported by the dispatcher, absent when unknown. */
  used?: number;
}

export interface Fact {
  id: string;
  description: string;
}

export interface Intent {
  id: string;
  from: string[];
  to: string | null;
  description: string;
  creator: string;
  worker: string | null;
  last_heartbeat_at: string | null;
  created_at: string;
  concluded_at: string | null;
}

export interface Hint {
  id: string;
  content: string;
  creator: string;
  created_at: string;
}

export interface ProjectReason {
  worker: string;
  trigger: string | null;
  started_at: string;
  last_heartbeat_at: string;
}

export interface ProjectMeta {
  id: string;
  title: string;
  status: ProjectStatus;
  bootstrap_enabled: boolean;
  created_at: string;
  reason: ProjectReason | null;
}

export interface ProjectSummary extends ProjectMeta {
  fact_count: number;
  intent_count: number;
  working_intent_count: number;
  unclaimed_intent_count: number;
  hint_count: number;
}

export interface ProjectDetail {
  project: ProjectMeta;
  facts: Fact[];
  intents: Intent[];
  hints: Hint[];
}

export interface ServerSettings {
  intent_timeout: number;
  reason_timeout: number;
}

export interface ModelTestResult {
  ok: boolean;
  status: number | null;
  detail: string;
  endpoint: string;
  duration_ms: number;
}

export type TimelineEventType =
  | "project_created"
  | "hint_added"
  | "reason_started"
  | "intent_declared"
  | "intent_running"
  | "intent_concluded"
  | "project_completed";

export interface TimelineEvent {
  id: string;
  type: TimelineEventType;
  timestamp: string;
  actor: string;
  title: string;
  meta: string[];
  targetType: "fact" | "intent" | "hints" | "reason";
  targetId: string;
  order: string | number;
  intentId: string | null;
  producedFactId: string | null;
  sourceFactIds: string[];
  isLast?: boolean;
}

export interface Selection {
  type: "fact" | "intent";
  id: string;
}
