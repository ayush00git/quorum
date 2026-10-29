"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState } from "@/components/ui/panel";
import { getRunIndex } from "@/lib/api";
import type { RunIndexEntry } from "@/lib/types";
import { RecordHint } from "./record-hint";

export function RunIndex() {
  const [runs, setRuns] = useState<RunIndexEntry[] | null>(null);

  useEffect(() => {
    const ac = new AbortController();
    getRunIndex(ac.signal).then(setRuns, () => !ac.signal.aborted && setRuns([]));
    return () => ac.abort();
  }, []);

  if (runs === null) return <p className="label animate-pulse" role="status">Loading runs</p>;
  if (runs.length === 0) {
    return (
      <EmptyState title="No recorded runs yet" className="rounded border border-border">
        <RecordHint />
      </EmptyState>
    );
  }

  return (
    <ul className="divide-y divide-border rounded border border-border">
      {runs.map((r) => (
        <li key={r.id}>
          <Link
            href={`/replay/${r.id}`}
            className="flex items-center gap-4 px-4 py-3 font-mono text-xs tabular-nums transition-colors duration-150 hover:bg-surface"
          >
            <span aria-hidden="true" className={`size-2.5 border border-fg ${r.found ? "bg-fg" : ""}`} />
            <span className="text-fg">{r.id}</span>
            <span className="text-muted">{r.target}</span>
            <span className="ml-auto text-muted">
              {r.found ? "violation" : "no violation"} · {r.steps} steps
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
