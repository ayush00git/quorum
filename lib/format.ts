import type { Fault, Op, Verdict } from "./types";

export function formatMs(ms: number): string {
  if (Math.abs(ms) < 1000) return `${Math.round(ms)} ms`;
  const s = ms / 1000;
  return `${s.toFixed(s < 10 ? 2 : 1)} s`;
}

export function formatCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(n < 10_000 ? 1 : 0)}k`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

const ARROW = "→";
const EMPTY = "∅";

/** Compact porcupine-style label: w(x,3), r(x)→2, r(x)→∅, cas(x,1,2). */
export function opLabel(op: Op): string {
  switch (op.kind) {
    case "write":
      return `w(${op.key},${op.value ?? "?"})`;
    case "read": {
      if (op.status === "info") return `r(${op.key})${ARROW}?`;
      if (op.status === "fail") return `r(${op.key})`;
      return `r(${op.key})${ARROW}${op.result ?? EMPTY}`;
    }
    case "cas":
      return `cas(${op.key},${op.expected ?? "?"},${op.value ?? "?"})`;
  }
}

/** Full, unambiguous description for tooltips and screen readers. */
export function opDescription(op: Op): string {
  const outcome =
    op.status === "info"
      ? "indeterminate (timed out)"
      : op.status === "fail"
        ? "failed"
        : op.kind === "read"
          ? `returned ${op.result ?? "unset"}`
          : "ok";
  const span =
    op.completeAt === null
      ? `invoked ${formatMs(op.invokeAt)}, never completed`
      : `${formatMs(op.invokeAt)} to ${formatMs(op.completeAt)}`;
  return `op ${op.id}: ${opLabel(op)} by p${op.process} via ${op.node || "?"}, ${outcome}, ${span}`;
}

function percent(rate: number): string {
  return `${Math.round(rate * 100)}%`;
}

export function faultLabel(f: Fault): string {
  const a = f.args;
  switch (f.action) {
    case "partition":
      return `partition ${(a.groups ?? []).map((g) => g.join(",")).join(" | ")}`;
    case "isolate":
      return `isolate ${a.node ?? "?"}`;
    case "heal":
      return "heal";
    case "crash":
      return `crash ${a.node ?? "?"}`;
    case "restart":
      return `restart ${a.node ?? "?"}`;
    case "pause":
      return `pause ${a.node ?? "?"} ${a.durationMs ?? 0}ms`;
    case "setNetwork": {
      const parts: string[] = [];
      if (a.minLatencyMs !== undefined || a.maxLatencyMs !== undefined) {
        parts.push(`${a.minLatencyMs ?? "·"}-${a.maxLatencyMs ?? "·"}ms`);
      }
      if (a.dropRate !== undefined) parts.push(`drop ${percent(a.dropRate)}`);
      if (a.duplicateRate !== undefined) parts.push(`dup ${percent(a.duplicateRate)}`);
      return `net ${parts.join(" ") || "reset"}`;
    }
    case "skewClock":
      return `skew ${a.node ?? "?"} ${a.driftPpm ?? 0}ppm`;
  }
}

export function verdictText(v: Verdict): string {
  return v === "ok" ? "linearizable" : v === "violation" ? "violation" : "unknown";
}
