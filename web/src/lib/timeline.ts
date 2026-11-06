import type { Intent, ProjectDetail, TimelineEvent } from "@/lib/types";

export function isBootstrapIntent(intent: Intent | null | undefined): boolean {
  return Boolean(
    intent &&
      intent.description === "bootstrap" &&
      intent.creator === "dispatcher.bootstrap" &&
      intent.from.length === 1 &&
      intent.from[0] === "origin" &&
      intent.to === null,
  );
}

export function getProducingIntent(
  project: ProjectDetail | null,
  factId: string,
): Intent | null {
  return project ? (project.intents.find((intent) => intent.to === factId) ?? null) : null;
}

function resolveBucket(bucket: TimelineEvent[]): TimelineEvent[] {
  if (bucket.length <= 1) return bucket;

  const eventById = new Map(bucket.map((event) => [event.id, event]));
  const declareEventIdByIntent = new Map<string, string>();
  const produceEventIdByFact = new Map<string, string>();
  const outgoing = new Map(bucket.map((event) => [event.id, new Set<string>()]));
  const incomingCount = new Map(bucket.map((event) => [event.id, 0]));

  const addDependency = (beforeId?: string, afterId?: string) => {
    if (!beforeId || !afterId || beforeId === afterId) return;
    const deps = outgoing.get(beforeId);
    if (!deps || deps.has(afterId)) return;
    deps.add(afterId);
    incomingCount.set(afterId, (incomingCount.get(afterId) ?? 0) + 1);
  };

  for (const event of bucket) {
    if (event.type === "intent_declared" && event.intentId) {
      declareEventIdByIntent.set(event.intentId, event.id);
    }
    if (event.producedFactId) produceEventIdByFact.set(event.producedFactId, event.id);
  }

  for (const event of bucket) {
    if ((event.type === "intent_concluded" || event.type === "project_completed") && event.intentId) {
      addDependency(declareEventIdByIntent.get(event.intentId), event.id);
    }
    for (const factId of event.sourceFactIds) {
      addDependency(produceEventIdByFact.get(factId), event.id);
    }
  }

  const ready = bucket
    .filter((event) => (incomingCount.get(event.id) ?? 0) === 0)
    .sort((a, b) => Number(a.order) - Number(b.order));
  const ordered: TimelineEvent[] = [];

  while (ready.length > 0) {
    const event = ready.shift();
    if (!event) break;
    ordered.push(event);
    for (const nextId of outgoing.get(event.id) ?? []) {
      incomingCount.set(nextId, (incomingCount.get(nextId) ?? 0) - 1);
      if (incomingCount.get(nextId) === 0) {
        const next = eventById.get(nextId);
        if (next) {
          ready.push(next);
          ready.sort((a, b) => Number(a.order) - Number(b.order));
        }
      }
    }
  }

  if (ordered.length === bucket.length) return ordered;
  const placed = new Set(ordered.map((event) => event.id));
  return [
    ...ordered,
    ...bucket
      .filter((event) => !placed.has(event.id))
      .sort((a, b) => Number(a.order) - Number(b.order)),
  ];
}

export function buildTimeline(project: ProjectDetail | null): TimelineEvent[] {
  if (!project) return [];
  const events: TimelineEvent[] = [];
  let order = 0;
  const origin = project.facts.find((fact) => fact.id === "origin");
  const goal = project.facts.find((fact) => fact.id === "goal");

  events.push({
    id: `project-created-${project.project.id}`,
    type: "project_created",
    timestamp: project.project.created_at,
    actor: "system",
    title: project.project.title,
    meta: [origin?.description ?? null, goal ? `goal: ${goal.description}` : null].filter(
      (item): item is string => Boolean(item),
    ),
    targetType: "fact",
    targetId: "origin",
    order: order++,
    intentId: null,
    producedFactId: null,
    sourceFactIds: [],
  });

  for (const hint of project.hints) {
    events.push({
      id: `hint-${hint.id}`,
      type: "hint_added",
      timestamp: hint.created_at,
      actor: hint.creator,
      title: hint.content,
      meta: [],
      targetType: "hints",
      targetId: hint.id,
      order: order++,
      intentId: null,
      producedFactId: null,
      sourceFactIds: [],
    });
  }

  for (const intent of project.intents) {
    events.push({
      id: `intent-declared-${intent.id}`,
      type: "intent_declared",
      timestamp: intent.created_at,
      actor: intent.creator,
      title: intent.description,
      meta: [intent.id, `from ${intent.from.join(", ")}`],
      targetType: "intent",
      targetId: intent.id,
      order: order++,
      intentId: intent.id,
      producedFactId: null,
      sourceFactIds: [...intent.from],
    });

    if (!intent.concluded_at || !intent.to) continue;

    if (intent.to === "goal") {
      events.push({
        id: `project-completed-${intent.id}`,
        type: "project_completed",
        timestamp: intent.concluded_at,
        actor: intent.worker ?? intent.creator,
        title: intent.description,
        meta: [intent.id, `from ${intent.from.join(", ")}`],
        targetType: "fact",
        targetId: "goal",
        order: order++,
        intentId: intent.id,
        producedFactId: "goal",
        sourceFactIds: [...intent.from],
      });
      continue;
    }

    const fact = project.facts.find((item) => item.id === intent.to);
    events.push({
      id: `intent-concluded-${intent.id}`,
      type: "intent_concluded",
      timestamp: intent.concluded_at,
      actor: intent.worker ?? intent.creator,
      title: fact ? fact.description : intent.description,
      meta: [intent.id, `produced ${intent.to}`, `from ${intent.from.join(", ")}`],
      targetType: "fact",
      targetId: intent.to,
      order: order++,
      intentId: intent.id,
      producedFactId: intent.to,
      sourceFactIds: [...intent.from],
    });
  }

  const chronological = [...events].sort(
    (a, b) => a.timestamp.localeCompare(b.timestamp) || Number(a.order) - Number(b.order),
  );

  const resolved: TimelineEvent[] = [];
  for (let index = 0; index < chronological.length; ) {
    const bucket = [chronological[index]];
    let cursor = index + 1;
    while (cursor < chronological.length && chronological[cursor].timestamp === chronological[index].timestamp) {
      bucket.push(chronological[cursor]);
      cursor += 1;
    }
    resolved.push(...resolveBucket(bucket));
    index = cursor;
  }

  return resolved.map((entry, index) => ({ ...entry, isLast: index === resolved.length - 1 }));
}

export function timelineEventBadge(entry: TimelineEvent): string {
  const labels: Record<string, string> = {
    project_created: "Project",
    hint_added: "Hint",
    reason_started: "Reason",
    intent_declared: "Intent",
    intent_running: "Execute",
    intent_concluded: "Conclude",
    project_completed: "Complete",
  };
  return labels[entry.type] ?? "Event";
}

export function timelineEventBadgeClass(entry: TimelineEvent): string {
  const classes: Record<string, string> = {
    project_created: "bg-slate-100 text-slate-600",
    hint_added: "bg-amber-50 text-amber-700",
    reason_started: "bg-sky-50 text-sky-700",
    intent_declared: "bg-violet-50 text-violet-700",
    intent_running: "bg-amber-50 text-amber-700",
    intent_concluded: "bg-teal-50 text-teal-700",
    project_completed: "bg-rose-50 text-rose-700",
  };
  return classes[entry.type] ?? "bg-slate-100 text-slate-600";
}

export function timelineEventDotClass(entry: TimelineEvent): string {
  const classes: Record<string, string> = {
    project_created: "bg-slate-400",
    hint_added: "bg-amber-400",
    reason_started: "bg-sky-400",
    intent_declared: "bg-violet-400",
    intent_running: "bg-amber-400",
    intent_concluded: "bg-teal-400",
    project_completed: "bg-rose-400",
  };
  return classes[entry.type] ?? "bg-slate-300";
}

export function timelineEventIsInteractive(entry: TimelineEvent | null): boolean {
  if (!entry) return false;
  return ["fact", "intent", "hints", "reason"].includes(entry.targetType);
}

export function timelineEventTriggersGraphFocus(entry: TimelineEvent | null): boolean {
  if (!entry) return false;
  return entry.targetType === "fact" || entry.targetType === "intent";
}
