"use client";

import { useEffect, useMemo, useState } from "react";
import { Command, EmptyState } from "@/components/ui/panel";
import { buildLog } from "@/lib/agent-log";
import { ApiError, getMeta } from "@/lib/api";
import { buildRecordedRun, downloadJson } from "@/lib/export";
import type { Meta } from "@/lib/types";
import { useAgentRun } from "@/lib/use-agent-run";
import { AgentLog } from "./agent-log";
import { ControlRail, RunStatus, type RunForm } from "./control-rail";
import { ExperimentWorkspace } from "./experiment-workspace";
import { FindingPanel } from "./finding-panel";
import { LabShell } from "./lab-shell";

export function LabApp() {
  const [meta, setMeta] = useState<Meta | null>(null);
  const [metaError, setMetaError] = useState<string | null>(null);
  const [form, setForm] = useState<Omit<RunForm, "target" | "maxSteps"> & Partial<RunForm>>({
    seed: 42,
    accessCode: "",
  });
  const [picked, setPicked] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const { state, start, stop, reset } = useAgentRun();

  useEffect(() => {
    const ac = new AbortController();
    getMeta(ac.signal).then(setMeta, (err: unknown) => {
      if (ac.signal.aborted) return;
      setMetaError(
        err instanceof ApiError && err.offline
          ? "Backend offline. Start the Go dev server with make dev."
          : err instanceof Error
            ? err.message
            : "Could not load the lab configuration.",
      );
    });
    return () => ac.abort();
  }, []);

  const resolved: RunForm = {
    ...form,
    target: form.target ?? meta?.targets[0]?.codename ?? "",
    maxSteps: form.maxSteps ?? meta?.maxSteps ?? 30,
  };

  const entries = useMemo(() => buildLog(state.steps), [state.steps]);
  const lastId = state.experiments.at(-1)?.id ?? null;
  const selectedId = picked ?? state.finding?.experimentId ?? lastId;

  const onExport = async () => {
    setExporting(true);
    try {
      const run = await buildRecordedRun(state, meta?.model ?? "");
      downloadJson(`${state.target}-${resolved.seed}.json`, run);
    } finally {
      setExporting(false);
    }
  };

  const summary = [resolved.target || "no target", state.status, state.steps.length ? `s${state.steps.length}` : null]
    .filter(Boolean)
    .join(" · ");

  return (
    <LabShell
      railSummary={summary}
      rail={
        <ControlRail
          meta={meta}
          metaError={metaError}
          form={resolved}
          onChange={(patch) => setForm((f) => ({ ...f, ...patch }))}
          state={state}
          onStart={() => {
            setPicked(null);
            void start({
              target: resolved.target,
              maxSteps: resolved.maxSteps,
              accessCode: resolved.accessCode || undefined,
            });
          }}
          onStop={stop}
          onReset={() => {
            setPicked(null);
            reset();
          }}
        />
      }
      workspace={(tab) => (
        <ExperimentWorkspace
          experiments={state.experiments}
          selectedId={selectedId}
          onSelect={setPicked}
          tab={tab}
          empty={<EmptyState title="No experiments yet">Each experiment Claude runs appears here with its verdict.</EmptyState>}
        />
      )}
      logTitle={
        <>
          <h2 className="label">Agent</h2>
          <span className="ml-auto font-mono text-2xs tabular-nums text-subtle">
            {state.steps.length > 0 ? `${state.steps.length} / ${state.maxSteps} steps` : ""}
          </span>
        </>
      }
      log={
        <AgentLog
          entries={entries}
          selectedId={selectedId}
          onSelectExperiment={setPicked}
          status={state.status === "idle" ? null : <RunStatus state={state} />}
          empty={
            <EmptyState title="The adversary is idle">
              <p>
                Claude forms a hypothesis, injects faults, reads the linearizability checker, and repeats until it
                finds a violation or runs out of budget.
              </p>
              {metaError ? (
                <>
                  <p className="mt-2">The Go backend is not reachable. From the repository root run:</p>
                  <Command>make dev</Command>
                </>
              ) : null}
            </EmptyState>
          }
          footer={
            state.status === "done" ? (
              <FindingPanel
                finding={state.finding}
                conclusion={state.conclusion}
                revealedBug={state.revealedBug}
                onSelectExperiment={setPicked}
                onExport={onExport}
                exporting={exporting}
              />
            ) : null
          }
        />
      }
    />
  );
}
