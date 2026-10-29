// Turns agent steps (assistant blocks plus tool results) into the entries the
// agent log renders. Shared by the live lab and replay.

import type {
  ContentBlock,
  ExperimentSummary,
  RecordedStep,
  TextBlock,
  ToolResultBlock,
  ToolUseBlock,
} from "./types";

export function isText(b: ContentBlock): b is TextBlock {
  return b.type === "text" && typeof (b as TextBlock).text === "string";
}

export function isToolUse(b: ContentBlock): b is ToolUseBlock {
  return b.type === "tool_use" && typeof (b as ToolUseBlock).id === "string";
}

export function isToolResult(b: ContentBlock): b is ToolResultBlock {
  return b.type === "tool_result" && typeof (b as ToolResultBlock).tool_use_id === "string";
}

export function toolResultText(b: ToolResultBlock): string {
  if (typeof b.content === "string") return b.content;
  return (b.content ?? []).map((c) => c.text).join("\n");
}

const EXPERIMENT_TOOLS = new Set(["run_experiment", "shrink"]);

export type TextLabel = "Hypothesis" | "Learned" | "Note";

export type LogEntry =
  | { kind: "text"; key: string; step: number; label: TextLabel; text: string }
  | {
      kind: "tool";
      key: string;
      step: number;
      name: string;
      input: unknown;
      result: string | null;
      isError: boolean;
      experiment: ExperimentSummary | null;
    };

/**
 * Text in a step that launches a new experiment is the hypothesis for it;
 * text in the step right after an experiment or shrink is what was learned.
 */
function labelFor(step: RecordedStep, prev: RecordedStep | undefined): TextLabel {
  const launches = step.assistant.some((b) => isToolUse(b) && b.name === "run_experiment");
  if (launches) return "Hypothesis";
  const followsExperiment = prev?.assistant.some(
    (b) => isToolUse(b) && EXPERIMENT_TOOLS.has(b.name),
  );
  return followsExperiment ? "Learned" : "Note";
}

function experimentFor(
  call: ToolUseBlock,
  result: string | null,
  step: RecordedStep,
): ExperimentSummary | null {
  if (!EXPERIMENT_TOOLS.has(call.name) || result === null) return null;
  // Match by id in the result text, newest first, so a shrink links to the
  // minimal experiment rather than the one it started from.
  for (let i = step.experiments.length - 1; i >= 0; i--) {
    const e = step.experiments[i];
    if (e && result.includes(e.id)) return e;
  }
  return null;
}

export function buildLog(steps: readonly RecordedStep[]): LogEntry[] {
  const entries: LogEntry[] = [];
  steps.forEach((step, s) => {
    const results = new Map<string, ToolResultBlock>();
    for (const b of step.toolResults) if (isToolResult(b)) results.set(b.tool_use_id, b);
    const label = labelFor(step, steps[s - 1]);

    step.assistant.forEach((b, i) => {
      const key = `${s}:${i}`;
      if (isText(b)) {
        const text = b.text.trim();
        if (text) entries.push({ kind: "text", key, step: s, label, text });
      } else if (isToolUse(b)) {
        const res = results.get(b.id);
        const result = res ? toolResultText(res) : null;
        entries.push({
          kind: "tool",
          key,
          step: s,
          name: b.name,
          input: b.input,
          result,
          isError: res?.is_error === true,
          experiment: experimentFor(b, result, step),
        });
      }
    });
  });
  return entries;
}

/** Experiments in the order they first appeared, without duplicates. */
export function collectExperiments(steps: readonly RecordedStep[]): ExperimentSummary[] {
  const seen = new Map<string, ExperimentSummary>();
  for (const step of steps) for (const e of step.experiments) if (!seen.has(e.id)) seen.set(e.id, e);
  return [...seen.values()];
}
