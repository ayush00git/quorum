"use client";

import { useId } from "react";
import { HatchDefs } from "@/components/ui/hatch";
import { type ClusterState, type NodeState, connected } from "@/lib/cluster";

const W = 320;
const H = 250;
const CX = W / 2;
const CY = 118;
const RING = 88;
const R = 19;

function position(i: number, count: number) {
  const a = -Math.PI / 2 + (i / count) * Math.PI * 2;
  return { x: CX + RING * Math.cos(a), y: CY + RING * Math.sin(a) };
}

function describe(n: NodeState): string {
  const parts = [n.id, n.role, `term ${n.term}`];
  if (n.crashed) parts.push("crashed");
  if (n.paused) parts.push("paused");
  return parts.join(", ");
}

function NodeGlyph({ n, x, y, hatch }: { n: NodeState; x: number; y: number; hatch: string }) {
  const leader = n.role === "leader" && !n.crashed;
  return (
    <g aria-label={describe(n)} role="img">
      <circle
        cx={x}
        cy={y}
        r={R}
        className={leader ? "fill-fg stroke-fg" : n.crashed ? "fill-bg stroke-subtle" : "fill-bg stroke-fg"}
        strokeWidth={1}
        strokeDasharray={n.role === "candidate" && !n.crashed ? "3 2" : undefined}
      />
      {n.paused && !n.crashed ? <circle cx={x} cy={y} r={R - 1} fill={`url(#${hatch})`} /> : null}
      {n.crashed ? (
        <path
          d={`M${x - 12} ${y - 12}L${x + 12} ${y + 12}M${x + 12} ${y - 12}L${x - 12} ${y + 12}`}
          className="stroke-fg"
          strokeWidth={1.25}
        />
      ) : null}
      <text
        x={x}
        y={y + 4}
        textAnchor="middle"
        className={`font-mono text-[11px] font-medium ${leader ? "fill-bg" : n.crashed ? "fill-subtle" : "fill-fg"}`}
        stroke={n.paused && !n.crashed ? "var(--bg)" : undefined}
        strokeWidth={n.paused && !n.crashed ? 3 : 0}
        paintOrder="stroke"
      >
        {n.id}
      </text>
      <text x={x} y={y + R + 13} textAnchor="middle" className="fill-muted font-mono text-[10px] tabular-nums">
        {n.crashed ? "down" : `t${n.term}${leader ? " L" : n.role === "candidate" ? " C" : ""}`}
      </text>
    </g>
  );
}

export function ClusterView({ state, atMs }: { state: ClusterState; atMs: number }) {
  const hatch = useId();
  const regionHatch = useId();
  const pos = new Map(state.nodes.map((n, i) => [n.id, position(i, state.nodes.length)]));
  const crashed = new Set(state.nodes.filter((n) => n.crashed).map((n) => n.id));
  const leaders = state.nodes.filter((n) => n.role === "leader" && !n.crashed);

  const links: { a: string; b: string; up: boolean }[] = [];
  state.nodes.forEach((a, i) => {
    for (const b of state.nodes.slice(i + 1)) {
      const up = connected(state.groups, a.id, b.id) && !crashed.has(a.id) && !crashed.has(b.id);
      links.push({ a: a.id, b: b.id, up });
    }
  });

  const summary = [
    leaders.length === 0
      ? "no leader"
      : leaders.map((l) => `${l.id} leads term ${l.term}`).join(", "),
    state.groups ? `partitioned ${state.groups.map((g) => g.join(",")).join(" | ")}` : "network whole",
  ].join("; ");

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="block h-auto w-full" role="group" aria-label={`Cluster at ${atMs} ms: ${summary}`}>
      <HatchDefs id={hatch} className="stroke-muted" gap={3} />
      <HatchDefs id={regionHatch} className="stroke-border-strong" gap={6} />

      {state.groups?.map((g, gi) => {
        // Ring order keeps the hull convex.
        const pts = state.nodes
          .filter((n) => g.includes(n.id))
          .map((n) => pos.get(n.id))
          .filter((p): p is { x: number; y: number } => !!p);
        if (pts.length === 0) return null;
        const d =
          pts.length === 1
            ? ""
            : `M${pts.map((p) => `${p.x} ${p.y}`).join("L")}${pts.length > 2 ? "Z" : ""}`;
        return pts.length === 1 ? (
          <circle key={gi} cx={pts[0]?.x} cy={pts[0]?.y} r={R + 14} fill={`url(#${regionHatch})`} className="stroke-border-strong" strokeDasharray="2 3" />
        ) : (
          <path
            key={gi}
            d={d}
            fill={`url(#${regionHatch})`}
            stroke={`url(#${regionHatch})`}
            strokeWidth={(R + 14) * 2}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        );
      })}

      {links.map(({ a, b, up }) => {
        const pa = pos.get(a);
        const pb = pos.get(b);
        if (!pa || !pb) return null;
        return (
          <line
            key={`${a}-${b}`}
            x1={pa.x}
            y1={pa.y}
            x2={pb.x}
            y2={pb.y}
            className={up ? "stroke-border-strong" : "stroke-border"}
            strokeDasharray={up ? undefined : "2 4"}
          />
        );
      })}

      {state.nodes.map((n) => {
        const p = pos.get(n.id);
        return p ? <NodeGlyph key={n.id} n={n} x={p.x} y={p.y} hatch={hatch} /> : null;
      })}
    </svg>
  );
}

export function ClusterLegend() {
  const items: { label: string; glyph: React.ReactNode }[] = [
    { label: "leader", glyph: <circle cx="6" cy="6" r="5" className="fill-fg" /> },
    { label: "follower", glyph: <circle cx="6" cy="6" r="4.5" className="fill-none stroke-fg" /> },
    { label: "candidate", glyph: <circle cx="6" cy="6" r="4.5" className="fill-none stroke-fg" strokeDasharray="2 1.5" /> },
    {
      label: "crashed",
      glyph: (
        <g>
          <circle cx="6" cy="6" r="4.5" className="fill-none stroke-subtle" />
          <path d="M2.5 2.5L9.5 9.5M9.5 2.5L2.5 9.5" className="stroke-fg" />
        </g>
      ),
    },
    {
      label: "paused",
      glyph: (
        <g>
          <circle cx="6" cy="6" r="4.5" className="fill-none stroke-fg" />
          <path d="M3 8L8 3M4.5 9.5L9.5 4.5" className="stroke-muted" />
        </g>
      ),
    },
    { label: "partitioned", glyph: <line x1="0" y1="6" x2="12" y2="6" className="stroke-fg" strokeDasharray="2 2" /> },
  ];
  return (
    <ul className="flex flex-wrap gap-x-3 gap-y-1" aria-label="Cluster legend">
      {items.map((i) => (
        <li key={i.label} className="flex items-center gap-1.5 font-mono text-2xs text-muted">
          <svg viewBox="0 0 12 12" width="12" height="12" aria-hidden="true">
            {i.glyph}
          </svg>
          {i.label}
        </li>
      ))}
    </ul>
  );
}
