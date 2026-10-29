import { formatCount } from "@/lib/format";
import type { Usage } from "@/lib/types";

export function RailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2.5 border-b border-border p-4">
      <h3 className="label">{title}</h3>
      {children}
    </div>
  );
}

export function Field({ label, htmlFor, children, hint }: { label: string; htmlFor: string; children: React.ReactNode; hint?: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={htmlFor} className="text-xs text-muted">
        {label}
      </label>
      {children}
      {hint ? <p className="text-2xs leading-4 text-subtle">{hint}</p> : null}
    </div>
  );
}

export function UsageMeter({ usage }: { usage: Usage }) {
  const rows: [string, number][] = [
    ["input", usage.inputTokens],
    ["output", usage.outputTokens],
    ["cache read", usage.cacheReadTokens],
    ["cache write", usage.cacheCreationTokens],
  ];
  return (
    <dl className="grid grid-cols-2 gap-px overflow-hidden rounded border border-border bg-border">
      {rows.map(([k, v]) => (
        <div key={k} className="flex flex-col gap-0.5 bg-bg px-2.5 py-2">
          <dt className="font-mono text-[10px] uppercase tracking-wider text-subtle">{k}</dt>
          <dd className="font-mono text-sm tabular-nums">{formatCount(v)}</dd>
        </div>
      ))}
    </dl>
  );
}

export function StatusLine({ tone, children }: { tone: "idle" | "busy" | "done" | "error"; children: React.ReactNode }) {
  return (
    <p className="flex items-start gap-2 text-xs leading-4" role="status">
      <span
        aria-hidden="true"
        className={`mt-1 inline-block size-2 shrink-0 ${
          tone === "done"
            ? "bg-fg"
            : tone === "error"
              ? "hatch border border-fg text-fg"
              : tone === "busy"
                ? "animate-pulse border border-fg"
                : "border border-border-strong"
        }`}
      />
      <span className={tone === "idle" ? "text-muted" : "text-fg"}>{children}</span>
    </p>
  );
}
