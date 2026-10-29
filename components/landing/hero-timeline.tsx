"use client";

import { useEffect, useState } from "react";
import { Timeline } from "@/components/timeline/timeline";
import { TimelineLegend } from "@/components/timeline/timeline-legend";
import { Command } from "@/components/ui/panel";
import { VerdictBadge } from "@/components/ui/verdict-glyph";
import { ApiError, getRecordedRun, getRunIndex, runExperiment } from "@/lib/api";
import { formatMs } from "@/lib/format";
import type { ExperimentResult, Spec, SpecInput } from "@/lib/types";

/** Shown when no recorded run exists: isolate a node mid-run, then heal. */
const HERO_SPEC: SpecInput = {
  seed: 42,
  target: "target-c",
  durationMs: 2400,
  clients: 4,
  keys: 2,
  readFrom: "any",
  faults: [
    { atMs: 700, action: "isolate", args: { node: "n1" } },
    { atMs: 1600, action: "heal", args: {} },
  ],
};

type State =
  | { status: "loading" }
  | { status: "ready"; result: ExperimentResult; runId: string | null }
  | { status: "offline" }
  | { status: "error"; message: string };

async function heroSpec(signal: AbortSignal): Promise<{ spec: SpecInput | Spec; runId: string | null }> {
  const index = await getRunIndex(signal).catch(() => []);
  const entry = index.find((r) => r.found) ?? index[0];
  if (!entry) return { spec: HERO_SPEC, runId: null };
  const run = await getRecordedRun(entry.id, signal).catch(() => null);
  const spec = run?.finding?.spec ?? run?.experiments.at(-1);
  return spec ? { spec, runId: entry.id } : { spec: HERO_SPEC, runId: null };
}

function Frame({ meta, children }: { meta: React.ReactNode; children: React.ReactNode }) {
  return (
    <figure className="rounded border border-border bg-bg">
      <figcaption className="flex h-9 items-center gap-3 border-b border-border px-3">
        <span className="label">History</span>
        {meta}
      </figcaption>
      <div className="p-3 sm:p-4">{children}</div>
    </figure>
  );
}

function Placeholder({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative flex min-h-48 flex-col justify-center gap-3">
      <div aria-hidden="true" className="absolute inset-0 flex flex-col justify-around">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="border-t border-border" />
        ))}
      </div>
      <div className="relative max-w-md bg-bg py-2 pr-4 text-[13px] text-muted">{children}</div>
    </div>
  );
}

export function HeroTimeline() {
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    const ac = new AbortController();
    heroSpec(ac.signal)
      .then(({ spec, runId }) => runExperiment(spec, ac.signal).then((result) => setState({ status: "ready", result, runId })))
      .catch((err: unknown) => {
        if (ac.signal.aborted) return;
        if (err instanceof ApiError && err.offline) setState({ status: "offline" });
        else setState({ status: "error", message: err instanceof Error ? err.message : "Could not render the history." });
      });
    return () => ac.abort();
  }, []);

  if (state.status !== "ready") {
    return (
      <Frame meta={<span className="ml-auto font-mono text-2xs text-subtle">/api/experiment</span>}>
        <Placeholder>
          {state.status === "loading" ? (
            <p className="label animate-pulse" role="status">Running a deterministic experiment</p>
          ) : state.status === "offline" ? (
            <div className="flex flex-col gap-2">
              <p className="text-fg">The simulator is offline.</p>
              <p>Start the Go backend to render a live operation history here.</p>
              <Command>make dev</Command>
            </div>
          ) : (
            <p>{state.message}</p>
          )}
        </Placeholder>
      </Frame>
    );
  }

  const { result, runId } = state;
  return (
    <Frame
      meta={
        <>
          <span className="truncate font-mono text-2xs text-subtle">
            {result.spec.target} · seed {result.spec.seed} · {result.history.length} ops · {formatMs(result.spec.durationMs)}
            {runId ? ` · from run ${runId}` : ""}
          </span>
          <span className="ml-auto shrink-0">
            <VerdictBadge verdict={result.check.verdict} />
          </span>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Timeline result={result} interactive={false} />
        {result.check.explanation ? (
          <p className="border-l-2 border-fg pl-3 text-[13px] leading-5">{result.check.explanation}</p>
        ) : null}
        <TimelineLegend />
      </div>
    </Frame>
  );
}
