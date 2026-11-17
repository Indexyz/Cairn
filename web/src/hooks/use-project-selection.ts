import { useMemo, useState, type RefObject } from "react";

import { collectIntentGroups, type IntentGroup } from "@/lib/graph";
import {
  timelineEventIsInteractive,
  timelineEventTriggersGraphFocus,
} from "@/lib/timeline";
import type { ProjectDetail, Selection, TimelineEvent } from "@/lib/types";

interface Options {
  project: ProjectDetail | null;
  timelineSource: ProjectDetail | null;
  timeline: TimelineEvent[];
  sideTab: "detail" | "hints" | "log";
  logRef: RefObject<HTMLDivElement | null>;
  duration: (ms: number) => string;
}

export function useProjectSelection({
  project,
  timelineSource,
  timeline,
  sideTab,
  logRef,
  duration,
}: Options) {
  const [selectedNode, setSelectedNode] = useState<Selection | null>(null);
  const [selectedFacts, setSelectedFacts] = useState<string[]>([]);
  const [selectedTimelineEntryId, setSelectedTimelineEntryId] = useState<string | null>(null);

  const selectedFactRecord = useMemo(() => {
    if (!project || selectedNode?.type !== "fact") return null;
    return project.facts.find((fact) => fact.id === selectedNode.id) ?? null;
  }, [project, selectedNode]);

  const selectedIntentRecord = useMemo(() => {
    if (!project || selectedNode?.type !== "intent") return null;
    return project.intents.find((intent) => intent.id === selectedNode.id) ?? null;
  }, [project, selectedNode]);

  const groups = useMemo(
    () => (timelineSource ? collectIntentGroups(timelineSource) : new Map<string, IntentGroup>()),
    [timelineSource],
  );

  const selectedGroup = useMemo(
    () => [...groups.values()].find((group) => group.nodeId === selectedNode?.id) ?? null,
    [groups, selectedNode],
  );

  function clearSelection() {
    setSelectedNode(null);
    setSelectedFacts([]);
    setSelectedTimelineEntryId(null);
  }

  function selectGroup(nodeId: string) {
    setSelectedFacts([]);
    setSelectedTimelineEntryId(null);
    setSelectedNode({ type: "group", id: nodeId });
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
    const next = selectedFacts.includes(factId)
      ? selectedFacts.filter((id) => id !== factId)
      : [...selectedFacts, factId];
    setSelectedFacts(next);
    setSelectedNode(next.length > 0 ? { type: "fact", id: next[next.length - 1] } : null);
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

  function openTimelineEntry(
    entry: TimelineEvent,
    centers: { centerOnFact: (factId: string) => void; centerOnIntent: (intentId: string) => void },
  ) {
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
      centers.centerOnIntent(entry.targetId);
      return;
    }
    setSelectedFacts([entry.targetId]);
    selectFact(entry.targetId);
    setSelectedTimelineEntryId(entry.id);
    centers.centerOnFact(entry.targetId);
  }

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
      timeLabel: `${duration(elapsed)} / ${duration(totalDuration)} · ${timePercent}%`,
    };
  }, [duration, selectedNode, selectedTimelineEntryId, timeline]);

  return {
    selectedNode,
    selectedFacts,
    setSelectedNode,
    setSelectedFacts,
    setSelectedTimelineEntryId,
    selectedTimelineEntryId,
    selectedFactRecord,
    selectedIntentRecord,
    selectedGroup,
    selectedTimelineSummary,
    clearSelection,
    selectGroup,
    selectIntent,
    selectFact,
    toggleFact,
    scrollToEntry,
    scrollToSelection,
    openTimelineEntry,
  };
}
