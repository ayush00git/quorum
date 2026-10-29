// Pure layout math for the porcupine-style history timeline. No React, no DOM,
// so it can be unit tested and reused by the lab, replay and landing hero.

import { faultLabel, opLabel } from "./format";
import type { CheckResult, Fault, Op } from "./types";

/** Visible time window in virtual milliseconds. */
export interface Viewport {
  start: number;
  end: number;
}

export const MIN_SPAN_MS = 10;
/** Approximate advance width of an 11px Geist Mono glyph. */
export const LABEL_CHAR_PX = 6.7;
const LABEL_PAD_PX = 8;
const MIN_BAR_PX = 2;

export function fullViewport(durationMs: number): Viewport {
  return { start: 0, end: Math.max(durationMs, MIN_SPAN_MS) };
}

/** Keeps the window inside [0, duration] and at least MIN_SPAN_MS wide. */
export function clampViewport(v: Viewport, durationMs: number): Viewport {
  const total = Math.max(durationMs, MIN_SPAN_MS);
  const span = Math.min(Math.max(v.end - v.start, MIN_SPAN_MS), total);
  const start = Math.min(Math.max(v.start, 0), total - span);
  return { start, end: start + span };
}

/** Zooms around `anchorMs`, which stays at the same screen position. factor < 1 zooms in. */
export function zoomViewport(
  v: Viewport,
  factor: number,
  anchorMs: number,
  durationMs: number,
): Viewport {
  const anchor = Math.min(Math.max(anchorMs, v.start), v.end);
  const start = anchor - (anchor - v.start) * factor;
  const end = anchor + (v.end - anchor) * factor;
  return clampViewport({ start, end }, durationMs);
}

export function panViewport(v: Viewport, deltaMs: number, durationMs: number): Viewport {
  return clampViewport({ start: v.start + deltaMs, end: v.end + deltaMs }, durationMs);
}

export function timeToX(t: number, v: Viewport, width: number): number {
  return ((t - v.start) / (v.end - v.start)) * width;
}

export function xToTime(x: number, v: Viewport, width: number): number {
  return v.start + (x / width) * (v.end - v.start);
}

/** Round tick positions (1, 2 or 5 times a power of ten) covering the window. */
export function niceTicks(v: Viewport, targetCount: number): number[] {
  const span = v.end - v.start;
  if (span <= 0 || targetCount < 1) return [];
  const raw = span / targetCount;
  const pow = 10 ** Math.floor(Math.log10(raw));
  const unit = raw / pow;
  const step = (unit <= 1 ? 1 : unit <= 2 ? 2 : unit <= 5 ? 5 : 10) * pow;
  const ticks: number[] = [];
  for (let t = Math.ceil(v.start / step) * step; t <= v.end + 1e-9; t += step) {
    ticks.push(Math.round(t * 1000) / 1000);
  }
  return ticks;
}

/** One lane per process id, ascending. */
export function assignLanes(history: readonly Op[]): Map<number, number> {
  const processes = [...new Set(history.map((op) => op.process))].sort((a, b) => a - b);
  return new Map(processes.map((p, i) => [p, i]));
}

/**
 * ok: plain outline. fail: did not take effect. info: indeterminate, open ended.
 * offending: cannot be linearized (illegalIds, or offending but not in
 * linearizedIds when illegalIds is absent).
 * conflicting: the rest of the offending set; the ops the illegal ones
 * conflict with.
 */
export type BarRole = "ok" | "fail" | "info" | "offending" | "conflicting";

export function opRole(op: Op, check: CheckResult | null): BarRole {
  if (check?.verdict === "violation" && check.offendingIds?.includes(op.id)) {
    // Prefer the checker's explicit illegal set; older results lack it.
    if (check.illegalIds) return check.illegalIds.includes(op.id) ? "offending" : "conflicting";
    return check.linearizedIds?.includes(op.id) ? "conflicting" : "offending";
  }
  return op.status;
}

export interface Bar {
  op: Op;
  lane: number;
  x: number;
  width: number;
  /** Indeterminate op: the bar runs to the end of the history. */
  openEnded: boolean;
  role: BarRole;
  label: string;
  showLabel: boolean;
}

export interface TimelineInput {
  history: readonly Op[];
  check: CheckResult | null;
  durationMs: number;
  viewport: Viewport;
  width: number;
}

export interface TimelineLayout {
  bars: Bar[];
  lanes: number[];
}

export function labelWidth(label: string): number {
  return label.length * LABEL_CHAR_PX + LABEL_PAD_PX;
}

export function layoutTimeline({
  history,
  check,
  durationMs,
  viewport,
  width,
}: TimelineInput): TimelineLayout {
  const laneOf = assignLanes(history);
  const bars: Bar[] = [];
  for (const op of history) {
    const openEnded = op.completeAt === null;
    const end = op.completeAt ?? Math.max(durationMs, op.invokeAt);
    const x0 = timeToX(op.invokeAt, viewport, width);
    const x1 = timeToX(end, viewport, width);
    if (x1 < 0 || x0 > width) continue;
    const left = Math.max(x0, 0);
    const right = Math.min(Math.max(x1, x0 + MIN_BAR_PX), width);
    const barWidth = Math.max(right - left, MIN_BAR_PX);
    const label = opLabel(op);
    bars.push({
      op,
      lane: laneOf.get(op.process) ?? 0,
      x: left,
      width: barWidth,
      openEnded,
      role: opRole(op, check),
      label,
      showLabel: barWidth >= labelWidth(label),
    });
  }
  return { bars, lanes: [...laneOf.keys()] };
}

export interface FaultMarker {
  fault: Fault;
  x: number;
  label: string;
}

export function layoutFaults(
  faults: readonly Fault[],
  viewport: Viewport,
  width: number,
): FaultMarker[] {
  return faults
    .filter((f) => f.atMs >= viewport.start && f.atMs <= viewport.end)
    .map((f) => ({ fault: f, x: timeToX(f.atMs, viewport, width), label: faultLabel(f) }));
}

/** Staggers marker labels onto rows so neighbours do not overlap. */
export function stackLabels(markers: readonly { x: number; label: string }[]): number[] {
  const rowEnds: number[] = [];
  return markers.map((m) => {
    const w = labelWidth(m.label);
    let row = rowEnds.findIndex((end) => m.x > end);
    if (row === -1) {
      row = rowEnds.length;
      rowEnds.push(0);
    }
    rowEnds[row] = m.x + w;
    return row;
  });
}
