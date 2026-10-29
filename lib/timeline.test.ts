import { describe, expect, it } from "vitest";
import {
  MIN_SPAN_MS,
  assignLanes,
  clampViewport,
  labelWidth,
  layoutFaults,
  layoutTimeline,
  niceTicks,
  opRole,
  panViewport,
  stackLabels,
  timeToX,
  xToTime,
  zoomViewport,
} from "./timeline";
import type { CheckResult, Op } from "./types";

function op(partial: Partial<Op> & Pick<Op, "id" | "process">): Op {
  return {
    kind: "write",
    key: "x",
    value: partial.id,
    status: "ok",
    invokeAt: 0,
    completeAt: 10,
    node: "n1",
    ...partial,
  };
}

describe("viewport math", () => {
  it("maps time to x and back", () => {
    const v = { start: 100, end: 300 };
    expect(timeToX(200, v, 400)).toBe(200);
    expect(xToTime(200, v, 400)).toBe(200);
    expect(xToTime(timeToX(137, v, 913), v, 913)).toBeCloseTo(137);
  });

  it("clamps into bounds and enforces a minimum span", () => {
    expect(clampViewport({ start: -50, end: 150 }, 1000)).toEqual({ start: 0, end: 200 });
    expect(clampViewport({ start: 900, end: 1200 }, 1000)).toEqual({ start: 700, end: 1000 });
    expect(clampViewport({ start: 0, end: 5000 }, 1000)).toEqual({ start: 0, end: 1000 });
    const tiny = clampViewport({ start: 500, end: 501 }, 1000);
    expect(tiny.end - tiny.start).toBe(MIN_SPAN_MS);
  });

  it("zooms around the anchor, keeping it fixed on screen", () => {
    const v = { start: 0, end: 1000 };
    const z = zoomViewport(v, 0.5, 400, 1000);
    expect(z).toEqual({ start: 200, end: 700 });
    expect(timeToX(400, z, 100)).toBeCloseTo(timeToX(400, v, 100));
  });

  it("zooming out never exceeds the history", () => {
    expect(zoomViewport({ start: 200, end: 700 }, 4, 400, 1000)).toEqual({ start: 0, end: 1000 });
  });

  it("pans without leaving the history", () => {
    expect(panViewport({ start: 0, end: 200 }, 100, 1000)).toEqual({ start: 100, end: 300 });
    expect(panViewport({ start: 0, end: 200 }, -100, 1000)).toEqual({ start: 0, end: 200 });
    expect(panViewport({ start: 700, end: 900 }, 500, 1000)).toEqual({ start: 800, end: 1000 });
  });
});

describe("niceTicks", () => {
  it("uses round steps", () => {
    expect(niceTicks({ start: 0, end: 1000 }, 5)).toEqual([0, 200, 400, 600, 800, 1000]);
    expect(niceTicks({ start: 0, end: 3000 }, 6)).toEqual([0, 500, 1000, 1500, 2000, 2500, 3000]);
  });

  it("starts at the first step inside the window", () => {
    expect(niceTicks({ start: 130, end: 170 }, 4)).toEqual([130, 140, 150, 160, 170]);
  });

  it("returns nothing for empty windows", () => {
    expect(niceTicks({ start: 5, end: 5 }, 4)).toEqual([]);
  });
});

describe("lanes", () => {
  it("assigns one lane per process in ascending order", () => {
    const lanes = assignLanes([op({ id: 1, process: 7 }), op({ id: 2, process: 2 }), op({ id: 3, process: 7 })]);
    expect([...lanes.entries()]).toEqual([
      [2, 0],
      [7, 1],
    ]);
  });
});

describe("opRole", () => {
  const violation: CheckResult = {
    verdict: "violation",
    key: "x",
    linearizedIds: [1, 2],
    offendingIds: [2, 3],
  };

  it("inverts unlinearizable ops and outlines the ops they conflict with", () => {
    expect(opRole(op({ id: 3, process: 1 }), violation)).toBe("offending");
    expect(opRole(op({ id: 2, process: 1 }), violation)).toBe("conflicting");
    expect(opRole(op({ id: 1, process: 1 }), violation)).toBe("ok");
  });

  it("falls back to the op status", () => {
    expect(opRole(op({ id: 4, process: 1, status: "info", completeAt: null }), null)).toBe("info");
    expect(opRole(op({ id: 5, process: 1, status: "fail" }), { verdict: "ok" })).toBe("fail");
  });

  it("ignores offending ids unless the verdict is a violation", () => {
    expect(opRole(op({ id: 3, process: 1 }), { ...violation, verdict: "unknown" })).toBe("ok");
  });
});

describe("layoutTimeline", () => {
  const viewport = { start: 0, end: 1000 };

  it("positions bars from invoke to complete", () => {
    const { bars } = layoutTimeline({
      history: [op({ id: 1, process: 0, invokeAt: 100, completeAt: 300 })],
      check: null,
      durationMs: 1000,
      viewport,
      width: 1000,
    });
    expect(bars).toHaveLength(1);
    expect(bars[0]).toMatchObject({ x: 100, width: 200, openEnded: false, lane: 0, label: "w(x,1)" });
  });

  it("runs indeterminate ops to the end of the history", () => {
    const { bars } = layoutTimeline({
      history: [op({ id: 1, process: 0, status: "info", invokeAt: 600, completeAt: null })],
      check: null,
      durationMs: 1000,
      viewport,
      width: 500,
    });
    expect(bars[0]).toMatchObject({ x: 300, width: 200, openEnded: true, role: "info" });
  });

  it("clips to the viewport and culls ops outside it", () => {
    const { bars } = layoutTimeline({
      history: [
        op({ id: 1, process: 0, invokeAt: 0, completeAt: 50 }),
        op({ id: 2, process: 0, invokeAt: 150, completeAt: 400 }),
        op({ id: 3, process: 1, invokeAt: 450, completeAt: 600 }),
      ],
      check: null,
      durationMs: 1000,
      viewport: { start: 200, end: 500 },
      width: 300,
    });
    expect(bars.map((b) => b.op.id)).toEqual([2, 3]);
    expect(bars[0]).toMatchObject({ x: 0, width: 200 });
    expect(bars[1]).toMatchObject({ x: 250, width: 50, lane: 1 });
  });

  it("gives instantaneous ops a visible minimum width", () => {
    const { bars } = layoutTimeline({
      history: [op({ id: 1, process: 0, invokeAt: 500, completeAt: 500 })],
      check: null,
      durationMs: 1000,
      viewport,
      width: 100,
    });
    expect(bars[0]?.width).toBe(2);
  });

  it("hides labels that do not fit their bar", () => {
    const history = [
      op({ id: 1, process: 0, invokeAt: 0, completeAt: 20 }),
      op({ id: 2, process: 0, invokeAt: 100, completeAt: 900 }),
    ];
    const { bars } = layoutTimeline({ history, check: null, durationMs: 1000, viewport, width: 1000 });
    expect(bars.map((b) => b.showLabel)).toEqual([false, true]);
  });

  it("formats read labels with results", () => {
    const { bars } = layoutTimeline({
      history: [
        op({ id: 1, process: 0, kind: "read", value: undefined, result: 4 }),
        op({ id: 2, process: 1, kind: "read", value: undefined }),
      ],
      check: null,
      durationMs: 10,
      viewport: { start: 0, end: 10 },
      width: 100,
    });
    expect(bars.map((b) => b.label)).toEqual(["r(x)→4", "r(x)→∅"]);
  });
});

describe("fault markers", () => {
  it("keeps only faults inside the window and labels them", () => {
    const markers = layoutFaults(
      [
        { atMs: 100, action: "isolate", args: { node: "n3" } },
        { atMs: 700, action: "heal", args: {} },
        { atMs: 2000, action: "crash", args: { node: "n1" } },
      ],
      { start: 0, end: 1000 },
      500,
    );
    expect(markers.map((m) => [m.x, m.label])).toEqual([
      [50, "isolate n3"],
      [350, "heal"],
    ]);
  });

  it("stacks overlapping labels onto new rows", () => {
    const w = labelWidth("isolate n3");
    expect(
      stackLabels([
        { x: 0, label: "isolate n3" },
        { x: w / 2, label: "heal" },
        { x: w + 1, label: "heal" },
      ]),
    ).toEqual([0, 1, 0]);
  });
});
