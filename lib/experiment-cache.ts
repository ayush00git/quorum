import { runExperiment } from "./api";
import type { ExperimentResult, Spec } from "./types";

// Experiments are deterministic, so a result fetched once is valid forever.
// One cache per browser tab, shared by the lab, replay and export.
const cache = new Map<string, Promise<ExperimentResult>>();

export function loadExperiment(id: string, spec: Spec): Promise<ExperimentResult> {
  const hit = cache.get(id);
  if (hit) return hit;
  const pending = runExperiment(spec);
  cache.set(id, pending);
  pending.catch(() => cache.delete(id));
  return pending;
}
