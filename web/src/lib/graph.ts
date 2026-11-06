import type { Core, ElementDefinition, LayoutOptions, StylesheetStyle } from "cytoscape";

import { isBootstrapIntent } from "@/lib/timeline";
import type { LayoutMode } from "@/lib/format";
import type { Intent, ProjectDetail } from "@/lib/types";

export function summarizeFactLabel(fact: { id: string; description: string }): string {
  if (fact.id === "origin") return "Origin";
  if (fact.id === "goal") return "Goal";
  const normalized = fact.description.replace(/\s+/g, " ").trim();
  const chars = Array.from(normalized);
  if (chars.length <= 24) return normalized || fact.id;
  return `${chars.slice(0, 24).join("")}…`;
}

function estimateLabelCharWidth(char: string, fontSize: number): number {
  if (/\s/.test(char)) return fontSize * 0.35;
  if (/[\u1100-\u115F\u2E80-\uA4CF\uAC00-\uD7A3\uF900-\uFAFF\uFE10-\uFE6F\uFF00-\uFF60\uFFE0-\uFFE6]/.test(char)) {
    return fontSize;
  }
  return fontSize * 0.58;
}

export function measureWrappedText(text: string, maxWidth: number, fontSize: number) {
  const content = text.trim() || " ";
  const lines: number[] = [];
  let currentWidth = 0;
  let currentChars = 0;
  let maxLineWidth = 0;

  const pushLine = () => {
    if (currentChars === 0 && lines.length > 0) lines.push(0);
    else if (currentChars > 0) lines.push(currentWidth);
    maxLineWidth = Math.max(maxLineWidth, currentWidth);
    currentWidth = 0;
    currentChars = 0;
  };

  for (const char of Array.from(content)) {
    if (char === "\n") {
      pushLine();
      continue;
    }
    const charWidth = estimateLabelCharWidth(char, fontSize);
    if (currentChars > 0 && currentWidth + charWidth > maxWidth) pushLine();
    currentWidth += charWidth;
    currentChars += 1;
  }

  pushLine();

  const lineHeight = fontSize * 1.35;
  return {
    width: Math.min(maxWidth, Math.max(fontSize * 1.6, maxLineWidth)),
    height: Math.max(1, lines.length) * lineHeight,
  };
}

export function factNodeSize(label: string, nodeType: string) {
  const preset =
    nodeType === "fact"
      ? { fontSize: 10, maxTextWidth: 116, minWidth: 52, minHeight: 34, paddingX: 10, paddingY: 10 }
      : { fontSize: 11, maxTextWidth: 92, minWidth: 58, minHeight: 38, paddingX: 10, paddingY: 10 };

  const measured = measureWrappedText(label, preset.maxTextWidth, preset.fontSize);
  return {
    width: Math.max(preset.minWidth, Math.ceil(measured.width + preset.paddingX * 2)),
    height: Math.max(preset.minHeight, Math.ceil(measured.height + preset.paddingY * 2)),
  };
}

export function openIntentNodeType(intent: Intent): string {
  if (isBootstrapIntent(intent)) return intent.worker ? "bootstrap_running" : "bootstrap_pending";
  return intent.worker ? "in_progress" : "unclaimed";
}

export function openIntentNodeSize(intent: Intent) {
  if (isBootstrapIntent(intent)) return { width: 82, height: 30 };
  return { width: 22, height: 22 };
}

export function buildElements(project: ProjectDetail): ElementDefinition[] {
  const nodes: ElementDefinition[] = [];
  const edges: ElementDefinition[] = [];

  for (const fact of project.facts) {
    const nodeType = fact.id === "origin" ? "origin" : fact.id === "goal" ? "goal" : "fact";
    const label = summarizeFactLabel(fact);
    const size = factNodeSize(label, nodeType);
    nodes.push({
      data: {
        id: fact.id,
        label,
        description: fact.description,
        nodeType,
        width: size.width,
        height: size.height,
      },
    });
  }

  for (const intent of project.intents) {
    if (intent.to) {
      for (const source of intent.from) {
        edges.push({
          data: {
            id: `${intent.id}_${source}`,
            source,
            target: intent.to,
            intentId: intent.id,
            label: intent.description,
            status: "concluded",
          },
        });
      }
      continue;
    }

    const placeholderId = `_ph_${intent.id}`;
    const size = openIntentNodeSize(intent);
    const nodeType = openIntentNodeType(intent);
    nodes.push({
      data: {
        id: placeholderId,
        label: isBootstrapIntent(intent) ? "Bootstrap" : "?",
        description: intent.description,
        nodeType,
        intentId: intent.id,
        width: size.width,
        height: size.height,
      },
    });
    for (const source of intent.from) {
      edges.push({
        data: {
          id: `${intent.id}_${source}`,
          source,
          target: placeholderId,
          intentId: intent.id,
          label: intent.description,
          status: nodeType,
        },
      });
    }
    if (isBootstrapIntent(intent)) {
      edges.push({
        data: {
          id: `${intent.id}_goal`,
          source: placeholderId,
          target: "goal",
          intentId: intent.id,
          label: "",
          status: nodeType,
          edgeType: "bootstrap_scope",
        },
      });
    }
  }

  return [...nodes, ...edges];
}

export function graphStyles(): StylesheetStyle[] {
  const common = {
    "text-valign": "center" as const,
    "text-halign": "center" as const,
    "font-family": "-apple-system,BlinkMacSystemFont,Inter,sans-serif",
  };
  return [
    {
      selector: 'node[nodeType="origin"]',
      style: { ...common, shape: "round-rectangle", "background-color": "#0f766e", label: "data(label)", color: "#fff", "font-size": "11px", "font-weight": "bold", "text-wrap": "wrap", "text-max-width": "92px", "text-overflow-wrap": "anywhere", width: "data(width)", height: "data(height)", "border-width": 0 },
    },
    {
      selector: 'node[nodeType="goal"]',
      style: { ...common, shape: "round-rectangle", "background-color": "#e11d48", label: "data(label)", color: "#fff", "font-size": "11px", "font-weight": "bold", "text-wrap": "wrap", "text-max-width": "92px", "text-overflow-wrap": "anywhere", width: "data(width)", height: "data(height)", "border-width": 0 },
    },
    {
      selector: 'node[nodeType="fact"]',
      style: { ...common, shape: "round-rectangle", "background-color": "#4f46e5", label: "data(label)", color: "#fff", "font-size": "10px", "font-weight": "bold", "text-wrap": "wrap", "text-max-width": "116px", "text-overflow-wrap": "anywhere", width: "data(width)", height: "data(height)", "border-width": 0 },
    },
    {
      selector: 'node[nodeType="in_progress"]',
      style: { ...common, shape: "ellipse", "background-color": "#f59e0b", "background-opacity": 0.8, label: "?", color: "#fff", "font-size": "11px", "font-weight": "bold", width: 22, height: 22, "border-width": 2, "border-color": "#d97706" },
    },
    {
      selector: 'node[nodeType="unclaimed"]',
      style: { ...common, shape: "ellipse", "background-color": "#cbd5e1", "background-opacity": 0.5, label: "?", color: "#94a3b8", "font-size": "11px", "font-weight": "bold", width: 20, height: 20, "border-width": 1.5, "border-color": "#94a3b8", "border-style": "dashed" },
    },
    {
      selector: 'node[nodeType="bootstrap_pending"]',
      style: { ...common, shape: "round-rectangle", "background-color": "#fff7ed", "background-opacity": 0.96, label: "data(label)", color: "#c2410c", "font-size": "10px", "font-weight": "bold", width: "data(width)", height: "data(height)", "border-width": 1.5, "border-color": "#fdba74", "border-style": "dashed", "text-wrap": "wrap", "text-max-width": "70px" },
    },
    {
      selector: 'node[nodeType="bootstrap_running"]',
      style: { ...common, shape: "round-rectangle", "background-color": "#fb923c", "background-opacity": 0.96, label: "data(label)", color: "#fff7ed", "font-size": "10px", "font-weight": "bold", width: "data(width)", height: "data(height)", "border-width": 2, "border-color": "#ea580c", "text-wrap": "wrap", "text-max-width": "70px" },
    },
    {
      selector: 'edge[status="concluded"]',
      style: { width: 2, "line-color": "#6ee7b7", "target-arrow-color": "#6ee7b7", "target-arrow-shape": "triangle", "curve-style": "bezier", label: "data(label)", "font-size": "7px", color: "#94a3b8", "text-rotation": "autorotate", "text-margin-y": -9, "text-max-width": "80px", "text-wrap": "ellipsis", "text-background-color": "#f8fafc", "text-background-opacity": 0.85, "text-background-padding": "2px", "text-events": "yes", "arrow-scale": 0.9 },
    },
    {
      selector: 'edge[status="in_progress"]',
      style: { width: 2, "line-color": "#fbbf24", "line-style": "dashed", "line-dash-pattern": [8, 4], "line-dash-offset": 0, "target-arrow-color": "#fbbf24", "target-arrow-shape": "triangle", "curve-style": "bezier", label: "data(label)", "font-size": "7px", color: "#b45309", "text-rotation": "autorotate", "text-margin-y": -9, "text-max-width": "80px", "text-wrap": "ellipsis", "text-background-color": "#fffbeb", "text-background-opacity": 0.85, "text-background-padding": "2px", "text-events": "yes", "arrow-scale": 0.9 },
    },
    {
      selector: 'edge[status="unclaimed"]',
      style: { width: 1.5, "line-color": "#cbd5e1", "line-style": "dashed", "line-dash-pattern": [5, 5], "target-arrow-color": "#cbd5e1", "target-arrow-shape": "triangle", "curve-style": "bezier", label: "data(label)", "font-size": "7px", color: "#94a3b8", "text-rotation": "autorotate", "text-margin-y": -9, "text-max-width": "80px", "text-wrap": "ellipsis", "text-background-color": "#f8fafc", "text-background-opacity": 0.85, "text-background-padding": "2px", "text-events": "yes", "arrow-scale": 0.7 },
    },
    {
      selector: 'edge[status="bootstrap_pending"]',
      style: { width: 2, "line-color": "#fdba74", "line-style": "dashed", "line-dash-pattern": [8, 4], "line-dash-offset": 0, "target-arrow-color": "#fdba74", "target-arrow-shape": "triangle", "curve-style": "bezier", label: "data(label)", "font-size": "7px", color: "#c2410c", "text-rotation": "autorotate", "text-margin-y": -9, "text-max-width": "88px", "text-wrap": "ellipsis", "text-background-color": "#fff7ed", "text-background-opacity": 0.92, "text-background-padding": "2px", "text-events": "yes", "arrow-scale": 0.85 },
    },
    {
      selector: 'edge[status="bootstrap_running"]',
      style: { width: 2.5, "line-color": "#fb923c", "line-style": "dashed", "line-dash-pattern": [10, 4], "line-dash-offset": 0, "target-arrow-color": "#fb923c", "target-arrow-shape": "triangle", "curve-style": "bezier", label: "data(label)", "font-size": "7px", color: "#c2410c", "text-rotation": "autorotate", "text-margin-y": -9, "text-max-width": "88px", "text-wrap": "ellipsis", "text-background-color": "#fff7ed", "text-background-opacity": 0.92, "text-background-padding": "2px", "text-events": "yes", "arrow-scale": 0.95 },
    },
    {
      selector: 'edge[edgeType="bootstrap_scope"]',
      style: { label: "", width: 1.8, "curve-style": "bezier", "line-style": "dotted", "line-dash-pattern": [2, 5], "target-arrow-shape": "triangle-backcurve", "arrow-scale": 0.75, "target-distance-from-node": 2 },
    },
    { selector: ".highlight", style: { "z-index": 999 } },
    { selector: "edge.highlight", style: { "z-index": 999 } },
    { selector: "node.focus", style: { "border-width": 3, "border-color": "#312e81", "border-opacity": 0.95, "z-index": 1000 } },
    { selector: "edge.focus", style: { "z-index": 1000, "overlay-color": "#93c5fd", "overlay-opacity": 0.22, "overlay-padding": 5 } },
    { selector: "node.selected-fact", style: { "border-width": 0, "underlay-color": "#93c5fd", "underlay-padding": 8, "underlay-opacity": 0.28, "z-index": 1001 } },
    { selector: ".faded", style: { opacity: 0.5 } },
    { selector: ".fresh", style: { opacity: 0 } },
  ];
}

export function layoutEngine(mode: LayoutMode) {
  if (mode.startsWith("elk")) return "elk";
  return mode.startsWith("klay") ? "klay" : "dagre";
}

export function layoutDirection(mode: LayoutMode) {
  return mode.endsWith("_lr") ? "LR" : "TB";
}

export function layoutOpts(mode: LayoutMode, animate = true): LayoutOptions {
  const direction = layoutDirection(mode);
  const engine = layoutEngine(mode);

  if (engine === "elk") {
    return {
      name: "elk",
      fit: true,
      padding: 50,
      animate,
      animationDuration: 350,
      animationEasing: "ease-in-out-cubic",
      elk: {
        algorithm: "layered",
        "elk.direction": direction === "TB" ? "DOWN" : "RIGHT",
        "elk.aspectRatio": "1.5",
        "elk.layered.nodePlacement.strategy": "BRANDES_KOEPF",
        "elk.spacing.nodeNode": "50",
        "elk.layered.spacing.nodeNodeBetweenLayers": "80",
        "elk.spacing.edgeNode": "25",
        "elk.layered.crossingMinimization.strategy": "LAYER_SWEEP",
        "elk.layered.nodePlacement.bk.fixedAlignment": "BALANCED",
      },
    } as LayoutOptions;
  }

  if (engine === "klay") {
    const horizontal = direction === "LR";
    return {
      name: "klay",
      fit: true,
      padding: 50,
      animate,
      animationDuration: 400,
      animationEasing: "ease-in-out-cubic",
      klay: {
        direction: direction === "TB" ? "DOWN" : "RIGHT",
        edgeRouting: "POLYLINE",
        crossingMinimization: "LAYER_SWEEP",
        nodeLayering: "NETWORK_SIMPLEX",
        nodePlacement: "BRANDES_KOEPF",
        separateConnectedComponents: true,
        spacing: horizontal ? 52 : 40,
        inLayerSpacingFactor: horizontal ? 1.15 : 1.0,
        thoroughness: 8,
      },
    } as LayoutOptions;
  }

  return {
    name: "dagre",
    rankDir: direction,
    nodeSep: 60,
    rankSep: 80,
    padding: 50,
    fit: true,
    animate,
    animationDuration: 400,
    animationEasing: "ease-in-out-cubic",
  } as LayoutOptions;
}

export function collectFactLineage(project: ProjectDetail, factId: string) {
  const upstreamFacts = new Set<string>();
  const upstreamIntents = new Set<string>();

  const walkFact = (id: string) => {
    if (upstreamFacts.has(id)) return;
    upstreamFacts.add(id);
    for (const intent of project.intents) {
      if (intent.to === id) walkIntent(intent.id);
    }
  };

  const walkIntent = (intentId: string) => {
    if (upstreamIntents.has(intentId)) return;
    upstreamIntents.add(intentId);
    const intent = project.intents.find((item) => item.id === intentId);
    if (!intent) return;
    for (const source of intent.from) walkFact(source);
  };

  walkFact(factId);
  return { upstreamFacts, upstreamIntents };
}

function collectIntentElements(intent: Intent, nodeIds: Set<string>, edgeIds: Set<string>) {
  if (intent.to) nodeIds.add(intent.to);
  else nodeIds.add(`_ph_${intent.id}`);
  for (const source of intent.from) {
    nodeIds.add(source);
    edgeIds.add(`${intent.id}_${source}`);
  }
  if (isBootstrapIntent(intent)) {
    nodeIds.add("goal");
    edgeIds.add(`${intent.id}_goal`);
  }
}

export function highlightIntent(cy: Core, project: ProjectDetail, intentId: string, selectedFacts: string[]) {
  const intent = project.intents.find((item) => item.id === intentId);
  if (!intent) return;
  cy.elements().removeClass("highlight focus faded");

  const nodeIds = new Set<string>();
  const edgeIds = new Set<string>();
  collectIntentElements(intent, nodeIds, edgeIds);

  const nodes = cy.nodes().filter((node) => nodeIds.has(node.id()));
  const edges = cy.edges().filter((edge) => edgeIds.has(edge.id()));
  nodes.addClass("highlight");
  edges.addClass("focus");

  const visible = nodes.add(edges);
  cy.elements().not(visible).addClass("faded");
  syncFactSelections(cy, selectedFacts);
}

export function highlightFact(cy: Core, project: ProjectDetail, factId: string, selectedFacts: string[]) {
  const { upstreamFacts, upstreamIntents } = collectFactLineage(project, factId);
  const nodeIds = new Set<string>(upstreamFacts);
  const edgeIds = new Set<string>();

  for (const intentId of upstreamIntents) {
    const intent = project.intents.find((item) => item.id === intentId);
    if (intent) collectIntentElements(intent, nodeIds, edgeIds);
  }

  cy.elements().removeClass("highlight focus faded");
  const nodes = cy.nodes().filter((node) => nodeIds.has(node.id()));
  const edges = cy.edges().filter((edge) => edgeIds.has(edge.id()));
  const focusNode = cy.getElementById(factId);

  nodes.addClass("highlight");
  edges.addClass("highlight");
  focusNode.addClass("focus");

  const visible = nodes.add(edges).add(focusNode);
  cy.elements().not(visible).addClass("faded");
  syncFactSelections(cy, selectedFacts);
}

export function syncFactSelections(cy: Core, selectedFacts: string[]) {
  cy.nodes().removeClass("selected-fact");
  for (const factId of selectedFacts) {
    const node = cy.getElementById(factId);
    if (node.length > 0) node.addClass("selected-fact");
  }
}

export function highlightMultiFact(cy: Core, selectedFacts: string[]) {
  cy.elements().removeClass("highlight focus faded selected-fact");
  syncFactSelections(cy, selectedFacts);
}
