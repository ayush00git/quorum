"use client";

import { useEffect, useRef, useState } from "react";
import { VerdictGlyph } from "@/components/ui/verdict-glyph";
import type { LogEntry } from "@/lib/agent-log";

const TEXT_CHIP: Record<string, string> = {
  Hypothesis: "border-fg bg-inverse text-inverse-fg",
  Learned: "border-fg text-fg",
  Note: "border-border-strong text-muted",
};

function StepNo({ step }: { step: number }) {
  return <span className="font-mono text-2xs tabular-nums text-subtle">s{String(step + 1).padStart(2, "0")}</span>;
}

function ToolEntry({
  entry,
  selectedId,
  onSelectExperiment,
}: {
  entry: Extract<LogEntry, { kind: "tool" }>;
  selectedId: string | null;
  onSelectExperiment: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [full, setFull] = useState(false);
  const lines = entry.result?.split("\n") ?? [];
  const clipped = !full && lines.length > 10;
  const exp = entry.experiment;

  return (
    <li className="flex flex-col gap-1.5">
      <div className="flex items-center gap-2">
        <StepNo step={entry.step} />
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          aria-expanded={open}
          className="group inline-flex min-w-0 items-center gap-1.5 font-mono text-xs text-fg"
        >
          <svg viewBox="0 0 8 8" className={`size-2 shrink-0 transition-transform duration-150 ${open ? "rotate-90" : ""}`} aria-hidden="true">
            <path d="M2 1l4 3-4 3z" className="fill-current" />
          </svg>
          <span className="truncate group-hover:underline">{entry.name}</span>
          <span className="text-subtle">()</span>
        </button>
        {exp ? (
          <button
            type="button"
            onClick={() => onSelectExperiment(exp.id)}
            aria-pressed={selectedId === exp.id}
            className={`ml-auto inline-flex h-5 shrink-0 items-center gap-1.5 rounded-sm border px-1.5 font-mono text-2xs transition-colors duration-150 ${
              selectedId === exp.id ? "border-fg text-fg" : "border-border text-muted hover:border-border-strong hover:text-fg"
            }`}
          >
            <VerdictGlyph verdict={exp.verdict} size={8} />
            {exp.id}
          </button>
        ) : null}
      </div>
      {open ? (
        <pre className="max-h-64 overflow-auto rounded border border-border bg-surface p-2 font-mono text-[11px] leading-4 text-muted">
          {JSON.stringify(entry.input, null, 2)}
        </pre>
      ) : null}
      {entry.result !== null ? (
        <div className={`border-l pl-3 ${entry.isError ? "border-dashed border-fg" : "border-border-strong"}`}>
          {entry.isError ? <p className="label mb-1 text-fg">Rejected</p> : null}
          <pre className="whitespace-pre-wrap break-words font-mono text-[11px] leading-4 text-muted">
            {clipped ? lines.slice(0, 10).join("\n") : entry.result}
          </pre>
          {lines.length > 10 ? (
            <button type="button" onClick={() => setFull((f) => !f)} className="mt-1 font-mono text-2xs text-subtle hover:text-fg">
              {full ? "show less" : `show ${lines.length - 10} more lines`}
            </button>
          ) : null}
        </div>
      ) : (
        <p className="label animate-pulse pl-3">running</p>
      )}
    </li>
  );
}

export function AgentLog({
  entries,
  status,
  footer,
  empty,
  selectedId,
  onSelectExperiment,
}: {
  entries: readonly LogEntry[];
  status?: React.ReactNode;
  footer?: React.ReactNode;
  empty: React.ReactNode;
  selectedId: string | null;
  onSelectExperiment: (id: string) => void;
}) {
  const scroller = useRef<HTMLDivElement | null>(null);
  const pinned = useRef(true);

  useEffect(() => {
    const el = scroller.current;
    if (el && pinned.current) el.scrollTop = el.scrollHeight;
  }, [entries.length, footer, status]);

  return (
    <div
      ref={scroller}
      className="min-h-0 flex-1 overflow-y-auto"
      onScroll={(e) => {
        const el = e.currentTarget;
        pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
      }}
    >
      {entries.length === 0 && !footer ? (
        empty
      ) : (
        <ol className="flex flex-col gap-5 p-3" aria-live="polite" aria-relevant="additions">
          {entries.map((e) =>
            e.kind === "text" ? (
              <li key={e.key} className="flex flex-col gap-1.5">
                <div className="flex items-center gap-2">
                  <StepNo step={e.step} />
                  <span className={`rounded-sm border px-1.5 font-mono text-[10px] uppercase leading-4 tracking-wider ${TEXT_CHIP[e.label]}`}>
                    {e.label}
                  </span>
                </div>
                <p className="whitespace-pre-wrap text-[13px] leading-5 text-fg">{e.text}</p>
              </li>
            ) : (
              <ToolEntry key={e.key} entry={e} selectedId={selectedId} onSelectExperiment={onSelectExperiment} />
            ),
          )}
        </ol>
      )}
      {status ? <div className="px-3 pb-3">{status}</div> : null}
      {footer}
    </div>
  );
}
