import type { RefObject } from "react";
import { useTranslation } from "react-i18next";

import {
  EmptySelection,
  FactCard,
  GroupCard,
  IntentCard,
  MultiFactList,
} from "@/components/project/side-panel-cards";

import type { UseFormatResult } from "@/lib/format";
import type { Intent, ProjectDetail, Selection, TimelineEvent } from "@/lib/types";
import { cn } from "@/lib/utils";
import { TimelineRow } from "@/components/project/timeline-row";

export type SideTab = "detail" | "hints" | "log";

export interface SidePanelGroup {
  worker: string;
  intents: Intent[];
}

export interface SidePanelSummary {
  sequencePercent: number;
  sequenceLabel: string;
  timePercent: number;
  timeLabel: string;
}

export interface ProjectSidePanelProps {
  sideTab: SideTab;
  onTab: (tab: SideTab) => void;
  width: number;
  project: ProjectDetail;
  selectedNode: Selection | null;
  selectedFacts: string[];
  selectedFactRecord: ProjectDetail["facts"][number] | null;
  selectedFactIntent: Intent | null;
  selectedIntentRecord: Intent | null;
  selectedGroup: SidePanelGroup | null;
  producingIntent: Intent | null;
  selectedTimelineSummary: SidePanelSummary | null;
  timeline: TimelineEvent[];
  selectedTimelineEntryId: string | null;
  onEntryOpen: (entry: TimelineEvent) => void;
  onHintOpen: (hintId: string) => void;
  onSelectIntent: (intentId: string) => void;
  onToggleFact: (factId: string) => void;
  logRef: RefObject<HTMLDivElement | null>;
  format: UseFormatResult;
}

export function ProjectSidePanel({
  sideTab,
  onTab,
  width,
  project,
  selectedTimelineSummary,
  timeline,
  selectedTimelineEntryId,
  onEntryOpen,
  onHintOpen,
  logRef,
  format,
  ...props
}: ProjectSidePanelProps) {
  const { t } = useTranslation();

  return (
    <aside
      className="flex shrink-0 flex-col overflow-hidden border-l bg-background"
      style={{ width }}
    >
      <div className="flex shrink-0 border-b">
        {(["detail", "hints", "log"] as const).map((tab) => (
          <button
            key={tab}
            type="button"
            onClick={() => onTab(tab)}
            className={cn(
              "flex-1 px-3 py-2.5 text-xs font-medium capitalize transition-colors",
              sideTab === tab
                ? "border-b-2 border-primary text-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t(`project.tabs.${tab}`)}
            {tab === "hints" && <span className="ml-1 opacity-60">{project.hints.length}</span>}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto p-3">
        {sideTab === "detail" && <DetailTab {...props} project={project} format={format} />}

        {sideTab === "hints" && (
          <div className="space-y-2">
            {project.hints.length === 0 && (
              <p className="py-10 text-center text-sm text-muted-foreground">{t("project.empty.hints")}</p>
            )}
            {project.hints.map((hint) => (
              <button
                key={hint.id}
                type="button"
                onClick={() => onHintOpen(hint.id)}
                className="w-full rounded-xl border bg-card p-3 text-left hover:bg-muted/40"
              >
                <div className="mb-2 flex items-center justify-between text-[11px] text-muted-foreground">
                  <span className="truncate font-medium">{hint.creator}</span>
                  <span>{format.time(hint.created_at)}</span>
                </div>
                <p className="text-sm break-words whitespace-pre-wrap">{hint.content}</p>
              </button>
            ))}
          </div>
        )}

        {sideTab === "log" && (
          <div ref={logRef} className="space-y-3">
            {selectedTimelineSummary && (
              <div className="sticky top-0 z-10 rounded-xl border bg-background/95 p-3 shadow-sm">
                {(["sequence", "time"] as const).map((kind) => {
                  const percent =
                    kind === "sequence"
                      ? selectedTimelineSummary.sequencePercent
                      : selectedTimelineSummary.timePercent;
                  const label =
                    kind === "sequence"
                      ? selectedTimelineSummary.sequenceLabel
                      : selectedTimelineSummary.timeLabel;
                  return (
                    <div key={kind} className="space-y-1 py-1">
                      <div className="flex justify-between text-[11px] text-muted-foreground">
                        <span className="capitalize">{t(`project.timeline.${kind}`)}</span>
                        <span className="font-mono">{label}</span>
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                        <div className="h-full rounded-full bg-primary/70" style={{ width: `${percent}%` }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
            {timeline.length === 0 && (
              <p className="py-10 text-center text-sm text-muted-foreground">{t("project.empty.activity")}</p>
            )}
            {timeline.map((entry) => (
              <TimelineRow
                key={entry.id}
                entry={entry}
                selected={selectedTimelineEntryId === entry.id}
                onOpen={onEntryOpen}
                time={format.time(entry.timestamp)}
                dateLabel={format.timelineDate(entry.timestamp)}
              />
            ))}
          </div>
        )}
      </div>
    </aside>
  );
}

type DetailTabProps = Pick<
  ProjectSidePanelProps,
  | "project"
  | "selectedNode"
  | "selectedFacts"
  | "selectedFactRecord"
  | "selectedFactIntent"
  | "selectedIntentRecord"
  | "selectedGroup"
  | "producingIntent"
  | "onSelectIntent"
  | "onToggleFact"
  | "format"
>;

function DetailTab({
  project,
  selectedNode,
  selectedFacts,
  selectedFactRecord,
  selectedFactIntent,
  selectedIntentRecord,
  selectedGroup,
  producingIntent,
  onSelectIntent,
  onToggleFact,
  format,
}: DetailTabProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-3">
            {selectedNode?.type === "group" && selectedGroup && (
              <GroupCard group={selectedGroup} format={format} onSelectIntent={onSelectIntent} />
            )}
            {!selectedNode && <EmptySelection />}
            {selectedNode?.type === "fact" && selectedFacts.length > 1 && (
              <MultiFactList
                project={project}
                selectedFacts={selectedFacts}
                onToggleFact={onToggleFact}
              />
            )}
            {selectedNode?.type === "fact" && selectedFacts.length <= 1 && selectedFactRecord && (
              <FactCard fact={selectedFactRecord} producingIntent={selectedFactIntent ?? undefined} />
            )}
            {selectedNode?.type === "intent" && selectedIntentRecord && (
              <IntentCard intent={selectedIntentRecord} format={format} />
            )}
            {producingIntent && selectedNode?.type === "fact" && (
              <p className="text-xs text-muted-foreground">
                {t("project.fact.producedByLine", {
                  intent: producingIntent.id,
                  from: producingIntent.from.join(", "),
                })}
              </p>
            )}
    </div>
  );
}
