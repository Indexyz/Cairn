import { useCallback, useEffect, useRef } from "react";
import cytoscape, { type Core, type ElementDefinition, type NodeSingular } from "cytoscape";
import dagre from "cytoscape-dagre";
import klay from "cytoscape-klay";
import elk from "cytoscape-elk";

import {
  buildElements,
  graphStyles,
  highlightFact,
  highlightGroup,
  highlightIntent,
  highlightMultiFact,
  layoutDirection,
  layoutOpts,
  syncFactSelections,
} from "@/lib/graph";
import type { ElementLayoutMode } from "@/lib/graph-types";
import type { LayoutMode } from "@/lib/format";
import type { ProjectDetail, Selection } from "@/lib/types";

cytoscape.use(dagre);
cytoscape.use(klay);
cytoscape.use(elk);

interface Options {
  containerRef: React.RefObject<HTMLDivElement | null>;
  resetKey: number;
  project: ProjectDetail | null;
  layoutMode: LayoutMode;
  theme?: "light" | "dark";
  selectedNode: Selection | null;
  selectedFacts: string[];
  onNodeTap: (node: NodeSingular, shiftKey: boolean) => void;
  onEdgeTap: (intentId: string | undefined, groupId: string | undefined) => void;
  onBackgroundTap: () => void;
}

export function useCytoscape(options: Options) {
  const { containerRef, project, layoutMode, theme = "light", selectedNode, selectedFacts, resetKey } = options;
  const cyRef = useRef<Core | null>(null);
  const handlers = useRef(options);
  const layoutRef = useRef<LayoutMode>(layoutMode);
  const themeRef = useRef<"light" | "dark">(theme);

  // Written in effects rather than during render: React may replay or drop render work.
  useEffect(() => {
    handlers.current = options;
  });
  useEffect(() => {
    layoutRef.current = layoutMode;
  }, [layoutMode]);
  useEffect(() => {
    themeRef.current = theme;
    const cy = cyRef.current;
    if (cy) cy.style(graphStyles(theme));
  }, [theme]);

  const positionsKey = useRef<ProjectDetail | null>(null);
  const signatureRef = useRef<string | null>(null);
  const fadeFrames = useRef<Set<number>>(new Set());
  // The container only exists once the project is loaded, so initialisation must re-run then.
  const hasProject = Boolean(project);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const instance = cytoscape({
      container,
      elements: [],
      style: graphStyles(themeRef.current),
      layout: layoutOpts(layoutRef.current),
      minZoom: 0.15,
      maxZoom: 3.5,
    });
    cyRef.current = instance;
    signatureRef.current = null;
    // Handy for debugging graph rendering from the console or a headless check.
    (window as unknown as { __cy?: Core }).__cy = instance;

    instance.on("tap", "node", (event) => {
      handlers.current.onNodeTap(event.target as NodeSingular, event.originalEvent.shiftKey);
    });
    instance.on("tap", "edge", (event) => {
      handlers.current.onEdgeTap(event.target.data("intentId"), event.target.data("groupId"));
    });
    instance.on("tap", (event) => {
      if (event.target === instance) handlers.current.onBackgroundTap();
    });

    const observer = new ResizeObserver(() => {
      instance.resize();
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
      instance.destroy();
      cyRef.current = null;
      signatureRef.current = null;
      fadeFrames.current.forEach((frame) => window.cancelAnimationFrame(frame));
      fadeFrames.current.clear();
      delete (window as unknown as { __cy?: Core }).__cy;
    };
  }, [containerRef, resetKey, hasProject]);

  const snapshotPositions = useCallback(() => {
    const positions = new Map<string, { x: number; y: number }>();
    cyRef.current?.nodes().forEach((node) => {
      positions.set(node.id(), { x: node.position("x"), y: node.position("y") });
    });
    return positions;
  }, []);

  const anchorFromIds = useCallback(
    (ids: string[], previous: Map<string, { x: number; y: number }>, offset = 36) => {
      const cy = cyRef.current;
      if (!cy) return null;
      const anchors: { x: number; y: number }[] = [];
      for (const id of ids) {
        const existing = cy.getElementById(id);
        if (existing.length > 0) {
          anchors.push({ x: existing.position("x"), y: existing.position("y") });
          continue;
        }
        const stored = previous.get(id);
        if (stored) anchors.push(stored);
      }
      if (anchors.length === 0) return null;
      const average = anchors.reduce(
        (acc, pos) => ({ x: acc.x + pos.x, y: acc.y + pos.y }),
        { x: 0, y: 0 },
      );
      const center = { x: average.x / anchors.length, y: average.y / anchors.length };
      return layoutDirection(layoutRef.current) === "TB"
        ? { x: center.x, y: center.y + offset }
        : { x: center.x + offset, y: center.y };
    },
    [],
  );

  const pulseActive = useCallback(() => {
    const cy = cyRef.current;
    if (!cy) return;
    cy.nodes('[nodeType="in_progress"], [nodeType="bootstrap_running"]').forEach((node) => {
      if (node.scratch("_pulse")) return;
      node.scratch("_pulse", true);
      const loop = () => {
        if (!node.inside()) {
          node.removeScratch("_pulse");
          return;
        }
        node.animate(
          { style: { "background-opacity": 0.4 } },
          {
            duration: 900,
            complete: () => {
              if (!node.inside()) {
                node.removeScratch("_pulse");
                return;
              }
              node.animate(
                { style: { "background-opacity": 0.9 } },
                { duration: 900, complete: loop },
              );
            },
          },
        );
      };
      loop();
    });
  }, []);

  const updateGraph = useCallback(() => {
    const cy = cyRef.current;
    if (!cy || !project) return;
    const elements = buildElements(project);

    // Cheap fingerprint: polling re-delivers unchanged data every few seconds, and the
    // whole rebuild (label metrics, style writes) is wasted work in that case.
    const signature = elements
      .map(
        (element) =>
          `${element.data.id}|${element.data.source ?? ""}|${element.data.nodeType ?? ""}|${
            element.data.status ?? ""
          }|${element.data.label ?? ""}`,
      )
      .join("\n");
    if (signature === signatureRef.current) return;
    signatureRef.current = signature;

    const wantedNodes = new Set(
      elements.filter((element) => "source" in element.data === false).map((element) => element.data.id),
    );
    const wantedEdges = new Set(
      elements
        .filter((element) => "source" in element.data)
        .map((element) => element.data.id as string),
    );
    const previous = snapshotPositions();
    const intentsById = new Map(project.intents.map((intent) => [intent.id, intent]));
    const intentByTarget = new Map(
      project.intents
        .filter((intent) => intent.to !== null)
        .map((intent) => [intent.to as string, intent]),
    );
    let structural = false;
    let added = 0;

    cy.nodes().forEach((node) => {
      if (!wantedNodes.has(node.id())) {
        node.remove();
        structural = true;
      }
    });
    cy.edges().forEach((edge) => {
      if (!wantedEdges.has(edge.id())) {
        edge.remove();
        structural = true;
      }
    });

    for (const element of elements) {
      const id = element.data.id as string;
      const existing = cy.getElementById(id);
      if (existing.length === 0) {
        let withPosition: ElementDefinition = element;
        if (!("source" in element.data)) {
          const nodeData = element.data;
          if (nodeData.intentId) {
            const intent = intentsById.get(nodeData.intentId as string);
            const position =
              previous.get(id) ?? (intent ? anchorFromIds(intent.from, previous, 32) : null);
            if (position) withPosition = { ...element, position };
          } else {
            const producing = intentByTarget.get(id);
            const placeholder = producing ? previous.get(`_ph_${producing.id}`) : undefined;
            const position =
              placeholder ?? (producing ? anchorFromIds(producing.from, previous, 44) : null);
            if (position) withPosition = { ...element, position };
          }
        }
        const created = cy.add(withPosition);
        added += 1;
        created.addClass("fresh");
        const fadeFrame = window.requestAnimationFrame(() => {
          fadeFrames.current.delete(fadeFrame);
          if (!created.inside()) return;
          created.animate(
            { style: { opacity: 1 } },
            {
              duration: 450,
              complete: () => {
                if (created.inside()) created.removeStyle("opacity");
              },
            },
          );
          created.removeClass("fresh");
        });
        fadeFrames.current.add(fadeFrame);
        structural = true;
      } else if (
        existing.data("nodeType") !== element.data.nodeType ||
        existing.data("label") !== element.data.label
      ) {
        existing.data(element.data);
        structural = true;
      } else if (existing.data("status") !== element.data.status) {
        // Status-only change (claimed / concluded): repaint without a relayout.
        existing.data(element.data);
      }
    }

    // Relayout only when the graph shape changed; status or label updates keep positions.
    if (structural) cy.layout(layoutOpts(layoutRef.current)).run();
    if (added > 0) pulseActive();
    positionsKey.current = project;
  }, [anchorFromIds, project, snapshotPositions, pulseActive]);

  useEffect(() => {
    void positionsKey.current;
    updateGraph();
  }, [project, updateGraph]);

  useEffect(() => {
    const cy = cyRef.current;
    if (!cy || !project) return;
    if (!selectedNode) {
      cy.elements().removeClass("highlight focus faded");
      syncFactSelections(cy, selectedFacts);
      return;
    }
    if (selectedNode.type === "group") {
      highlightGroup(cy, selectedNode.id, selectedFacts);
      return;
    }
    if (selectedNode.type === "intent") {
      highlightIntent(cy, project, selectedNode.id, selectedFacts);
      return;
    }
    if (selectedFacts.length > 1 && selectedFacts.includes(selectedNode.id)) {
      highlightMultiFact(cy, selectedFacts);
      return;
    }
    highlightFact(cy, project, selectedNode.id, selectedFacts);
  }, [selectedNode, selectedFacts, project]);

  useEffect(() => {
    const cy = cyRef.current;
    if (!cy) return;
    if (cy.nodes().length > 0) cy.layout(layoutOpts(layoutMode)).run();
  }, [layoutMode]);

  const fit = useCallback(() => {
    cyRef.current?.fit(undefined, 50);
  }, []);

  const centerOnFact = useCallback((factId: string) => {
    const cy = cyRef.current;
    if (!cy) return;
    const node = cy.getElementById(factId);
    if (node.length > 0) cy.animate({ center: { eles: node }, duration: 220 });
  }, []);

  const centerOnIntent = useCallback((intentId: string) => {
    const cy = cyRef.current;
    if (!cy) return;
    const edges = cy.edges(`[intentId="${intentId}"]`);
    if (edges.length === 0) return;
    cy.animate({ center: { eles: edges.add(edges.sources()).add(edges.targets()) }, duration: 220 });
  }, []);

  return { fit, centerOnFact, centerOnIntent, updateGraph };
}

export type { ElementLayoutMode };
