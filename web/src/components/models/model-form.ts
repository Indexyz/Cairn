import type { TaskType, WorkerModel, WorkerType } from "@/lib/types";

export const WORKER_TYPES: WorkerType[] = ["claudecode", "codex", "pi", "mock"];
export const TASK_TYPES: TaskType[] = ["bootstrap", "reason", "explore"];

export const TYPE_ENV_KEYS: Record<WorkerType, string[]> = {
  claudecode: ["ANTHROPIC_MODEL", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN"],
  codex: ["CODEX_MODEL", "CODEX_BASE_URL", "OPENAI_API_KEY"],
  pi: ["PI_MODEL", "PI_BASE_URL", "PI_API_KEY", "PI_PROVIDER_API", "PI_MODEL_CONTEXT_WINDOW"],
  mock: [],
};

export interface EnvRow {
  id: number;
  key: string;
  value: string;
}

export interface ModelForm {
  name: string;
  type: WorkerType;
  task_types: TaskType[];
  max_running: number;
  priority: number;
  env: EnvRow[];
}

export function emptyForm(nextId: () => number): ModelForm {
  return {
    name: "",
    type: "claudecode",
    task_types: ["bootstrap", "reason", "explore"],
    max_running: 1,
    priority: 0,
    env: TYPE_ENV_KEYS.claudecode.map((key) => ({ id: nextId(), key, value: "" })),
  };
}

export function toForm(model: WorkerModel): ModelForm {
  return {
    name: model.name,
    type: model.type,
    task_types: [...model.task_types],
    max_running: model.max_running,
    priority: model.priority,
    env: Object.entries(model.env).map(([key, value]) => ({ id: nextEnvId(), key, value })),
  };
}

let envRowId = 0;
export function nextEnvId() {
  return envRowId++;
}

export function toPayload(form: ModelForm): WorkerModel {
  const env: Record<string, string> = {};
  for (const row of form.env) {
    const key = row.key.trim();
    if (key) env[key] = row.value;
  }
  return {
    name: form.name.trim(),
    type: form.type,
    task_types: form.task_types,
    max_running: form.max_running,
    priority: form.priority,
    env,
  };
}

