"use client";

import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { HatchDefs } from "@/components/ui/hatch";
import { formatMs, opDescription } from "@/lib/format";
import {
  type Bar,
  fullViewport,
  labelWidth,
  layoutFaults,
  layoutTimeline,
  niceTicks,
  panViewport,
  stackLabels,
  timeToX,
  xToTime,
  zoomViewport,
} from "@/lib/timeline";
import type { ExperimentResult } from "@/lib/types";
import { useElementWidth } from "@/lib/use-element-width";
import { TimelineLegend } from "./timeline-legend";
import { TimeScrubber, TimelineToolbar } from "./timeline-toolbar";

const GUTTER = 36;
const RIGHT_PAD = 8;
const AXIS_H = 18;
const FAULT_ROW_H = 13;
const LANE_H = 22;
const BAR_H = 15;

const BAR_CLASS: Record<Bar["role"], string> = {
  ok: "fill-bg stroke-fg",
  conflicting: "fill-bg stroke-fg",
  offending: "fill-fg stroke-fg",
  info: "stroke-fg",
  fail: "fill-none stroke-subtle",
};

const TEXT_CLASS: Record<Bar["role"], string> = {
  ok: "fill-fg",
  conflicting: "fill-fg font-semibold",
  offending: "fill-bg font-semibold",
  info: "fill-fg stroke-bg",
  fail: "fill-subtle",
};

interface Hover {
  bar: Bar;
  left: number;
  top: number;
}

export interface TimelineProps {
  result: ExperimentResult;
  /** Scrubber position in virtual ms; drives the cluster view. */
  cursorMs?: number;
  onCursorChange?: (ms: number) => void;
  /** Zoom, pan, scrubber and legend. Off for the landing hero. */
  interactive?: boolean;
}

export function Timeline({ result, cursorMs, onCursorChange, interactive = true }: TimelineProps) {
  const duration = result.spec.durationMs;
  const [container, width] = useElementWidth<HTMLDivElement>();
  const svgRef = useRef<SVGSVGElement | null>(null);
  const hatchId = useId();
  const [viewport, setViewport] = useState(() => fullViewport(duration));
  const [hover, setHover] = useState<Hover | null>(null);
  const drag = useRef<{ x: number; start: typeof viewport; moved: boolean } | null>(null);

  const plotW = Math.max(width - GUTTER - RIGHT_PAD, 1);
  const layout = useMemo(
    () =>
      layoutTimeline({ history: result.history, check: result.check, durationMs: duration, viewport, width: plotW }),
    [result.history, result.check, duration, viewport, plotW],
  );
  const faults = useMemo(() => layoutFaults(result.spec.faults, viewport, plotW), [result.spec.faults, viewport, plotW]);
  const faultRows = useMemo(() => stackLabels(faults), [faults]);
  const ticks = useMemo(() => niceTicks(viewport, Math.max(Math.floor(plotW / 90), 2)), [viewport, plotW]);

  const rowCount = faultRows.length ? Math.max(...faultRows) + 1 : 0;
  const top = AXIS_H + rowCount * FAULT_ROW_H + 4;
  const height = top + Math.max(layout.lanes.length, 1) * LANE_H + 4;

  const zoom = useCallback(
    (factor: number, anchor?: number) =>
      setViewport((v) => zoomViewport(v, factor, anchor ?? (v.start + v.end) / 2, duration)),
    [duration],
  );

  // Native listener: React's onWheel is passive and cannot preventDefault.
  useEffect(() => {
    const svg = svgRef.current;
    if (!svg || !interactive) return;
    const onWheel = (e: WheelEvent) => {
      const rect = svg.getBoundingClientRect();
      const x = e.clientX - rect.left - GUTTER;
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        setViewport((v) => zoomViewport(v, Math.exp(e.deltaY * 0.002), xToTime(x, v, plotW), duration));
      } else if (e.shiftKey || Math.abs(e.deltaX) > Math.abs(e.deltaY)) {
        e.preventDefault();
        const d = e.shiftKey ? e.deltaY : e.deltaX;
        setViewport((v) => panViewport(v, (d / plotW) * (v.end - v.start), duration));
      }
    };
    svg.addEventListener("wheel", onWheel, { passive: false });
    return () => svg.removeEventListener("wheel", onWheel);
  }, [interactive, plotW, duration]);

  const onPointerDown = (e: React.PointerEvent<SVGSVGElement>) => {
    if (!interactive || e.button !== 0) return;
    drag.current = { x: e.clientX, start: viewport, moved: false };
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    const d = drag.current;
    if (!d) return;
    const dx = e.clientX - d.x;
    if (Math.abs(dx) > 3) d.moved = true;
    if (d.moved) {
      setHover(null);
      setViewport(panViewport(d.start, (-dx / plotW) * (d.start.end - d.start.start), duration));
    }
  };

  const onPointerUp = (e: React.PointerEvent<SVGSVGElement>) => {
    const d = drag.current;
    drag.current = null;
    if (!d || d.moved || !onCursorChange) return;
    const x = e.clientX - e.currentTarget.getBoundingClientRect().left - GUTTER;
    if (x >= 0) onCursorChange(Math.min(Math.max(xToTime(x, viewport, plotW), 0), duration));
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const span = viewport.end - viewport.start;
    const actions: Record<string, () => void> = {
      ArrowLeft: () => setViewport((v) => panViewport(v, -span * 0.1, duration)),
      ArrowRight: () => setViewport((v) => panViewport(v, span * 0.1, duration)),
      "+": () => zoom(0.7),
      "=": () => zoom(0.7),
      "-": () => zoom(1 / 0.7),
      "0": () => setViewport(fullViewport(duration)),
    };
    const act = actions[e.key];
    if (act) {
      e.preventDefault();
      act();
    }
  };

  const cursorX = cursorMs !== undefined ? timeToX(cursorMs, viewport, plotW) : null;
  const offending = layout.bars.filter((b) => b.role === "offending").length;

  return (
    <div className="flex flex-col gap-2">
      {interactive ? (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <TimelineLegend />
          <TimelineToolbar
            viewport={viewport}
            onZoomIn={() => zoom(0.6)}
            onZoomOut={() => zoom(1 / 0.6)}
            onFit={() => setViewport(fullViewport(duration))}
          />
        </div>
      ) : null}
      <div ref={container} className="relative w-full">
        {width > 0 ? (
          <svg
            ref={svgRef}
            width={width}
            height={height}
            role="img"
            tabIndex={interactive ? 0 : undefined}
            aria-label={`History timeline: ${result.history.length} operations across ${layout.lanes.length} processes, verdict ${result.check.verdict}${offending ? `, ${offending} operations not linearizable` : ""}. Arrow keys pan, plus and minus zoom, 0 resets.`}
            className={`block select-none font-mono ${interactive ? "cursor-grab active:cursor-grabbing" : ""}`}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={() => (drag.current = null)}
            onPointerLeave={() => setHover(null)}
            onKeyDown={interactive ? onKeyDown : undefined}
          >
            <HatchDefs id={hatchId} className="stroke-muted" gap={4} />

            {layout.lanes.map((p, i) => (
              <g key={p}>
                <line x1={0} x2={width} y1={top + (i + 1) * LANE_H - 0.5} y2={top + (i + 1) * LANE_H - 0.5} className="stroke-border" />
                <text x={GUTTER - 8} y={top + i * LANE_H + LANE_H / 2 + 3.5} textAnchor="end" className="fill-subtle text-[10px] tabular-nums">
                  p{p}
                </text>
              </g>
            ))}

            <g transform={`translate(${GUTTER},0)`}>
              {ticks.map((t) => {
                const x = Math.round(timeToX(t, viewport, plotW)) + 0.5;
                return (
                  <g key={t}>
                    <line x1={x} x2={x} y1={AXIS_H - 4} y2={height} className="stroke-border" />
                    {x + 3 + labelWidth(formatMs(t)) <= plotW + RIGHT_PAD ? (
                      <text x={x + 3} y={11} className="fill-subtle text-[10px] tabular-nums">
                        {formatMs(t)}
                      </text>
                    ) : null}
                  </g>
                );
              })}

              {faults.map((m, i) => {
                const row = faultRows[i] ?? 0;
                const x = Math.round(m.x) + 0.5;
                const flip = m.x + labelWidth(m.label) > plotW;
                return (
                  <g key={`${m.fault.atMs}-${i}`}>
                    <line x1={x} x2={x} y1={AXIS_H + row * FAULT_ROW_H + 2} y2={height} className="stroke-fg" strokeDasharray="3 3" />
                    <text
                      x={flip ? x - 4 : x + 4}
                      y={AXIS_H + row * FAULT_ROW_H + 11}
                      textAnchor={flip ? "end" : "start"}
                      className="fill-fg text-[10px]"
                    >
                      {m.label}
                    </text>
                  </g>
                );
              })}

              {layout.bars.map((b) => {
                const y = top + b.lane * LANE_H + (LANE_H - BAR_H) / 2;
                const hovered = hover?.bar.op.id === b.op.id;
                return (
                  <g
                    key={b.op.id}
                    onPointerEnter={() => setHover({ bar: b, left: GUTTER + b.x, top: y + BAR_H + 4 })}
                    onPointerLeave={() => setHover((h) => (h?.bar.op.id === b.op.id ? null : h))}
                  >
                    <rect
                      x={b.x + 0.5}
                      y={y + 0.5}
                      width={Math.max(b.width - 1, 1)}
                      height={BAR_H - 1}
                      rx={1}
                      className={BAR_CLASS[b.role]}
                      fill={b.role === "info" ? `url(#${hatchId})` : undefined}
                      strokeWidth={b.role === "conflicting" || hovered ? 2 : 1}
                      strokeDasharray={b.role === "info" ? "3 2" : b.role === "fail" ? "1 2" : undefined}
                    />
                    {b.showLabel ? (
                      <text
                        x={b.x + 4}
                        y={y + BAR_H / 2 + 3.5}
                        className={`pointer-events-none text-[11px] tabular-nums ${TEXT_CLASS[b.role]}`}
                        strokeWidth={b.role === "info" ? 3 : 0}
                        paintOrder="stroke"
                      >
                        {b.label}
                      </text>
                    ) : null}
                  </g>
                );
              })}

              {cursorX !== null && cursorX >= 0 && cursorX <= plotW ? (
                <g className="pointer-events-none">
                  <line x1={cursorX} x2={cursorX} y1={AXIS_H - 4} y2={height} className="stroke-fg" strokeWidth={1} />
                  <path d={`M${cursorX - 4} ${AXIS_H - 6}h8l-4 5z`} className="fill-fg" />
                </g>
              ) : null}
            </g>
          </svg>
        ) : (
          <div style={{ height }} />
        )}

        {hover ? (
          <div
            role="tooltip"
            className="pointer-events-none absolute z-10 max-w-72 rounded border border-border-strong bg-bg px-2.5 py-2 font-mono text-[11px] leading-4"
            style={{ left: Math.min(hover.left, Math.max(width - 288, 0)), top: hover.top }}
          >
            <p className="text-fg">{opDescription(hover.bar.op)}</p>
            {hover.bar.role === "offending" || hover.bar.role === "conflicting" ? (
              <p className="mt-1 text-muted">
                {hover.bar.role === "offending" ? "Cannot be linearized." : "Conflicts with the offending ops."}
              </p>
            ) : null}
          </div>
        ) : null}
      </div>

      {interactive && onCursorChange && cursorMs !== undefined ? (
        <TimeScrubber value={cursorMs} max={duration} onChange={onCursorChange} />
      ) : null}
    </div>
  );
}
