"use client";

import { Field, RailSection, UsageMeter } from "@/components/lab/rail";
import type { RecordedRun } from "@/lib/types";
import type { Speed, VerifyStatus } from "@/lib/use-replay";

const SPEEDS: Speed[] = [1, 2, 4];

export function VerifyIndicator({ verify, total }: { verify: Record<string, VerifyStatus>; total: number }) {
  const values = Object.values(verify);
  const matched = values.filter((v) => v === "match").length;
  const mismatched = values.filter((v) => v === "mismatch").length;
  const offline = values.some((v) => v === "offline");
  const failed = values.some((v) => v === "error");
  const pending = total - values.length;

  let tone: "ok" | "bad" | "warn" | "wait" = "wait";
  let text = "waiting for the first experiment";
  if (mismatched > 0) {
    tone = "bad";
    text = `history hash mismatch on ${mismatched} of ${total}`;
  } else if (offline) {
    tone = "warn";
    text = "backend offline, replay not verified";
  } else if (failed) {
    tone = "warn";
    text = "some experiments could not be re-run";
  } else if (matched > 0) {
    tone = "ok";
    text = `deterministic replay verified · ${matched}/${total}${pending > 0 ? " · checking" : ""}`;
  } else if (total > 0) {
    text = "re-running experiments";
  }

  return (
    <p className="flex items-start gap-2 font-mono text-2xs leading-4" role="status">
      <span
        aria-hidden="true"
        className={`mt-0.5 inline-block size-2.5 shrink-0 border border-fg ${
          tone === "ok" ? "bg-fg" : tone === "bad" ? "hatch text-fg" : tone === "warn" ? "border-dashed" : "animate-pulse"
        }`}
      />
      <span className={tone === "wait" ? "text-muted" : "text-fg"}>{text}</span>
    </p>
  );
}

function IconButton({ label, onClick, disabled, children }: { label: string; onClick: () => void; disabled?: boolean; children: React.ReactNode }) {
  return (
    <button type="button" className="btn px-0" onClick={onClick} disabled={disabled} aria-label={label} title={label}>
      <svg viewBox="0 0 12 12" className="size-3" aria-hidden="true">
        {children}
      </svg>
    </button>
  );
}

export function ReplayRail({
  run,
  shown,
  total,
  playing,
  finished,
  speed,
  onSpeed,
  onToggle,
  onBack,
  onForward,
  verify,
  experimentCount,
}: {
  run: RecordedRun;
  shown: number;
  total: number;
  playing: boolean;
  finished: boolean;
  speed: Speed;
  onSpeed: (s: Speed) => void;
  onToggle: () => void;
  onBack: () => void;
  onForward: () => void;
  verify: Record<string, VerifyStatus>;
  experimentCount: number;
}) {
  const started = new Date(run.startedAt);
  return (
    <div className="flex flex-col">
      <RailSection title="Recorded run">
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 text-xs">
          {[
            ["target", run.target],
            ["model", run.model],
            ["recorded", Number.isNaN(started.getTime()) ? run.startedAt : started.toISOString().slice(0, 16).replace("T", " ")],
            ["steps", String(total)],
            ["experiments", String(run.experiments.length)],
          ].map(([k, v]) => (
            <div key={k} className="contents">
              <dt className="text-muted">{k}</dt>
              <dd className="truncate text-right font-mono tabular-nums">{v}</dd>
            </div>
          ))}
        </dl>
      </RailSection>

      <RailSection title="Playback">
        <div className="grid grid-cols-3 gap-1.5">
          <IconButton label="Step back" onClick={onBack} disabled={shown === 0}>
            <path d="M3 2v8M10 2L4.5 6 10 10z" className="fill-current stroke-current" />
          </IconButton>
          <IconButton label={playing ? "Pause" : finished ? "Replay from start" : "Play"} onClick={onToggle}>
            {playing ? <path d="M3 2h2v8H3zM7 2h2v8H7z" className="fill-current" /> : <path d="M3 1.5L10 6 3 10.5z" className="fill-current" />}
          </IconButton>
          <IconButton label="Step forward" onClick={onForward} disabled={finished}>
            <path d="M9 2v8M2 2l5.5 4L2 10z" className="fill-current stroke-current" />
          </IconButton>
        </div>
        <Field label="Speed" htmlFor="speed-1">
          <div role="radiogroup" aria-label="Playback speed" className="grid grid-cols-3 gap-1.5">
            {SPEEDS.map((s) => (
              <button
                key={s}
                id={`speed-${s}`}
                type="button"
                role="radio"
                aria-checked={speed === s}
                onClick={() => onSpeed(s)}
                className={`btn font-mono ${speed === s ? "btn-primary" : ""}`}
              >
                {s}x
              </button>
            ))}
          </div>
        </Field>
        <div className="flex flex-col gap-1.5">
          <div className="h-1 w-full overflow-hidden rounded-sm border border-border" aria-hidden="true">
            <div className="h-full bg-fg transition-[width] duration-200" style={{ width: `${total ? (shown / total) * 100 : 0}%` }} />
          </div>
          <p className="font-mono text-2xs tabular-nums text-subtle">
            step {shown} / {total}
          </p>
        </div>
      </RailSection>

      <RailSection title="Determinism">
        <VerifyIndicator verify={verify} total={experimentCount} />
        <p className="text-2xs leading-4 text-subtle">
          Each recorded experiment is re-executed by the simulator and its history hash compared with the recording.
        </p>
      </RailSection>

      <RailSection title="Tokens">
        <UsageMeter usage={run.usage} />
        <p className="font-mono text-2xs text-subtle">totals for the whole recorded run</p>
      </RailSection>
    </div>
  );
}
