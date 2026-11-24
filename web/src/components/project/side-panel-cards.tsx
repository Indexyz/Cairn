import { useTranslation } from "react-i18next";
import { MousePointerClick } from "lucide-react";

import type { SidePanelGroup } from "@/components/project/project-side-panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Separator } from "@/components/ui/separator";
import type { UseFormatResult } from "@/lib/format";
import { isBootstrapIntent } from "@/lib/timeline";
import type { Intent, ProjectDetail } from "@/lib/types";

export function GroupCard({
  group,
  format,
  onSelectIntent,
}: {
  group: SidePanelGroup;
  format: UseFormatResult;
  onSelectIntent: (intentId: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <article className="rounded-xl border border-warning/25 bg-warning/6 p-3 dark:bg-warning/10">
      <div className="mb-2 flex items-center gap-2">
        <Badge variant="warning" size="sm" className="font-mono">
          {t("project.group.title")}
        </Badge>
        <Badge variant="secondary" size="sm">
          {t("project.group.count", { count: group.intents.length })}
        </Badge>
      </div>
      <p className="truncate font-mono text-xs text-foreground">{group.worker}</p>
      <p className="mt-1 text-xs text-muted-foreground">{t("project.group.hint")}</p>
      <ul className="mt-2.5 space-y-1.5">
        {[...group.intents]
          .sort((a, b) => b.created_at.localeCompare(a.created_at))
          .map((member) => (
            <li key={member.id}>
              <button
                type="button"
                onClick={() => onSelectIntent(member.id)}
                className="w-full rounded-lg border bg-card px-2.5 py-2 text-left shadow-xs/5 transition-colors hover:bg-accent/50"
              >
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[11px] text-muted-foreground">{member.id}</span>
                  <Badge
                    variant={member.to ? "success" : member.worker ? "warning" : "secondary"}
                    size="sm"
                  >
                    {member.to
                      ? t("project.group.concluded")
                      : member.worker
                        ? t("project.group.running")
                        : t("project.group.unclaimed")}
                  </Badge>
                </div>
                <p className="mt-1 line-clamp-2 text-xs break-words text-foreground">
                  {member.description}
                </p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  {format.time(member.created_at)}
                </p>
              </button>
            </li>
          ))}
      </ul>
    </article>
  );
}

export function EmptySelection() {
  const { t } = useTranslation();
  return (
    <Empty className="rounded-xl border border-dashed px-4 py-10">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <MousePointerClick />
        </EmptyMedia>
        <EmptyTitle className="text-sm">{t("project.empty.selection")}</EmptyTitle>
        <EmptyDescription className="text-xs">{t("project.empty.multiSelect")}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  );
}

export function MultiFactList({
  project,
  selectedFacts,
  onToggleFact,
}: {
  project: ProjectDetail;
  selectedFacts: string[];
  onToggleFact: (factId: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2">
      {selectedFacts.map((factId) => {
        const fact = project.facts.find((item) => item.id === factId);
        if (!fact) return null;
        return (
          <article key={factId} className="rounded-xl border bg-card p-3 shadow-xs/5">
            <div className="mb-2 flex items-center justify-between gap-2">
              <Badge variant="secondary" size="sm" className="font-mono">
                {fact.id}
              </Badge>
              <Button variant="link" size="xs" onClick={() => onToggleFact(fact.id)}>
                {t("common.remove")}
              </Button>
            </div>
            <p className="text-sm break-words whitespace-pre-wrap text-foreground">
              {fact.description}
            </p>
          </article>
        );
      })}
    </div>
  );
}

export function FactCard({
  fact,
  producingIntent,
}: {
  fact: ProjectDetail["facts"][number];
  producingIntent: Intent | undefined;
}) {
  const { t } = useTranslation();
  return (
    <article className="rounded-xl border bg-card p-3.5 shadow-xs/5">
      <Badge variant="secondary" size="sm" className="font-mono">
        {fact.id}
      </Badge>
      <p className="mt-2.5 text-sm break-words whitespace-pre-wrap text-foreground">
        {fact.description}
      </p>
      {producingIntent && (
        <>
          <Separator className="my-3" />
          <dl className="grid grid-cols-2 gap-y-1.5 text-xs">
            <dt className="text-muted-foreground">{t("project.fact.producedBy")}</dt>
            <dd className="text-right font-mono text-foreground">{producingIntent.id}</dd>
            <dt className="text-muted-foreground">{t("project.fact.from")}</dt>
            <dd className="text-right font-mono text-foreground">
              {producingIntent.from.join(", ")}
            </dd>
            <dt className="text-muted-foreground">{t("project.fact.worker")}</dt>
            <dd className="text-right text-foreground">{producingIntent.worker ?? "—"}</dd>
          </dl>
        </>
      )}
    </article>
  );
}

export function IntentCard({ intent, format }: { intent: Intent; format: UseFormatResult }) {
  const { t } = useTranslation();
  const statusVariant = intent.to ? "success" : intent.worker ? "warning" : "secondary";
  return (
    <article className="rounded-xl border bg-card p-3.5 shadow-xs/5">
      <div className="mb-2.5 flex items-center gap-2">
        <Badge variant="secondary" size="sm" className="font-mono">
          {intent.id}
        </Badge>
        <Badge variant={statusVariant} size="sm">
          {intent.to
            ? t("project.intent.concluded")
            : isBootstrapIntent(intent)
              ? intent.worker
                ? t("project.intent.bootstrapRunning")
                : t("project.intent.bootstrapPending")
              : intent.worker
                ? t("project.intent.inProgress")
                : t("project.intent.unclaimed")}
        </Badge>
      </div>
      <p className="text-sm break-words whitespace-pre-wrap text-foreground">
        {intent.description}
      </p>
      <Separator className="my-3" />
      <dl className="grid grid-cols-2 gap-y-1.5 text-xs">
        <dt className="text-muted-foreground">{t("project.intent.from")}</dt>
        <dd className="text-right font-mono text-foreground">{intent.from.join(", ")}</dd>
        <dt className="text-muted-foreground">{t("project.intent.to")}</dt>
        <dd className="text-right font-mono text-foreground">{intent.to ?? "—"}</dd>
        <dt className="text-muted-foreground">{t("project.intent.worker")}</dt>
        <dd className="text-right text-foreground">{intent.worker ?? "—"}</dd>
        <dt className="text-muted-foreground">{t("project.intent.created")}</dt>
        <dd className="text-right tabular-nums text-foreground">
          {format.time(intent.created_at)}
        </dd>
      </dl>
    </article>
  );
}
