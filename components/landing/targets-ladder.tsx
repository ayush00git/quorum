"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { getMeta, getRunIndex } from "@/lib/api";
import type { RunIndexEntry, TargetInfo } from "@/lib/types";

const LEVEL: Record<string, number> = { "n/a": 0, easy: 1, medium: 2, hard: 3, "very hard": 4 };

function Difficulty({ value }: { value: string }) {
  const level = LEVEL[value] ?? 0;
  return (
    <span className="inline-flex items-center gap-2">
      <span className="inline-flex gap-0.5" aria-hidden="true">
        {[1, 2, 3, 4].map((i) => (
          <span key={i} className={`size-2 border border-fg ${i <= level ? "bg-fg" : ""}`} />
        ))}
      </span>
      <span className="text-muted">{value}</span>
    </span>
  );
}

type State = { status: "loading" } | { status: "ready"; targets: TargetInfo[]; runs: RunIndexEntry[] } | { status: "offline"; runs: RunIndexEntry[] };

export function TargetsLadder() {
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    const ac = new AbortController();
    const runs = getRunIndex(ac.signal).catch(() => []);
    getMeta(ac.signal).then(
      async (meta) => setState({ status: "ready", targets: meta.targets, runs: await runs }),
      async () => {
        if (!ac.signal.aborted) setState({ status: "offline", runs: await runs });
      },
    );
    return () => ac.abort();
  }, []);

  const runs = state.status === "loading" ? [] : state.runs;
  const targets: TargetInfo[] =
    state.status === "ready"
      ? state.targets
      : [...new Set(runs.map((r) => r.target))].sort().map((codename) => ({ codename, difficulty: "" }));

  if (state.status === "loading") {
    return <p className="label animate-pulse py-6" role="status">Loading targets</p>;
  }
  if (targets.length === 0) {
    return (
      <p className="py-6 text-[13px] text-muted">
        Targets load from the Go backend. Start it with <code className="font-mono text-fg">make dev</code>.
      </p>
    );
  }

  return (
    <div className="relative overflow-x-auto">
      <table className="w-full min-w-[520px] border-collapse text-[13px]">
        <thead>
          <tr className="border-b border-border-strong text-left">
            <th scope="col" className="label py-2 pr-4 font-normal">Target</th>
            <th scope="col" className="label py-2 pr-4 font-normal">Difficulty</th>
            <th scope="col" className="label py-2 pr-4 font-normal">Recorded run</th>
            <th scope="col" className="label py-2 font-normal"><span className="sr-only">Replay</span></th>
          </tr>
        </thead>
        <tbody className="font-mono text-xs tabular-nums">
          {targets.map((t) => {
            const run = runs.find((r) => r.target === t.codename && r.found) ?? runs.find((r) => r.target === t.codename);
            return (
              <tr key={t.codename} className="border-b border-border">
                <td className="py-3 pr-4 text-fg">{t.codename}</td>
                <td className="py-3 pr-4">{t.difficulty ? <Difficulty value={t.difficulty} /> : <span className="text-subtle">–</span>}</td>
                <td className="py-3 pr-4">
                  {run ? (
                    <span className="inline-flex items-center gap-2">
                      <span aria-hidden="true" className={`size-2.5 border border-fg ${run.found ? "bg-fg" : ""}`} />
                      {run.found ? `violation in ${run.steps} steps` : `concluded clean in ${run.steps} steps`}
                    </span>
                  ) : (
                    <span className="text-subtle">not recorded</span>
                  )}
                </td>
                <td className="py-3 text-right">
                  {run ? (
                    <Link href={`/replay/${run.id}`} className="text-muted underline-offset-2 hover:text-fg hover:underline">
                      replay
                    </Link>
                  ) : null}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
