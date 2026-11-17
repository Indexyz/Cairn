import { useMemo } from "react";

import type { Intent, ProjectDetail, ProjectStatus, Selection } from "@/lib/types";

export function useProjectFlags(status: ProjectStatus | undefined, replayActive: boolean) {
  const isActive = status === "active" && !replayActive;
  // A blocked project still runs its outstanding intents and can be completed.
  const isRunnable = (status === "active" || status === "blocked") && !replayActive;
  const canWriteHints = !replayActive;

  return { isActive, isRunnable, canWriteHints };
}

interface DerivationsOptions {
  project: ProjectDetail | null;
  selectedNode: Selection | null;
  selectedFactRecord: ProjectDetail["facts"][number] | null;
  selectedIntentRecord: Intent | null;
  isRunnable: boolean;
  actorName: string;
}

export function useProjectDerivations({
  project,
  selectedNode,
  selectedFactRecord,
  selectedIntentRecord,
  isRunnable,
  actorName,
}: DerivationsOptions) {
  const selectedFactIntent = useMemo(
    () =>
      selectedFactRecord
        ? project?.intents.find((intent) => intent.to === selectedFactRecord.id)
        : undefined,
    [project, selectedFactRecord],
  );

  const producingIntent = useMemo(
    () => project?.intents.find((intent) => intent.to === (selectedNode?.id ?? "")),
    [project, selectedNode],
  );

  const openIntent = selectedIntentRecord && !selectedIntentRecord.to && isRunnable ? selectedIntentRecord : null;
  const actionableIntent = openIntent && (!openIntent.worker || openIntent.worker === actorName) ? openIntent : null;
  const releasableIntent = openIntent && openIntent.worker === actorName ? openIntent : null;

  return { selectedFactIntent, producingIntent, openIntent, actionableIntent, releasableIntent };
}
