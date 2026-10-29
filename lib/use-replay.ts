"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { collectExperiments, isText } from "./agent-log";
import { ApiError } from "./api";
import { loadExperiment } from "./experiment-cache";
import type { RecordedRun, RecordedStep } from "./types";

export type Speed = 1 | 2 | 4;

export type VerifyStatus = "match" | "mismatch" | "offline" | "error" | "unrecorded";

/** Reading pace: a beat per step plus time proportional to its text. */
export function stepDelayMs(step: RecordedStep): number {
  const chars = step.assistant.reduce((n, b) => n + (isText(b) ? b.text.length : 0), 0);
  return Math.min(1200 + chars * 12, 7000);
}

export function useReplay(run: RecordedRun) {
  const [shown, setShown] = useState(0);
  const [playing, setPlaying] = useState(true);
  const [speed, setSpeed] = useState<Speed>(1);
  const [verify, setVerify] = useState<Record<string, VerifyStatus>>({});

  const total = run.steps.length;
  const steps = useMemo(() => run.steps.slice(0, shown), [run.steps, shown]);
  const experiments = useMemo(() => collectExperiments(steps), [steps]);
  const finished = shown >= total;

  useEffect(() => {
    if (!playing || finished) return;
    const next = run.steps[shown];
    const delay = shown === 0 ? 400 : next ? stepDelayMs(next) / speed : 0;
    const t = setTimeout(() => setShown((n) => Math.min(n + 1, total)), delay);
    return () => clearTimeout(t);
  }, [playing, finished, shown, speed, run.steps, total]);

  // Re-execute each experiment as it appears and compare history hashes.
  // Missing entries are still pending.
  const started = useRef(new Set<string>());
  useEffect(() => {
    for (const e of experiments) {
      if (started.current.has(e.id)) continue;
      started.current.add(e.id);
      const expected = run.historyHashes[e.id];
      loadExperiment(e.id, e.spec).then(
        (r) => {
          const status: VerifyStatus =
            expected === undefined ? "unrecorded" : r.historyHash === expected ? "match" : "mismatch";
          setVerify((v) => ({ ...v, [e.id]: status }));
        },
        (err: unknown) => {
          const status: VerifyStatus = err instanceof ApiError && !err.offline ? "error" : "offline";
          setVerify((v) => ({ ...v, [e.id]: status }));
        },
      );
    }
  }, [experiments, run.historyHashes]);

  const toggle = useCallback(() => {
    if (finished) {
      setShown(0);
      setPlaying(true);
    } else setPlaying((p) => !p);
  }, [finished]);

  const stepForward = useCallback(() => {
    setPlaying(false);
    setShown((n) => Math.min(n + 1, total));
  }, [total]);

  const stepBack = useCallback(() => {
    setPlaying(false);
    setShown((n) => Math.max(n - 1, 0));
  }, []);

  return {
    steps,
    experiments,
    shown,
    total,
    finished,
    playing: playing && !finished,
    speed,
    setSpeed,
    toggle,
    stepForward,
    stepBack,
    verify,
  };
}
