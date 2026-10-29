import { loadExperiment } from "./experiment-cache";
import type { RecordedRun } from "./types";
import type { RunState } from "./use-agent-run";

/** Builds the recorded-run file (docs/api.md) from a finished live run. */
export async function buildRecordedRun(state: RunState, model: string): Promise<RecordedRun> {
  const settled = await Promise.allSettled(
    state.experiments.map((e) => loadExperiment(e.id, e.spec)),
  );
  const historyHashes: Record<string, string> = {};
  settled.forEach((r, i) => {
    const e = state.experiments[i];
    if (e && r.status === "fulfilled") historyHashes[e.id] = r.value.historyHash;
  });
  return {
    version: 1,
    target: state.target,
    model,
    startedAt: state.startedAt ?? new Date().toISOString(),
    steps: state.steps,
    experiments: state.experiments.map((e) => e.spec),
    finding: state.finding,
    conclusion: state.conclusion,
    revealedBug: state.revealedBug ?? "",
    usage: state.usage,
    historyHashes,
  };
}

export function downloadJson(filename: string, data: unknown): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
