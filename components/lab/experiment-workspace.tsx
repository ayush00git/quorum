"use client";

import { useEffect, useMemo, useState } from "react";
import { ClusterLegend, ClusterView } from "@/components/cluster/cluster-view";
import { Timeline } from "@/components/timeline/timeline";
import { TimeScrubber } from "@/components/timeline/timeline-toolbar";
import { EmptyState, Panel } from "@/components/ui/panel";
import { VerdictBadge } from "@/components/ui/verdict-glyph";
import { ApiError } from "@/lib/api";
import { clusterAt } from "@/lib/cluster";
import { loadExperiment } from "@/lib/experiment-cache";
import type { ExperimentResult, ExperimentSummary } from "@/lib/types";
import { ExperimentList } from "./experiment-list";

export type WorkspaceTab = "agent" | "timeline" | "cluster";

type Loaded = { id: string; result: ExperimentResult } | { id: string; error: string };

function errorText(err: unknown): string {
  if (err instanceof ApiError) return err.offline ? "The backend is offline, so this experiment cannot be re-run." : err.message;
  return "Could not load the experiment.";
}

export function ExperimentWorkspace({
  experiments,
  selectedId,
  onSelect,
  tab,
  empty,
}: {
  experiments: readonly ExperimentSummary[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  tab: WorkspaceTab;
  empty: React.ReactNode;
}) {
  const selected = experiments.find((e) => e.id === selectedId) ?? null;
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [cursor, setCursor] = useState<{ id: string; ms: number } | null>(null);

  useEffect(() => {
    if (!selected) return;
    let live = true;
    loadExperiment(selected.id, selected.spec).then(
      (result) => live && setLoaded({ id: selected.id, result }),
      (err: unknown) => live && setLoaded({ id: selected.id, error: errorText(err) }),
    );
    return () => {
      live = false;
    };
  }, [selected]);

  const current = loaded && selected && loaded.id === selected.id ? loaded : null;
  const result = current && "result" in current ? current.result : null;
  const cursorMs = result ? (cursor?.id === result.id ? cursor.ms : result.spec.durationMs) : 0;
  const setCursorMs = (ms: number) => result && setCursor({ id: result.id, ms });

  const cluster = useMemo(
    () => (result ? clusterAt(result.spec.nodes, result.events, result.spec.faults, cursorMs) : null),
    [result, cursorMs],
  );

  const show = (t: WorkspaceTab) => (tab === t ? "flex" : "hidden lg:flex");

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="grid shrink-0 border-b border-border xl:grid-cols-[minmax(280px,340px)_minmax(0,1fr)]">
        <Panel title="Cluster" className={`${show("cluster")} border-b xl:border-r xl:border-b-0`} bodyClassName="flex flex-col gap-3 p-3">
          {cluster && result ? (
            <>
              <ClusterView state={cluster} atMs={cursorMs} />
              <div className="lg:hidden">
                <TimeScrubber value={cursorMs} max={result.spec.durationMs} onChange={setCursorMs} />
              </div>
              <ClusterLegend />
            </>
          ) : (
            <EmptyState title="No cluster to show">Select an experiment to see the cluster at any moment.</EmptyState>
          )}
        </Panel>
        <Panel
          title="Experiments"
          actions={<span className="font-mono text-2xs tabular-nums text-subtle">{experiments.length}</span>}
          className={show("timeline")}
          bodyClassName="max-h-64 overflow-y-auto xl:max-h-[22rem]"
        >
          {experiments.length > 0 ? (
            <ExperimentList experiments={experiments} selectedId={selectedId} onSelect={onSelect} />
          ) : (
            empty
          )}
        </Panel>
      </div>

      <Panel
        title={selected ? <>History <span className="normal-case tracking-normal text-subtle">· {selected.id}</span></> : "History"}
        actions={result ? <VerdictBadge verdict={result.check.verdict} /> : null}
        className={`${show("timeline")} flex-1`}
        bodyClassName="flex flex-col gap-3 p-3"
      >
        {!selected ? (
          <EmptyState title="No experiment selected">The timeline fills in as soon as the agent runs its first experiment.</EmptyState>
        ) : current && "error" in current ? (
          <EmptyState title="Experiment unavailable">{current.error}</EmptyState>
        ) : !result ? (
          <p className="label animate-pulse p-1" role="status">Running experiment</p>
        ) : (
          <>
            {result.check.explanation ? (
              <p className={`border-l-2 pl-3 text-[13px] leading-5 ${result.check.verdict === "violation" ? "border-fg text-fg" : "border-border-strong text-muted"}`}>
                {result.check.explanation}
              </p>
            ) : null}
            <dl className="flex flex-wrap gap-x-5 gap-y-1 font-mono text-2xs tabular-nums text-muted">
              {[
                ["ops", result.stats.ops],
                ["ok", result.stats.ok],
                ["fail", result.stats.fail],
                ["info", result.stats.info],
                ["msgs", result.stats.messagesSent],
                ["dropped", result.stats.messagesDropped],
                ["leaders", result.stats.leaders.length],
                ["seed", result.spec.seed],
              ].map(([k, v]) => (
                <div key={k} className="flex gap-1.5">
                  <dt className="uppercase tracking-wider text-subtle">{k}</dt>
                  <dd className="text-fg">{v}</dd>
                </div>
              ))}
            </dl>
            <Timeline key={result.id} result={result} cursorMs={cursorMs} onCursorChange={setCursorMs} />
          </>
        )}
      </Panel>
    </div>
  );
}
