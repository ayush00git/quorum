import { formatMs } from "@/lib/format";
import type { Viewport } from "@/lib/timeline";

function ToolButton({ label, onClick, children }: { label: string; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="inline-flex size-6 items-center justify-center rounded-sm border border-border font-mono text-xs text-muted transition-colors duration-150 hover:border-border-strong hover:text-fg"
    >
      {children}
    </button>
  );
}

export function TimelineToolbar({
  viewport,
  onZoomIn,
  onZoomOut,
  onFit,
}: {
  viewport: Viewport;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFit: () => void;
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="mr-1 hidden font-mono text-2xs tabular-nums text-subtle sm:inline">
        {formatMs(viewport.start)} – {formatMs(viewport.end)}
      </span>
      <ToolButton label="Zoom in" onClick={onZoomIn}>+</ToolButton>
      <ToolButton label="Zoom out" onClick={onZoomOut}>−</ToolButton>
      <ToolButton label="Fit whole history" onClick={onFit}>
        <svg viewBox="0 0 10 10" className="size-2.5" aria-hidden="true">
          <path d="M0.5 3V0.5H3M7 0.5H9.5V3M9.5 7V9.5H7M3 9.5H0.5V7" className="fill-none stroke-current" />
        </svg>
      </ToolButton>
    </div>
  );
}

export function TimeScrubber({
  value,
  max,
  onChange,
}: {
  value: number;
  max: number;
  onChange: (ms: number) => void;
}) {
  return (
    <div className="flex items-center gap-3">
      <span className="label shrink-0">t</span>
      <input
        type="range"
        className="range h-4 flex-1"
        min={0}
        max={max}
        step={1}
        value={Math.round(value)}
        onChange={(e) => onChange(Number(e.target.value))}
        aria-label="Time scrubber"
        aria-valuetext={formatMs(value)}
      />
      <span className="w-16 shrink-0 text-right font-mono text-xs tabular-nums">{formatMs(value)}</span>
    </div>
  );
}
