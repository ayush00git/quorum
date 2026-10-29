"use client";

import { useState } from "react";
import { VerdictBadge } from "@/components/ui/verdict-glyph";
import { faultLabel, formatMs } from "@/lib/format";
import type { Finding } from "@/lib/types";

export function FindingPanel({
  finding,
  conclusion,
  revealedBug,
  onSelectExperiment,
  onExport,
  exporting = false,
}: {
  finding: Finding | null;
  conclusion: string | null;
  revealedBug: string | null;
  onSelectExperiment: (id: string) => void;
  onExport?: () => void;
  exporting?: boolean;
}) {
  const [revealed, setRevealed] = useState(false);
  if (!finding && !conclusion) return null;

  return (
    <section aria-labelledby="finding-title" className="m-3 rounded border border-fg">
      <header className="flex items-center gap-2 border-b border-fg px-3 py-2">
        <h2 id="finding-title" className="label text-fg">
          {finding ? "Finding" : "Conclusion"}
        </h2>
        <span className="ml-auto">
          <VerdictBadge verdict={finding ? "violation" : "ok"} />
        </span>
      </header>
      <div className="flex flex-col gap-4 p-3">
        {finding ? (
          <>
            <div className="flex flex-col gap-1.5">
              <h3 className="text-[15px] font-semibold leading-5 tracking-tight">{finding.title}</h3>
              <p className="text-[13px] leading-5 text-muted">{finding.anomaly}</p>
            </div>
            <div>
              <p className="label mb-2">Causal chain</p>
              <ol className="flex flex-col gap-2">
                {finding.causalChain.map((c, i) => (
                  <li key={i} className="grid grid-cols-[1.5rem_1fr] gap-1 text-[13px] leading-5">
                    <span className="font-mono text-xs tabular-nums text-subtle">{String(i + 1).padStart(2, "0")}</span>
                    <span>{c}</span>
                  </li>
                ))}
              </ol>
            </div>
            <div>
              <div className="mb-2 flex items-center gap-2">
                <p className="label">Minimal fault schedule</p>
                <button
                  type="button"
                  onClick={() => onSelectExperiment(finding.experimentId)}
                  className="ml-auto font-mono text-2xs text-muted underline-offset-2 hover:text-fg hover:underline"
                >
                  view {finding.experimentId}
                </button>
              </div>
              <ul className="rounded border border-border font-mono text-xs tabular-nums">
                {finding.spec.faults.length === 0 ? (
                  <li className="px-2 py-1.5 text-muted">no faults needed</li>
                ) : (
                  finding.spec.faults.map((f, i) => (
                    <li key={i} className="flex gap-3 border-t border-border px-2 py-1.5 first:border-t-0">
                      <span className="w-16 shrink-0 text-right text-subtle">{formatMs(f.atMs)}</span>
                      <span>{faultLabel(f)}</span>
                    </li>
                  ))
                )}
              </ul>
              <p className="mt-1.5 font-mono text-2xs tabular-nums text-subtle">
                seed {finding.spec.seed} · {formatMs(finding.spec.durationMs)} · {finding.spec.clients} clients · {finding.spec.keys} keys · reads from {finding.spec.readFrom}
              </p>
            </div>
          </>
        ) : (
          <div className="flex flex-col gap-1.5">
            <h3 className="text-[15px] font-semibold leading-5 tracking-tight">No violation found</h3>
            <p className="whitespace-pre-wrap text-[13px] leading-5 text-muted">{conclusion}</p>
          </div>
        )}

        {revealed && revealedBug ? (
          <div className="rounded bg-inverse px-3 py-2 text-inverse-fg" role="status">
            <p className="font-mono text-2xs uppercase tracking-wider opacity-70">Planted bug</p>
            <p className="font-mono text-sm font-medium">{revealedBug}</p>
          </div>
        ) : null}

        <div className="flex flex-wrap gap-2">
          {revealedBug && !revealed ? (
            <button type="button" className="btn btn-primary" onClick={() => setRevealed(true)}>
              Reveal planted bug
            </button>
          ) : null}
          {onExport ? (
            <button type="button" className="btn" onClick={onExport} disabled={exporting}>
              {exporting ? "Exporting" : "Export run"}
            </button>
          ) : null}
        </div>
      </div>
    </section>
  );
}
