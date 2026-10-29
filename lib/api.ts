import type {
  AgentRequest,
  AgentResponse,
  ExperimentResult,
  Meta,
  RecordedRun,
  RunIndexEntry,
  Spec,
  SpecInput,
} from "./types";

/** A failed request. `status` 0 means the backend could not be reached. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }

  get offline(): boolean {
    return this.status === 0 || this.code === "proxy";
  }

  /** Anthropic 429 / 529: the browser retries these with backoff. */
  get retryable(): boolean {
    return this.code === "rate_limited" || this.code === "overloaded";
  }
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null;
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  let res: Response;
  try {
    res = await fetch(url, {
      ...init,
      headers: { "Content-Type": "application/json", ...init.headers },
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") throw err;
    throw new ApiError(0, "offline", "Could not reach the Quorum backend.");
  }

  const text = await res.text();
  let body: unknown = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    // Non-JSON means something other than the Go server answered (dev proxy
    // with the backend down, static host without Go, ...).
    throw new ApiError(
      res.ok ? 500 : res.status,
      res.status >= 500 || res.status === 404 ? "proxy" : "bad_response",
      res.status >= 500 || res.status === 404
        ? "The Quorum backend is not running."
        : "Unexpected response from the backend.",
    );
  }

  if (!res.ok) {
    const err = isRecord(body) && isRecord(body.error) ? body.error : null;
    const code = err && typeof err.code === "string" ? err.code : "internal";
    const message =
      err && typeof err.message === "string" ? err.message : `Request failed (${res.status}).`;
    throw new ApiError(res.status, code, message);
  }
  return body as T;
}

// Go marshals nil slices as null. Coerce them here so the UI can rely on arrays.
function normalizeSpec(spec: Spec): Spec {
  return {
    ...spec,
    faults: (spec.faults ?? []).map((f) => ({ ...f, args: f.args ?? {} })),
  };
}

export function normalizeResult(r: ExperimentResult): ExperimentResult {
  return {
    ...r,
    spec: normalizeSpec(r.spec),
    history: r.history ?? [],
    events: r.events ?? [],
    check: r.check ?? { verdict: "unknown" },
    stats: { ...r.stats, leaders: r.stats?.leaders ?? [] },
  };
}

function normalizeAgentResponse(r: AgentResponse): AgentResponse {
  return {
    ...r,
    assistant: r.assistant ?? [],
    toolResults: r.toolResults ?? [],
    experiments: (r.experiments ?? []).map((e) => ({ ...e, spec: normalizeSpec(e.spec) })),
    finding: r.finding ? { ...r.finding, spec: normalizeSpec(r.finding.spec) } : undefined,
  };
}

export function getMeta(signal?: AbortSignal): Promise<Meta> {
  return request<Meta>("/api/meta", { signal });
}

export async function runExperiment(
  spec: SpecInput | Spec,
  signal?: AbortSignal,
): Promise<ExperimentResult> {
  const r = await request<ExperimentResult>("/api/experiment", {
    method: "POST",
    body: JSON.stringify({ spec }),
    signal,
  });
  return normalizeResult(r);
}

export async function agentStep(req: AgentRequest, signal?: AbortSignal): Promise<AgentResponse> {
  const r = await request<AgentResponse>("/api/agent", {
    method: "POST",
    body: JSON.stringify(req),
    signal,
  });
  return normalizeAgentResponse(r);
}

/** Recorded runs are static files in public/runs. */
export async function getRunIndex(signal?: AbortSignal): Promise<RunIndexEntry[]> {
  const res = await fetch("/runs/index.json", { signal, cache: "no-store" });
  if (!res.ok) return [];
  const body: unknown = await res.json();
  return Array.isArray(body) ? (body as RunIndexEntry[]) : [];
}

export async function getRecordedRun(id: string, signal?: AbortSignal): Promise<RecordedRun | null> {
  if (!/^[a-z0-9-]+$/i.test(id)) return null;
  const res = await fetch(`/runs/${id}.json`, { signal, cache: "no-store" });
  if (!res.ok) return null;
  return (await res.json()) as RecordedRun;
}
