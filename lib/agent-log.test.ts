import { describe, expect, it } from "vitest";
import { buildLog, collectExperiments } from "./agent-log";
import type { ExperimentSummary, RecordedStep, Spec } from "./types";

const spec = { target: "target-a" } as Spec;
const exp = (id: string): ExperimentSummary => ({
  id,
  spec,
  verdict: "ok",
  ops: 10,
  faults: 1,
  durationMs: 1000,
  shrunkFrom: "",
});

const steps: RecordedStep[] = [
  {
    assistant: [
      { type: "text", text: "Isolating the leader should expose stale reads." },
      { type: "tool_use", id: "t1", name: "run_experiment", input: { seed: 1 } },
    ],
    toolResults: [{ type: "tool_result", tool_use_id: "t1", content: "experiment abc123: ok" }],
    experiments: [exp("abc123")],
  },
  {
    assistant: [
      { type: "text", text: "No violation; reads went through the log." },
      { type: "tool_use", id: "t2", name: "inspect_events", input: { experimentId: "abc123" } },
    ],
    toolResults: [
      { type: "tool_result", tool_use_id: "t2", content: [{ type: "text", text: "bad input" }], is_error: true },
    ],
    experiments: [],
  },
  { assistant: [{ type: "text", text: "  " }, { type: "thinking", thinking: "..." }], toolResults: [], experiments: [] },
];

describe("buildLog", () => {
  const log = buildLog(steps);

  it("labels hypotheses and lessons", () => {
    const texts = log.filter((e) => e.kind === "text");
    expect(texts.map((e) => e.label)).toEqual(["Hypothesis", "Learned"]);
  });

  it("pairs tool calls with results and experiments", () => {
    const tools = log.filter((e) => e.kind === "tool");
    expect(tools).toHaveLength(2);
    expect(tools[0]).toMatchObject({ name: "run_experiment", result: "experiment abc123: ok", isError: false });
    expect(tools[0]?.kind === "tool" && tools[0].experiment?.id).toBe("abc123");
    expect(tools[1]).toMatchObject({ result: "bad input", isError: true, experiment: null });
  });

  it("skips blank text and unknown blocks", () => {
    expect(log.filter((e) => e.step === 2)).toEqual([]);
  });
});

describe("collectExperiments", () => {
  it("dedupes by id in first-seen order", () => {
    const s: RecordedStep[] = [
      { assistant: [], toolResults: [], experiments: [exp("a"), exp("b")] },
      { assistant: [], toolResults: [], experiments: [exp("a"), exp("c")] },
    ];
    expect(collectExperiments(s).map((e) => e.id)).toEqual(["a", "b", "c"]);
  });
});
