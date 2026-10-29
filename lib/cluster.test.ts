import { describe, expect, it } from "vitest";
import { clusterAt, connected, groupsAt, nodeIds } from "./cluster";
import type { Fault, SimEvent } from "./types";

const ids = nodeIds(5);

describe("groupsAt", () => {
  const faults: Fault[] = [
    { atMs: 100, action: "isolate", args: { node: "n2" } },
    { atMs: 500, action: "heal", args: {} },
    { atMs: 800, action: "partition", args: { groups: [["n1", "n2"], ["n3", "n4"]] } },
  ];

  it("tracks isolate, heal and partition over time", () => {
    expect(groupsAt(faults, ids, 50)).toBeNull();
    expect(groupsAt(faults, ids, 100)).toEqual([["n2"], ["n1", "n3", "n4", "n5"]]);
    expect(groupsAt(faults, ids, 600)).toBeNull();
    expect(groupsAt(faults, ids, 900)).toEqual([["n1", "n2"], ["n3", "n4"], ["n5"]]);
  });

  it("decides connectivity by group membership", () => {
    const g = groupsAt(faults, ids, 100);
    expect(connected(g, "n1", "n3")).toBe(true);
    expect(connected(g, "n1", "n2")).toBe(false);
    expect(connected(null, "n1", "n2")).toBe(true);
  });
});

describe("clusterAt", () => {
  const events: SimEvent[] = [
    { atMs: 10, node: "n1", kind: "election-started", term: 1 },
    { atMs: 11, node: "n3", kind: "election-started", term: 1 },
    { atMs: 20, node: "n1", kind: "became-leader", term: 1 },
    { atMs: 300, node: "n1", kind: "crash" },
    { atMs: 400, node: "n4", kind: "pause-start" },
    { atMs: 450, node: "n2", kind: "election-started", term: 2 },
    { atMs: 460, node: "n2", kind: "became-leader", term: 2 },
    { atMs: 600, node: "n4", kind: "pause-end" },
    { atMs: 700, node: "n1", kind: "restart" },
  ];

  it("starts with all followers in term 0", () => {
    const s = clusterAt(5, events, [], 0);
    expect(s.nodes.map((n) => [n.id, n.role, n.term])).toEqual(ids.map((id) => [id, "follower", 0]));
  });

  it("elects a leader and defeats rival candidates", () => {
    const s = clusterAt(5, events, [], 100);
    expect(s.nodes.find((n) => n.id === "n1")).toMatchObject({ role: "leader", term: 1 });
    expect(s.nodes.find((n) => n.id === "n3")).toMatchObject({ role: "follower", term: 1 });
  });

  it("tracks crashes, pauses and new leaders", () => {
    const s = clusterAt(5, events, [], 500);
    expect(s.nodes.find((n) => n.id === "n1")).toMatchObject({ crashed: true, role: "follower", term: 1 });
    expect(s.nodes.find((n) => n.id === "n4")?.paused).toBe(true);
    expect(s.nodes.find((n) => n.id === "n2")).toMatchObject({ role: "leader", term: 2 });
  });

  it("clears crash and pause on restart and resume", () => {
    const s = clusterAt(5, events, [], 1000);
    expect(s.nodes.find((n) => n.id === "n1")?.crashed).toBe(false);
    expect(s.nodes.find((n) => n.id === "n4")?.paused).toBe(false);
  });

  it("keeps a deposed leader marked until it steps down", () => {
    const zombie: SimEvent[] = [
      { atMs: 0, node: "n1", kind: "became-leader", term: 1 },
      { atMs: 50, node: "n2", kind: "became-leader", term: 2 },
    ];
    const leaders = clusterAt(5, zombie, [], 60).nodes.filter((n) => n.role === "leader");
    expect(leaders.map((n) => n.id)).toEqual(["n1", "n2"]);
  });
});
