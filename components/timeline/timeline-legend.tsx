"use client";

import { useId } from "react";
import { HatchDefs } from "@/components/ui/hatch";

type Kind = "ok" | "offending" | "conflicting" | "info" | "fail" | "fault";

function Swatch({ kind, hatchId }: { kind: Kind; hatchId: string }) {
  return (
    <svg viewBox="0 0 18 10" width="18" height="10" aria-hidden="true" className="shrink-0">
      {kind === "fault" ? (
        <line x1="9" y1="0" x2="9" y2="10" className="stroke-fg" strokeDasharray="2 2" />
      ) : (
        <rect
          x="0.5"
          y="1.5"
          width="17"
          height="7"
          className={
            kind === "offending"
              ? "fill-fg stroke-fg"
              : kind === "fail"
                ? "fill-none stroke-subtle"
                : kind === "info"
                  ? "stroke-fg"
                  : "fill-bg stroke-fg"
          }
          fill={kind === "info" ? `url(#${hatchId})` : undefined}
          strokeWidth={kind === "conflicting" ? 2 : 1}
          strokeDasharray={kind === "info" ? "3 2" : kind === "fail" ? "1 2" : undefined}
        />
      )}
    </svg>
  );
}

const ITEMS = [
  { kind: "ok", label: "ok" },
  { kind: "offending", label: "not linearizable" },
  { kind: "conflicting", label: "conflicts with" },
  { kind: "info", label: "indeterminate" },
  { kind: "fail", label: "failed" },
  { kind: "fault", label: "fault" },
] as const;

export function TimelineLegend() {
  const hatchId = useId();
  return (
    <div className="relative">
      <svg width="0" height="0" className="absolute" aria-hidden="true">
        <HatchDefs id={hatchId} className="stroke-muted" gap={3} />
      </svg>
      <ul className="flex flex-wrap items-center gap-x-4 gap-y-1.5" aria-label="Timeline legend">
      {ITEMS.map((i) => (
        <li key={i.kind} className="flex items-center gap-1.5 font-mono text-2xs text-muted">
          <Swatch kind={i.kind} hatchId={hatchId} />
          {i.label}
        </li>
        ))}
      </ul>
    </div>
  );
}
