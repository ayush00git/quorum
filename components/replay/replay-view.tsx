"use client";

import { useMemo, useState } from "react";
import { AgentLog } from "@/components/lab/agent-log";
import { ExperimentWorkspace } from "@/components/lab/experiment-workspace";
import { FindingPanel } from "@/components/lab/finding-panel";
import { LabShell } from "@/components/lab/lab-shell";
import { EmptyState } from "@/components/ui/panel";
import { buildLog } from "@/lib/agent-log";
import type { RecordedRun } from "@/lib/types";
import { useReplay } from "@/lib/use-replay";
import { ReplayRail } from "./replay-rail";

export function ReplayView({ run }: { run: RecordedRun }) {
  const r = useReplay(run);
  const [picked, setPicked] = useState<string | null>(null);
  const entries = useMemo(() => buildLog(r.steps), [r.steps]);

  const pickedVisible = picked && r.experiments.some((e) => e.id === picked) ? picked : null;
  const findingId = r.finished ? (run.finding?.experimentId ?? null) : null;
  const selectedId = pickedVisible ?? findingId ?? r.experiments.at(-1)?.id ?? null;

  return (
    <LabShell
      railSummary={`${run.target} · step ${r.shown}/${r.total} · ${r.speed}x${r.playing ? "" : " · paused"}`}
      rail={
        <ReplayRail
          run={run}
          shown={r.shown}
          total={r.total}
          playing={r.playing}
          finished={r.finished}
          speed={r.speed}
          onSpeed={r.setSpeed}
          onToggle={r.toggle}
          onBack={r.stepBack}
          onForward={r.stepForward}
          verify={r.verify}
          experimentCount={r.experiments.length}
        />
      }
      workspace={(tab) => (
        <ExperimentWorkspace
          experiments={r.experiments}
          selectedId={selectedId}
          onSelect={setPicked}
          tab={tab}
          empty={<EmptyState title="No experiments yet">Experiments appear as the replay reaches them.</EmptyState>}
        />
      )}
      logTitle={
        <>
          <h2 className="label">Agent · replay</h2>
          <span className="ml-auto font-mono text-2xs tabular-nums text-subtle">
            {r.shown} / {r.total} steps
          </span>
        </>
      }
      log={
        <AgentLog
          entries={entries}
          selectedId={selectedId}
          onSelectExperiment={setPicked}
          empty={<EmptyState title="Starting replay">Claude&apos;s first message is on its way.</EmptyState>}
          footer={
            r.finished ? (
              <FindingPanel
                finding={run.finding}
                conclusion={run.conclusion}
                revealedBug={run.revealedBug || null}
                onSelectExperiment={setPicked}
              />
            ) : null
          }
        />
      }
    />
  );
}
