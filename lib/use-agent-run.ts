"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, agentStep } from "./api";
import type { ExperimentSummary, Finding, Message, RecordedStep, Usage } from "./types";

export type RunStatus = "idle" | "running" | "backoff" | "done" | "stopped" | "error";

export interface RunError {
  code: string;
  message: string;
}

export interface RunState {
  status: RunStatus;
  target: string;
  maxSteps: number;
  startedAt: string | null;
  steps: RecordedStep[];
  experiments: ExperimentSummary[];
  usage: Usage;
  finding: Finding | null;
  conclusion: string | null;
  revealedBug: string | null;
  error: RunError | null;
  /** Epoch ms of the next retry while backing off from 429 / 529. */
  retryAt: number | null;
  retryCode: string | null;
}

export interface StartOptions {
  target: string;
  maxSteps: number;
  accessCode?: string;
}

export const EMPTY_USAGE: Usage = {
  inputTokens: 0,
  outputTokens: 0,
  cacheReadTokens: 0,
  cacheCreationTokens: 0,
};

const INITIAL: RunState = {
  status: "idle",
  target: "",
  maxSteps: 0,
  startedAt: null,
  steps: [],
  experiments: [],
  usage: EMPTY_USAGE,
  finding: null,
  conclusion: null,
  revealedBug: null,
  error: null,
  retryAt: null,
  retryCode: null,
};

const BACKOFF_MS = [1000, 2000, 4000, 8000, 16000, 30000];

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const t = setTimeout(resolve, ms);
    signal.addEventListener(
      "abort",
      () => {
        clearTimeout(t);
        reject(new DOMException("Aborted", "AbortError"));
      },
      { once: true },
    );
  });
}

function addUsage(a: Usage, b: Usage): Usage {
  return {
    inputTokens: a.inputTokens + (b.inputTokens ?? 0),
    outputTokens: a.outputTokens + (b.outputTokens ?? 0),
    cacheReadTokens: a.cacheReadTokens + (b.cacheReadTokens ?? 0),
    cacheCreationTokens: a.cacheCreationTokens + (b.cacheCreationTokens ?? 0),
  };
}

function mergeExperiments(
  known: ExperimentSummary[],
  incoming: ExperimentSummary[],
): ExperimentSummary[] {
  const ids = new Set(known.map((e) => e.id));
  return [...known, ...incoming.filter((e) => !ids.has(e.id))];
}

/**
 * Drives the stateless agent loop: the browser owns `messages`, POSTs them to
 * /api/agent, appends the assistant turn and tool results, and repeats until
 * the server says done.
 */
export function useAgentRun() {
  const [state, setState] = useState<RunState>(INITIAL);
  const controller = useRef<AbortController | null>(null);

  useEffect(() => () => controller.current?.abort(), []);

  const start = useCallback(async ({ target, maxSteps, accessCode }: StartOptions) => {
    controller.current?.abort();
    const ac = new AbortController();
    controller.current = ac;
    setState({
      ...INITIAL,
      status: "running",
      target,
      maxSteps,
      startedAt: new Date().toISOString(),
    });

    let messages: Message[] = [];
    try {
      for (;;) {
        let attempt = 0;
        let res;
        for (;;) {
          try {
            res = await agentStep({ target, maxSteps, messages, accessCode }, ac.signal);
            break;
          } catch (err) {
            if (!(err instanceof ApiError) || !err.retryable || attempt >= BACKOFF_MS.length) {
              throw err;
            }
            const delay = BACKOFF_MS[attempt] ?? 30000;
            attempt++;
            setState((s) => ({
              ...s,
              status: "backoff",
              retryAt: Date.now() + delay,
              retryCode: err.code,
            }));
            await sleep(delay, ac.signal);
            setState((s) => ({ ...s, status: "running", retryAt: null, retryCode: null }));
          }
        }

        if (res.kickoff) messages = [res.kickoff];
        messages = [...messages, { role: "assistant", content: res.assistant }];
        if (res.toolResults.length > 0) {
          messages = [...messages, { role: "user", content: res.toolResults }];
        }
        const step: RecordedStep = {
          assistant: res.assistant,
          toolResults: res.toolResults,
          experiments: res.experiments,
        };
        const done = res.done;
        setState((s) => ({
          ...s,
          status: done ? "done" : "running",
          steps: [...s.steps, step],
          experiments: mergeExperiments(s.experiments, res.experiments),
          usage: addUsage(s.usage, res.usage ?? EMPTY_USAGE),
          finding: res.finding ?? s.finding,
          conclusion: res.conclusion ?? s.conclusion,
          revealedBug: res.revealedBug ?? s.revealedBug,
        }));
        if (done) return;
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      const e =
        err instanceof ApiError
          ? { code: err.code, message: err.message }
          : { code: "internal", message: err instanceof Error ? err.message : String(err) };
      setState((s) => ({ ...s, status: "error", error: e, retryAt: null, retryCode: null }));
    } finally {
      if (controller.current === ac) controller.current = null;
    }
  }, []);

  const stop = useCallback(() => {
    controller.current?.abort();
    controller.current = null;
    setState((s) =>
      s.status === "running" || s.status === "backoff"
        ? { ...s, status: "stopped", retryAt: null, retryCode: null }
        : s,
    );
  }, []);

  const reset = useCallback(() => {
    controller.current?.abort();
    controller.current = null;
    setState(INITIAL);
  }, []);

  return { state, start, stop, reset };
}
