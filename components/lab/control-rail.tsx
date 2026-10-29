"use client";

import { useEffect, useState } from "react";
import type { Meta } from "@/lib/types";
import type { RunState } from "@/lib/use-agent-run";
import { Field, RailSection, StatusLine, UsageMeter } from "./rail";

export interface RunForm {
  target: string;
  seed: number;
  maxSteps: number;
  accessCode: string;
}

function useCountdown(until: number | null): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (until === null) return;
    const t = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(t);
  }, [until]);
  return until === null ? 0 : Math.max(0, Math.ceil((until - now) / 1000));
}

const ERROR_TEXT: Record<string, string> = {
  access_denied: "Access code rejected.",
  not_configured: "The server has no ANTHROPIC_API_KEY configured.",
  offline: "The backend is offline. Start it with make dev.",
  proxy: "The backend is offline. Start it with make dev.",
};

export function RunStatus({ state }: { state: RunState }) {
  const wait = useCountdown(state.retryAt);
  const step = state.steps.length;
  switch (state.status) {
    case "idle":
      return <StatusLine tone="idle">Ready. Pick a target and start the adversary.</StatusLine>;
    case "running":
      return <StatusLine tone="busy">Step {step + 1} of {state.maxSteps}: Claude is thinking</StatusLine>;
    case "backoff":
      return (
        <StatusLine tone="busy">
          {state.retryCode === "overloaded" ? "Anthropic is overloaded" : "Rate limited"}. Retrying in {wait}s.
        </StatusLine>
      );
    case "done":
      return <StatusLine tone="done">{state.finding ? "Violation found and confirmed by the checker." : "Run finished with no violation."}</StatusLine>;
    case "stopped":
      return <StatusLine tone="idle">Stopped after {step} steps.</StatusLine>;
    case "error":
      return <StatusLine tone="error">{ERROR_TEXT[state.error?.code ?? ""] ?? state.error?.message ?? "Something went wrong."}</StatusLine>;
  }
}

export function ControlRail({
  meta,
  metaError,
  form,
  onChange,
  state,
  onStart,
  onStop,
  onReset,
}: {
  meta: Meta | null;
  metaError: string | null;
  form: RunForm;
  onChange: (patch: Partial<RunForm>) => void;
  state: RunState;
  onStart: () => void;
  onStop: () => void;
  onReset: () => void;
}) {
  const active = state.status === "running" || state.status === "backoff";
  const locked = active || state.steps.length > 0;
  const minSteps = meta?.limits.minSteps ?? 10;
  const maxSteps = meta?.maxSteps ?? 40;

  return (
    <div className="flex flex-col">
      <RailSection title="Target">
        <Field label="Cluster variant" htmlFor="target">
          <select
            id="target"
            className="field"
            value={form.target}
            disabled={locked || !meta}
            onChange={(e) => onChange({ target: e.target.value })}
          >
            {meta ? (
              meta.targets.map((t) => (
                <option key={t.codename} value={t.codename}>
                  {t.codename} · {t.difficulty}
                </option>
              ))
            ) : (
              <option value="">{metaError ? "unavailable" : "loading"}</option>
            )}
          </select>
        </Field>
        <Field label="Seed" htmlFor="seed" hint="Base seed suggested to Claude for its experiments. Also names the exported run.">
          <div className="flex gap-1.5">
            <input
              id="seed"
              type="number"
              inputMode="numeric"
              className="field"
              value={form.seed}
              disabled={locked}
              onChange={(e) => onChange({ seed: Math.max(0, Math.trunc(Number(e.target.value) || 0)) })}
            />
            <button
              type="button"
              className="btn shrink-0 px-2"
              disabled={locked}
              aria-label="Shuffle seed"
              title="Shuffle seed"
              onClick={() => onChange({ seed: Math.floor(Math.random() * 100000) })}
            >
              <svg viewBox="0 0 12 12" className="size-3" aria-hidden="true">
                <path d="M1 3h2.5l5 6H11M1 9h2.5l5-6H11M9.5 1.5L11 3 9.5 4.5M9.5 7.5L11 9 9.5 10.5" className="fill-none stroke-current" />
              </svg>
            </button>
          </div>
        </Field>
        <Field label={`Step budget · ${form.maxSteps}`} htmlFor="budget">
          <input
            id="budget"
            type="range"
            className="range"
            min={minSteps}
            max={maxSteps}
            value={form.maxSteps}
            disabled={locked}
            onChange={(e) => onChange({ maxSteps: Number(e.target.value) })}
          />
          <div className="flex justify-between font-mono text-[10px] tabular-nums text-subtle">
            <span>{minSteps}</span>
            <span>{maxSteps}</span>
          </div>
        </Field>
        <div className="flex items-baseline justify-between gap-2 text-xs">
          <span className="text-muted">Model</span>
          <span className="truncate font-mono text-fg">{meta?.model ?? "–"}</span>
        </div>
        {meta?.accessCodeRequired ? (
          <Field label="Access code" htmlFor="access">
            <input
              id="access"
              type="password"
              autoComplete="off"
              className="field"
              value={form.accessCode}
              disabled={active}
              onChange={(e) => onChange({ accessCode: e.target.value })}
            />
          </Field>
        ) : null}
      </RailSection>

      <RailSection title="Run">
        <div className="grid grid-cols-3 gap-1.5">
          <button type="button" className="btn btn-primary" onClick={onStart} disabled={active || !meta || !form.target}>
            Start
          </button>
          <button type="button" className="btn" onClick={onStop} disabled={!active}>
            Stop
          </button>
          <button type="button" className="btn" onClick={onReset} disabled={active || state.status === "idle"}>
            Reset
          </button>
        </div>
        <RunStatus state={state} />
        {metaError ? <StatusLine tone="error">{metaError}</StatusLine> : null}
      </RailSection>

      <RailSection title="Tokens">
        <UsageMeter usage={state.usage} />
        <p className="font-mono text-2xs tabular-nums text-subtle">
          steps {state.steps.length} / {state.maxSteps || form.maxSteps}
        </p>
      </RailSection>
    </div>
  );
}
