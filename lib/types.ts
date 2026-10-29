// TypeScript mirrors of the Go JSON payloads. Keep field names and optionality
// in lockstep with pkg/experiment/types.go, pkg/workload/op.go,
// pkg/sim/event.go, pkg/checker/result.go and docs/api.md.

// ---- pkg/workload ---------------------------------------------------------

export type OpKind = "read" | "write" | "cas";
export type OpStatus = "ok" | "fail" | "info";

export interface Op {
  id: number;
  process: number;
  kind: OpKind;
  key: string;
  /** Write value or cas target. */
  value?: number;
  /** cas only. */
  expected?: number;
  /** Read result; absent means the key was unset. */
  result?: number;
  status: OpStatus;
  invokeAt: number;
  /** null for indeterminate (info) operations. */
  completeAt: number | null;
  node: string;
}

// ---- pkg/sim --------------------------------------------------------------

export type EventKind =
  | "election-started"
  | "vote-granted"
  | "became-leader"
  | "stepped-down"
  | "commit-advanced"
  | "crash"
  | "restart"
  | "pause-start"
  | "pause-end"
  | "partition"
  | "heal"
  | "network-change"
  | "clock-skew";

export interface SimEvent {
  atMs: number;
  /** Absent for cluster-wide events (partition, heal, network). */
  node?: string;
  kind: EventKind;
  term?: number;
  detail?: string;
}

// ---- pkg/checker ----------------------------------------------------------

export type Verdict = "ok" | "violation" | "unknown";

export interface CheckResult {
  verdict: Verdict;
  key?: string;
  linearizedIds?: number[];
  /** illegalIds plus the ops they conflict with. */
  offendingIds?: number[];
  /** Offending ops that cannot follow the longest linearizable prefix. */
  illegalIds?: number[];
  explanation?: string;
}

// ---- pkg/experiment -------------------------------------------------------

export type FaultAction =
  | "partition"
  | "isolate"
  | "heal"
  | "crash"
  | "restart"
  | "pause"
  | "setNetwork"
  | "skewClock";

export type ReadFrom = "leader" | "any";

export interface Network {
  minLatencyMs: number;
  maxLatencyMs: number;
  dropRate: number;
  duplicateRate: number;
}

export interface FaultArgs {
  groups?: string[][];
  node?: string;
  durationMs?: number;
  minLatencyMs?: number;
  maxLatencyMs?: number;
  dropRate?: number;
  duplicateRate?: number;
  driftPpm?: number;
}

export interface Fault {
  atMs: number;
  action: FaultAction;
  args: FaultArgs;
}

export interface Spec {
  seed: number;
  target: string;
  durationMs: number;
  nodes: number;
  clients: number;
  keys: number;
  readRatio: number;
  readFrom: ReadFrom;
  opTimeoutMs: number;
  network: Network;
  faults: Fault[];
}

/** A request spec: omitted fields get server defaults (experiment.Normalize). */
export type SpecInput = Partial<Omit<Spec, "network">> & {
  target: string;
  network?: Partial<Network>;
};

export interface LeaderSpan {
  term: number;
  node: string;
  fromMs: number;
  toMs: number;
}

export interface Stats {
  ops: number;
  ok: number;
  fail: number;
  info: number;
  reads: number;
  writes: number;
  messagesSent: number;
  messagesDropped: number;
  leaders: LeaderSpan[];
}

export interface ExperimentResult {
  id: string;
  spec: Spec;
  history: Op[];
  events: SimEvent[];
  check: CheckResult;
  stats: Stats;
  historyHash: string;
}

export interface TargetInfo {
  codename: string;
  difficulty: string;
}

// ---- HTTP: /api/meta ------------------------------------------------------

export interface Limits {
  maxDurationMs: number;
  maxFaults: number;
  nodes: number;
  minSteps: number;
  maxStepsCap: number;
}

export interface Meta {
  model: string;
  maxSteps: number;
  accessCodeRequired: boolean;
  targets: TargetInfo[];
  limits: Limits;
}

// ---- HTTP: /api/experiment ------------------------------------------------

export interface ExperimentRequest {
  spec: SpecInput;
}

// ---- HTTP: /api/agent (Anthropic Messages API shapes) ---------------------

export interface TextBlock {
  type: "text";
  text: string;
}

export interface ToolUseBlock {
  type: "tool_use";
  id: string;
  name: string;
  input: unknown;
}

export interface ToolResultBlock {
  type: "tool_result";
  tool_use_id: string;
  content: string | TextBlock[];
  is_error?: boolean;
}

/** Any other block the API may return (thinking, etc.), passed through verbatim. */
export interface OtherBlock {
  type: string;
  [field: string]: unknown;
}

export type ContentBlock = TextBlock | ToolUseBlock | ToolResultBlock | OtherBlock;

export type Role = "user" | "assistant";

export interface Message {
  role: Role;
  content: ContentBlock[];
}

export interface AgentRequest {
  target: string;
  maxSteps?: number;
  messages: Message[];
  accessCode?: string;
}

export interface ExperimentSummary {
  id: string;
  spec: Spec;
  verdict: Verdict;
  ops: number;
  faults: number;
  durationMs: number;
  /** Id of the experiment this one was shrunk from; empty when not a shrink. */
  shrunkFrom: string;
}

export interface Usage {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens: number;
  cacheCreationTokens: number;
}

export interface Finding {
  experimentId: string;
  title: string;
  anomaly: string;
  causalChain: string[];
  spec: Spec;
  check: CheckResult;
}

export interface AgentResponse {
  kickoff?: Message;
  assistant: ContentBlock[];
  toolResults: ContentBlock[];
  experiments: ExperimentSummary[];
  usage: Usage;
  step: number;
  done: boolean;
  finding?: Finding;
  conclusion?: string;
  revealedBug?: string;
}

// ---- Errors ---------------------------------------------------------------

export type ErrorCode =
  | "bad_request"
  | "access_denied"
  | "method_not_allowed"
  | "too_large"
  | "rate_limited"
  | "overloaded"
  | "upstream_error"
  | "internal"
  | "not_configured";

export interface ErrorBody {
  error: { code: ErrorCode | string; message: string };
}

// ---- Recorded runs (public/runs) ------------------------------------------

export interface RecordedStep {
  assistant: ContentBlock[];
  toolResults: ContentBlock[];
  experiments: ExperimentSummary[];
}

export interface RecordedRun {
  version: number;
  target: string;
  model: string;
  startedAt: string;
  steps: RecordedStep[];
  experiments: Spec[];
  finding: Finding | null;
  conclusion: string | null;
  revealedBug: string;
  usage: Usage;
  historyHashes: Record<string, string>;
}

export interface RunIndexEntry {
  id: string;
  target: string;
  steps: number;
  found: boolean;
}
