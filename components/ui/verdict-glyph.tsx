import { verdictText } from "@/lib/format";
import type { Verdict } from "@/lib/types";

/** ok: outlined square. violation: solid block. unknown: dashed outline. */
export function VerdictGlyph({ verdict, size = 10 }: { verdict: Verdict; size?: number }) {
  return (
    <svg
      viewBox="0 0 10 10"
      width={size}
      height={size}
      role="img"
      aria-label={verdictText(verdict)}
      className="shrink-0"
    >
      {verdict === "violation" ? (
        <rect x="0" y="0" width="10" height="10" className="fill-fg" />
      ) : (
        <rect
          x="0.5"
          y="0.5"
          width="9"
          height="9"
          className="fill-none stroke-fg"
          strokeWidth="1"
          strokeDasharray={verdict === "unknown" ? "2 1.5" : undefined}
        />
      )}
    </svg>
  );
}

export function VerdictBadge({ verdict }: { verdict: Verdict }) {
  const inverted = verdict === "violation";
  return (
    <span
      className={`inline-flex h-5 items-center gap-1.5 rounded-sm border px-1.5 font-mono text-2xs uppercase tracking-wider ${
        inverted
          ? "border-inverse bg-inverse text-inverse-fg"
          : verdict === "unknown"
            ? "border-dashed border-border-strong text-muted"
            : "border-border-strong text-fg"
      }`}
    >
      {verdictText(verdict)}
    </span>
  );
}
