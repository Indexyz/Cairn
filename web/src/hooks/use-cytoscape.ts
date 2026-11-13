import { useCallback, useEffect, useRef } from "react";
import cytoscape, { type Core, type ElementDefinition, type NodeSingular } from "cytoscape";
import dagre from "cytoscape-dagre";
import klay from "cytoscape-klay";
import elk from "cytoscape-elk";

import {
  buildElements,
  graphStyles,
  highlightFact,
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
  selectedNode: Selection | null;
  selectedFacts: string[];
  onNodeTap: (node: NodeSingular, shiftKey: boolean) => void;
  onEdgeTap: (intentId: string) => void;
  onBackgroundTap: () => void;
}

export function useCytoscape(options: Options) {
  const { containerRef, project, layoutMode, selectedNode, selectedFacts, resetKey } = options;
  const cyRef = useRef<Core | null>(null);
  const handlers = useRef(options);
  handlers.current = options;
  const layoutRef = useRef<LayoutMode>(layoutMode);
  layoutRef.current = layoutMode;

  const positionsKey = useRef<ProjectDetail | null>(null);
  // The container only exists once the project is loaded, so initialisation must re-run then.
  const hasProject = Boolean(project);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const instance = cytoscape({
      container,
      elements: [],
      style: graphStyles(),
      layout: layoutOpts(layoutRef.current),
      minZoom: 0.15,
      maxZoom: 3.5,
    });
    cyRef.current = instance;
    // Handy for debugging graph rendering from the console or a headless check.
    (window as unknown as { __cy?: Core }).__cy = instance;

    instance.on("tap", "node", (event) => {
      handlers.current.onNodeTap(event.target as NodeSingular, event.originalEvent.shiftKey);
    });
    instance.on("tap", "edge", (event) => {
      handlers.current.onEdgeTap(event.target.data("intentId"));
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

  const updateGraph = useCallback(() => {
    const cy = cyRef.current;
    if (!cy || !project) return;
    const elements = buildElements(project);
    const wantedNodes = new Set(
      elements.filter((element) => "source" in element.data === false).map((element) => element.data.id),
    );
    const wantedEdges = new Set(
      elements
        .filter((element) => "source" in element.data)
        .map((element) => element.data.id as string),
    );
    const previous = snapshotPositions();
    let changed = false;

    cy.nodes().forEach((node) => {
      if (!wantedNodes.has(node.id())) {
        node.remove();
        changed = true;
      }
    });
    cy.edges().forEach((edge) => {
      if (!wantedEdges.has(edge.id())) {
        edge.remove();
        changed = true;
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
            const intent = project.intents.find((item) => item.id === nodeData.intentId);
            const position =
              previous.get(id) ?? (intent ? anchorFromIds(intent.from, previous, 32) : null);
            if (position) withPosition = { ...element, position };
          } else {
            const producing = project.intents.find((intent) => intent.to === id);
            const placeholder = producing ? previous.get(`_ph_${producing.id}`) : undefined;
            const position =
              placeholder ?? (producing ? anchorFromIds(producing.from, previous, 44) : null);
            if (position) withPosition = { ...element, position };
          }
        }
        const added = cy.add(withPosition);
        added.addClass("fresh");
        window.setTimeout(() => {
          if (!added.inside()) return;
          added.animate(
            { style: { opacity: 1 } },
            {
              duration: 450,
              complete: () => {
                if (added.inside()) added.removeStyle("opacity");
              },
            },
          );
          added.removeClass("fresh");
        }, 30);
        changed = true;
      } else if (existing.data("nodeType") !== element.data.nodeType || existing.data("label") !== element.data.label) {
        existing.data(element.data);
        changed = true;
      } else if (existing.data("status") !== element.data.status) {
        existing.data(element.data);
      }
    }

    if (changed) cy.layout(layoutOpts(layoutRef.current)).run();
    positionsKey.current = project;
  }, [anchorFromIds, project, snapshotPositions]);

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
