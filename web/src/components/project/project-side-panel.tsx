import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Activity, Lightbulb } from "lucide-react";

import {
  EmptySelection,
  FactCard,
  GroupCard,
  IntentCard,
  MultiFactList,
} from "@/components/project/side-panel-cards";
import { TimelineRow } from "@/components/project/timeline-row";
import { Badge } from "@/components/ui/badge";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Tabs, TabsList, TabsPanel, TabsTab } from "@/components/ui/tabs";
import type { UseFormatResult } from "@/lib/format";
import type { Intent, ProjectDetail, Selection, TimelineEvent } from "@/lib/types";

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
      <Tabs
        value={sideTab}
        onValueChange={(value) => onTab(value as SideTab)}
        className="flex min-h-0 flex-1 flex-col gap-0"
      >
        <div className="shrink-0 border-b px-2">
          <TabsList variant="underline" size="sm" className="w-full">
            <TabsTab value="detail">{t("project.tabs.detail")}</TabsTab>
            <TabsTab value="hints">
              <span>{t("project.tabs.hints")}</span>
              {project.hints.length > 0 && (
                <Badge variant="secondary" size="sm" className="ml-1 tabular-nums">
                  {project.hints.length}
                </Badge>
              )}
            </TabsTab>
            <TabsTab value="log">{t("project.tabs.log")}</TabsTab>
          </TabsList>
        </div>

        <TabsPanel value="detail" className="flex-1 overflow-y-auto p-3">
          <DetailTab {...props} project={project} format={format} />
        </TabsPanel>

        <TabsPanel value="hints" className="flex-1 overflow-y-auto p-3">
          <div className="space-y-2">
            {project.hints.length === 0 ? (
              <Empty className="py-10">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <Lightbulb />
                  </EmptyMedia>
                  <EmptyTitle className="text-sm">{t("project.empty.hints")}</EmptyTitle>
                </EmptyHeader>
              </Empty>
            ) : (
              project.hints.map((hint) => (
                <button
                  key={hint.id}
                  type="button"
                  onClick={() => onHintOpen(hint.id)}
                  className="w-full rounded-xl border bg-card p-3 text-left shadow-xs/5 transition-colors hover:bg-accent/50"
                >
                  <div className="mb-1.5 flex items-center justify-between text-xs text-muted-foreground">
                    <span className="truncate font-medium text-foreground">{hint.creator}</span>
                    <span className="tabular-nums">{format.time(hint.created_at)}</span>
                  </div>
                  <p className="text-sm break-words whitespace-pre-wrap text-foreground">
                    {hint.content}
                  </p>
                </button>
              ))
            )}
          </div>
        </TabsPanel>

        <TabsPanel value="log" className="flex-1 overflow-y-auto p-3">
          <div ref={logRef} className="space-y-3">
            {selectedTimelineSummary && (
              <div className="sticky top-0 z-10 rounded-xl border bg-popover/95 p-3 shadow-sm/5 backdrop-blur-sm">
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
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span className="capitalize">{t(`project.timeline.${kind}`)}</span>
                        <span className="font-mono">{label}</span>
                      </div>
                      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                        <div
                          className="h-full rounded-full bg-primary transition-[width] duration-200"
                          style={{ width: `${percent}%` }}
                        />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
            {timeline.length === 0 ? (
              <Empty className="py-10">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <Activity />
                  </EmptyMedia>
                  <EmptyTitle className="text-sm">{t("project.empty.activity")}</EmptyTitle>
                </EmptyHeader>
              </Empty>
            ) : (
              timeline.map((entry) => (
                <TimelineRow
                  key={entry.id}
                  entry={entry}
                  selected={selectedTimelineEntryId === entry.id}
                  onOpen={onEntryOpen}
                  time={format.time(entry.timestamp)}
                  dateLabel={format.timelineDate(entry.timestamp)}
                />
              ))
            )}
          </div>
        </TabsPanel>
      </Tabs>
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
