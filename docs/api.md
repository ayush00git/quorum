# Quorum HTTP API

All endpoints are served by Go (`/api/*`). Next.js has no API routes.
Every payload here is mirrored in `lib/types.ts`. Go structs live in
`pkg/experiment/types.go`, `pkg/workload/op.go`, `pkg/sim/event.go`,
`pkg/checker/result.go` and `pkg/agent` (agent payloads).

## Conventions

- JSON in and out, `Content-Type: application/json`.
- Request bodies are capped at 400 KB and decoded strictly (unknown fields are rejected).
- Errors always look like:

```json
{ "error": { "code": "bad_request", "message": "faults[2].args.node: unknown node \"n9\"" } }
```

| HTTP | code | when |
|---|---|---|
| 400 | `bad_request` | malformed JSON, unknown field, validation failure |
| 401 | `access_denied` | `ACCESS_CODE` is set and the request lacks the right `accessCode` |
| 405 | `method_not_allowed` | wrong method |
| 413 | `too_large` | body over 400 KB |
| 429 | `rate_limited` | Anthropic 429 (browser retries with backoff) |
| 503 | `overloaded` | Anthropic 529 (browser retries with backoff) |
| 502 | `upstream_error` | any other Anthropic failure |
| 500 | `internal` | unexpected error (panics are recovered) |
| 500 | `not_configured` | `ANTHROPIC_API_KEY` missing |

## GET /api/meta

```json
{
  "model": "claude-sonnet-5-5",
  "maxSteps": 30,
  "accessCodeRequired": false,
  "targets": [{ "codename": "target-a", "difficulty": "n/a" }],
  "limits": { "maxDurationMs": 10000, "maxFaults": 30, "nodes": 5, "minSteps": 10, "maxStepsCap": 40 }
}
```

`maxSteps` comes from `AGENT_MAX_STEPS` (default 30, clamped to [10, 40]).
The step-budget slider ranges from `limits.minSteps` to `maxSteps`.

## POST /api/experiment

Request: `{ "spec": Spec }`. Omitted or zero optional fields get defaults
(`experiment.Normalize`). Response: `Result`
(`{ id, spec, history, events, check, stats, historyHash }`), where `spec` is
the normalized spec. Deterministic: the same spec always yields the same
`historyHash`.

Defaults: `durationMs` 3000, `nodes` 5, `clients` 4, `keys` 2,
`readRatio` 0.5, `readFrom` "leader", `opTimeoutMs` 500,
`network` `{ minLatencyMs: 1, maxLatencyMs: 10, dropRate: 0, duplicateRate: 0 }`.
Node ids are `n1`..`n5`. Keys are `x`, `y`, `z`, ... Fault argument shapes:

| action | args |
|---|---|
| `partition` | `{ groups: [["n1","n2"],["n3","n4","n5"]] }` |
| `isolate` | `{ node }` |
| `heal` | `{}` |
| `crash` | `{ node }` |
| `restart` | `{ node }` |
| `pause` | `{ node, durationMs }` |
| `setNetwork` | `{ minLatencyMs?, maxLatencyMs?, dropRate?, duplicateRate? }` |
| `skewClock` | `{ node, driftPpm }` (stretch) |

## POST /api/agent

One agent step: exactly one Claude call, tools executed in Go.

Request:

```json
{
  "target": "target-c",
  "maxSteps": 30,
  "seed": 42,
  "messages": [ { "role": "user", "content": [ ... ] } ],
  "accessCode": "optional"
}
```

- `messages` is the Anthropic Messages API array the browser holds. The very
  first request sends `[]`; the server injects its own kickoff user message,
  and returns it as `kickoff` so the browser can prepend it.
- Roles must be `user` or `assistant`; at most `2 * maxSteps + 2` messages.
- Roles alternate starting with `user`, and the last message must be a
  `user` message. Every content block must be an object with a `type`.
- `maxSteps` is optional, clamped to [10, server `maxSteps`]. A request whose
  history already holds `maxSteps` assistant messages is rejected (400).
- `seed` is optional. When present, the kickoff message asks the agent to use
  it as the base seed for its experiments. It has no other effect.
- The client never chooses the model, system prompt, tools or token limits.

Response:

```json
{
  "kickoff": { "role": "user", "content": [ ... ] },
  "assistant": [ /* Anthropic content blocks: text, tool_use */ ],
  "toolResults": [ /* tool_result blocks (or one text nudge), send back verbatim as a user message */ ],
  "experiments": [
    { "id": "a1b2c3d4", "spec": { }, "verdict": "violation", "ops": 120, "faults": 3, "durationMs": 3000, "shrunkFrom": "" }
  ],
  "usage": { "inputTokens": 0, "outputTokens": 0, "cacheReadTokens": 0, "cacheCreationTokens": 0 },
  "step": 3,
  "done": false,
  "finding": {
    "experimentId": "a1b2c3d4",
    "title": "...",
    "anomaly": "...",
    "causalChain": ["..."],
    "spec": { },
    "check": { }
  },
  "conclusion": "only when the agent called conclude",
  "revealedBug": "only when done"
}
```

`kickoff` only appears on the first step. `finding`, `conclusion` and
`revealedBug` only appear when `done` is true. `done` is also true when the
step budget runs out (then `conclusion` explains that). The browser appends
`{ role: "assistant", content: assistant }` and, if `toolResults` is
non-empty, `{ role: "user", content: toolResults }`, then calls again until
`done`. `assistant`, `toolResults` and `experiments` are always arrays, never
`null`. `assistant` holds the content blocks exactly as Claude returned them,
including `thinking` blocks, which must be sent back unchanged.

Tool results:

- Each `tool_result` has string `content`, ending in
  `budget: {"stepsUsed":N,"stepsLeft":M}`. Failed calls (invalid input, a
  rejected `report_finding`) have `is_error: true` and content starting with
  `error: `; the run continues.
- `run_experiment` and `shrink` results contain the experiment id as
  `experiment <id>:` (the summary's first line). A `shrink` result starts with
  `shrunk <sourceId> to experiment <id>`. The UI matches log entries to
  `experiments[]` by that id.
- `inspect_history` lines mark checker-flagged ops: `*` cannot be linearized,
  `!` conflicts with an op that cannot.
- If Claude ends a turn without calling a tool and the run is not done,
  `toolResults` is a single `text` block nudging it to continue. The browser
  appends it as the next user message like any tool results.

## Recorded run file (`public/runs/<target>-<seed>.json`)

```json
{
  "version": 1,
  "target": "target-c",
  "model": "claude-sonnet-5-5",
  "startedAt": "2026-10-29T12:00:00Z",
  "steps": [ { "assistant": [ ], "toolResults": [ ], "experiments": [ ] } ],
  "experiments": [ Spec ],
  "finding": null,
  "conclusion": null,
  "revealedBug": "...",
  "usage": { "inputTokens": 0, "outputTokens": 0, "cacheReadTokens": 0, "cacheCreationTokens": 0 },
  "historyHashes": { "<experimentId>": "<historyHash>" }
}
```

`public/runs/index.json` lists available runs:
`[{ "id": "target-c-42", "target": "target-c", "steps": 12, "found": true }]`.
