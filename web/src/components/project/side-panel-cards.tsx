import { useTranslation } from "react-i18next";

import type { UseFormatResult } from "@/lib/format";
import type { Intent, ProjectDetail } from "@/lib/types";
import { isBootstrapIntent } from "@/lib/timeline";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { SidePanelGroup } from "@/components/project/project-side-panel";

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
                <article className="rounded-xl border border-amber-200 bg-amber-50/40 p-3">
                  <div className="mb-2 flex items-center gap-2">
                    <span className="rounded bg-amber-100 px-1.5 py-0.5 font-mono text-[10px] text-amber-700">
                      {t("project.group.title")}
                    </span>
                    <Badge variant="secondary">
                      {t("project.group.count", { count: group.intents.length })}
                    </Badge>
                  </div>
                  <p className="truncate font-mono text-xs text-muted-foreground">{group.worker}</p>
                  <p className="mt-1 text-[11px] text-muted-foreground">{t("project.group.hint")}</p>
                  <ul className="mt-2 space-y-1">
                    {[...group.intents]
                      .sort((a, b) => b.created_at.localeCompare(a.created_at))
                      .map((member) => (
                        <li key={member.id}>
                          <button
                            type="button"
                            onClick={() => onSelectIntent(member.id)}
                            className="w-full rounded-lg border bg-background px-2 py-1.5 text-left hover:bg-muted/50"
                          >
                            <div className="flex items-center gap-2">
                              <span className="font-mono text-[10px]">{member.id}</span>
                              <Badge variant="secondary">
                                {member.to
                                  ? t("project.group.concluded")
                                  : member.worker
                                    ? t("project.group.running")
                                    : t("project.group.unclaimed")}
                              </Badge>
                            </div>
                            <p className="mt-0.5 line-clamp-2 text-xs break-words">{member.description}</p>
                            <p className="text-[10px] text-muted-foreground">{format.time(member.created_at)}</p>
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
    <div className="rounded-xl border border-dashed px-4 py-10 text-center">
      <p className="text-sm text-muted-foreground">{t("project.empty.selection")}</p>
      <p className="text-xs text-muted-foreground/70">{t("project.empty.multiSelect")}</p>
    </div>
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
                      <article key={factId} className="rounded-xl border p-3">
                        <div className="mb-2 flex items-center justify-between">
                          <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">{fact.id}</span>
                          <Button variant="link" size="sm" onClick={() => onToggleFact(fact.id)}>
                            {t("common.remove")}
                          </Button>
                        </div>
                        <p className="text-sm break-words whitespace-pre-wrap">{fact.description}</p>
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
                <article className="rounded-xl border p-3">
                  <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">
                    {fact.id}
                  </span>
                  <p className="mt-2 text-sm break-words whitespace-pre-wrap">
                    {fact.description}
                  </p>
                  {producingIntent && (
                    <dl className="mt-3 grid grid-cols-2 gap-1 border-t pt-3 text-xs">
                      <dt className="text-muted-foreground">{t("project.fact.producedBy")}</dt>
                      <dd className="text-right font-mono">{producingIntent.id}</dd>
                      <dt className="text-muted-foreground">{t("project.fact.from")}</dt>
                      <dd className="text-right font-mono">{producingIntent.from.join(", ")}</dd>
                      <dt className="text-muted-foreground">{t("project.fact.worker")}</dt>
                      <dd className="text-right">{producingIntent.worker ?? "—"}</dd>
                    </dl>
                  )}
                </article>
  );
}

export function IntentCard({ intent, format }: { intent: Intent; format: UseFormatResult }) {
  const { t } = useTranslation();
  return (
                <article className="rounded-xl border p-3">
                  <div className="mb-2 flex items-center gap-2">
                    <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px]">
                      {intent.id}
                    </span>
                    <Badge variant="secondary">
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
                  <p className="text-sm break-words whitespace-pre-wrap">
                    {intent.description}
                  </p>
                  <dl className="mt-3 grid grid-cols-2 gap-1 border-t pt-3 text-xs">
                    <dt className="text-muted-foreground">{t("project.intent.from")}</dt>
                    <dd className="text-right font-mono">{intent.from.join(", ")}</dd>
                    <dt className="text-muted-foreground">{t("project.intent.to")}</dt>
                    <dd className="text-right font-mono">{intent.to ?? "—"}</dd>
                    <dt className="text-muted-foreground">{t("project.intent.worker")}</dt>
                    <dd className="text-right">{intent.worker ?? "—"}</dd>
                    <dt className="text-muted-foreground">{t("project.intent.created")}</dt>
                    <dd className="text-right">{format.time(intent.created_at)}</dd>
                  </dl>
                </article>
  );
}
