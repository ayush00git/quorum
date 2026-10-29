// Derives the cluster picture at a point in virtual time from a run's event
// log (roles, terms, crashes, pauses) and its fault schedule (partitions).

import type { Fault, SimEvent } from "./types";

export type NodeRole = "leader" | "candidate" | "follower";

export interface NodeState {
  id: string;
  role: NodeRole;
  term: number;
  crashed: boolean;
  paused: boolean;
}

export interface ClusterState {
  nodes: NodeState[];
  /** Partition groups in effect, or null when the network is whole. */
  groups: string[][] | null;
}

export function nodeIds(count: number): string[] {
  return Array.from({ length: count }, (_, i) => `n${i + 1}`);
}

export function groupsAt(
  faults: readonly Fault[],
  ids: readonly string[],
  atMs: number,
): string[][] | null {
  let groups: string[][] | null = null;
  const applied = faults
    .map((f, i) => ({ f, i }))
    .filter(({ f }) => f.atMs <= atMs)
    .sort((a, b) => a.f.atMs - b.f.atMs || a.i - b.i);
  for (const { f } of applied) {
    if (f.action === "heal") groups = null;
    else if (f.action === "isolate" && f.args.node) {
      const node = f.args.node;
      groups = [[node], ids.filter((id) => id !== node)];
    } else if (f.action === "partition" && f.args.groups) {
      const listed = new Set(f.args.groups.flat());
      const rest = ids.filter((id) => !listed.has(id));
      groups = rest.length > 0 ? [...f.args.groups, rest] : f.args.groups.map((g) => [...g]);
    }
  }
  return groups;
}

export function connected(groups: string[][] | null, a: string, b: string): boolean {
  if (!groups) return true;
  return groups.some((g) => g.includes(a) && g.includes(b));
}

export function clusterAt(
  nodeCount: number,
  events: readonly SimEvent[],
  faults: readonly Fault[],
  atMs: number,
): ClusterState {
  const ids = nodeIds(nodeCount);
  const nodes: NodeState[] = ids.map((id) => ({
    id,
    role: "follower",
    term: 0,
    crashed: false,
    paused: false,
  }));
  const state = new Map(nodes.map((n) => [n.id, n]));

  for (const e of events) {
    if (e.atMs > atMs) break;
    const n = e.node ? state.get(e.node) : undefined;
    if (!n) continue;
    if (e.term !== undefined && e.term > n.term) n.term = e.term;
    switch (e.kind) {
      case "election-started":
        n.role = "candidate";
        break;
      case "became-leader":
        n.role = "leader";
        // Candidates in this or an older term lose the election.
        for (const other of nodes) {
          if (other !== n && other.role === "candidate" && other.term <= n.term) {
            other.role = "follower";
          }
        }
        break;
      case "stepped-down":
      case "vote-granted":
        n.role = "follower";
        break;
      case "crash":
        n.crashed = true;
        n.paused = false;
        n.role = "follower";
        break;
      case "restart":
        n.crashed = false;
        n.role = "follower";
        break;
      case "pause-start":
        n.paused = true;
        break;
      case "pause-end":
        n.paused = false;
        break;
      default:
        break;
    }
  }

  return { nodes, groups: groupsAt(faults, ids, atMs) };
}
