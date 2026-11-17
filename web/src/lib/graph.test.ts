import { expect, test } from "vitest";

import { buildElements, collectIntentGroups, groupNodeId } from "@/lib/graph";
import type { Intent, ProjectDetail } from "@/lib/types";

function intent(id: string, creator: string, from: string[] = ["origin"], to: string | null = null): Intent {
  return {
    id,
    from,
    to,
    description: `intent ${id}`,
    creator,
    worker: null,
    last_heartbeat_at: null,
    created_at: "2026-01-01T00:00:01Z",
    concluded_at: to ? "2026-01-01T00:00:02Z" : null,
  };
}

function project(intents: Intent[]): ProjectDetail {
  return {
    project: {
      id: "proj_001",
      title: "t",
      status: "active",
      bootstrap_enabled: false,
      max_steps: null,
      schedule_id: null,
      created_at: "2026-01-01T00:00:00Z",
      reason: null,
    },
    facts: [
      { id: "origin", description: "start" },
      { id: "goal", description: "finish" },
    ],
    intents,
    hints: [],
  };
}

test("machine intents of one worker collapse into a single node", () => {
  const source = project([intent("i001", "m@node-a"), intent("i002", "m@node-a")]);

  const groups = collectIntentGroups(source);

  expect([...groups.keys()]).toEqual(["m@node-a"]);
  expect(groups.get("m@node-a")!.nodeId).toBe(groupNodeId("m@node-a"));

  const elements = buildElements(source);
  const nodes = elements.filter((element) => !("source" in element.data));
  const edges = elements.filter((element) => "source" in element.data);

  expect(nodes.filter((node) => node.data.nodeType === "group")).toHaveLength(1);
  expect(nodes.map((node) => node.data.id)).toEqual(["origin", "goal", groupNodeId("m@node-a")]);
  expect(edges).toHaveLength(1);
  expect(edges[0].data.target).toBe(groupNodeId("m@node-a"));
});

test("a single machine intent and human intents stay individual", () => {
  const source = project([intent("i001", "m@node-a"), intent("i002", "Human"), intent("i003", "Human")]);

  expect(collectIntentGroups(source).size).toBe(0);

  const nodes = buildElements(source).filter((element) => !("source" in element.data));
  expect(nodes.map((node) => node.data.id)).toEqual(["origin", "goal", "_ph_i001", "_ph_i002", "_ph_i003"]);
});

test("concluded members keep the group and add no target edges", () => {
  const source = project([
    intent("i001", "m@node-a", ["origin"], "f001"),
    intent("i002", "m@node-a", ["origin"], null),
  ]);
  source.facts.push({ id: "f001", description: "produced" });

  const edges = buildElements(source).filter((element) => "source" in element.data);

  expect(edges.map((edge) => edge.data.target)).toEqual([groupNodeId("m@node-a")]);
});
