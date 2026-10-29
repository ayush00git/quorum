# Build Quorum: an AI adversary that hunts consistency bugs in distributed systems

Read this entire document before writing any code. It is the full spec. Plan the work with a task list, then build it autonomously, phase by phase. Do not stop to ask for confirmation unless you are genuinely blocked. When something in this spec is ambiguous, make the most reasonable engineering decision, keep moving, and record the decision in the README under "Design decisions".

---

## 1. The product

Quorum is a Jepsen-style testing lab where Claude plays the adversary. A simulated 5-node Raft key-value cluster runs inside a deterministic simulator written in Go. Some cluster variants ("targets") contain a planted, realistic consistency bug; one is correct. Claude does not know which bug, if any, is planted. It only sees the cluster as a black box.

Claude's job is to break it. It forms a hypothesis about how the system might violate linearizability, designs a fault schedule (network partitions, crashes, restarts, process pauses, latency), runs an experiment, reads the results of a real linearizability checker, refines its hypothesis, and repeats. When it finds a violation, it shrinks the fault schedule to a minimal reproduction and writes a finding that explains the causal chain. On the correct target it should conclude, after spending its budget, that it found no violation.

The checker is the ground truth. Claude can never claim a bug the checker has not confirmed.

The user watches all of this live in a Next.js interface: Claude's reasoning, the cluster topology changing under faults, and a timeline of every client operation with the violating operations highlighted.

Working name: **Quorum**.

---

## 2. Hard constraints

**Language split**
- The backend is **Go**: the simulator, Raft, workload, checker, experiment runner, shrinker, agent loop, Anthropic calls and all HTTP endpoints.
- The frontend is **Next.js with TypeScript, for UI only**. Next.js must have no API routes, no route handlers and no server actions. Every `/api/*` path is served by Go.

**Backend layout**
- `go.mod` lives at the repository root.
- HTTP endpoints live in a root-level `/api` directory: one `.go` file per endpoint, all in `package handler`, each exporting exactly one function with the `http.HandlerFunc` signature and a unique name (`Meta`, `Agent`, `Experiment`). Handlers stay thin and delegate to shared packages.
- Shared code lives under `/pkg` (not `/internal`).
- Standard library `net/http` only. No web framework.

**Statelessness**
- Every request is self-contained: no in-memory state shared across requests, no database, no filesystem writes, no goroutines that outlive the request. Every request must finish well under 60 seconds.

**Local development**
- `cmd/devserver` mounts the same three handlers on `127.0.0.1:8080`.
- `next.config.ts` rewrites `/api/:path*` to `http://127.0.0.1:8080/api/:path*` in development only.
- One command (`make dev`) runs both the Go dev server and `next dev` together.

**Determinism rules for the simulator and everything it touches**
- No `time.Now`, no global `math/rand`, no goroutines or channels inside the simulation.
- Never let map iteration order affect behavior: sort keys wherever order matters.
- Same experiment spec in, byte-identical result out, every time.

**Secrets**
- The Anthropic API key is read only by Go from the environment and never reaches the browser.

**Quality gates**
- `go vet ./...` and `go test -race ./...` pass.
- `next build` passes with zero type errors and zero lint errors.

---

## 3. Tech stack

**Backend**
- Go (use the locally installed version and declare it in `go.mod`).
- `github.com/anthropics/anthropic-sdk-go` for Claude.
- `github.com/anishathalye/porcupine` for linearizability checking.
- Keep other dependencies to a minimum. Before using any library API you are not certain about, read its source or `go doc` output instead of guessing.

**Frontend**
- Next.js, latest stable, App Router, TypeScript strict (`strict: true`, `noUncheckedIndexedAccess: true`), no `any`.
- **Tailwind CSS v4**, CSS-first configuration: `@import "tailwindcss";` with design tokens in an `@theme` block in `app/globals.css`. Do not create a `tailwind.config.js`.
- Geist Sans and Geist Mono.
- No component libraries. Build every component by hand so the design stays coherent.
- Use pnpm if installed, otherwise npm.

**Environment variables** (create `.env.example`, gitignore all real env files):

```
ANTHROPIC_API_KEY=        # required, read by Go only
ANTHROPIC_MODEL=claude-sonnet-5-5
AGENT_MAX_STEPS=30
ACCESS_CODE=              # optional; when set, /api/agent requires it
```

---

## 4. Repository layout

```
go.mod
Makefile
api/
  meta.go          GET  /api/meta
  agent.go         POST /api/agent
  experiment.go    POST /api/experiment
pkg/
  sim/             deterministic discrete-event simulator
  raft/            Raft KV node + bug flags
  workload/        simulated clients, op generation, history recording
  checker/         porcupine model, verdicts, human-readable explanations
  experiment/      spec, runner, summarizer, shrinker, hashing
  agent/           tools, executor, system prompt, loop, Anthropic client
  httpx/           JSON helpers, validation, error mapping, access code
cmd/
  devserver/       local server mounting the api handlers
  record/          headless agent run that writes a recorded run
app/               Next.js pages
components/        UI components
lib/               TS API client, types mirroring Go JSON, timeline layout math, replay player
public/runs/       curated recorded runs (JSON)
docs/api.md        the HTTP contract
```

Keep files focused (roughly under 300 lines). Idiomatic Go: small packages, clear types, errors wrapped with context, table-driven tests. No dead code, no commented-out code, no placeholder TODOs left behind.

---

## 5. Deterministic simulation engine (`pkg/sim`)

- **Seeded PRNG**: a small explicit generator (xoshiro256** or SplitMix64) owned by the simulator. Support forking independent named streams (`network`, `elections`, `workload`) so changing the fault schedule does not perturb unrelated randomness. This keeps shrinking stable.
- **Virtual clock** in integer milliseconds.
- **Event queue**: `container/heap` ordered by `(time, sequence)` for deterministic tie-breaking.
- **Node runtime**: nodes implement `OnStart`, `OnMessage(from, msg)`, `OnTimer(id)`. Each node gets a `Disk` whose contents survive crashes; everything else is volatile and reset on crash.
- **Network model**:
  - Per-message latency uniform in `[MinLatencyMs, MaxLatencyMs]`.
  - Configurable drop rate and duplicate rate.
  - Partitions are sets of groups. A message is dropped if its endpoints are in different groups at send time **or** at delivery time, so partitions also cut in-flight messages.
  - Messages to crashed nodes are dropped.
- **Fault actions**, each scheduled at a virtual time:
  - `partition { groups: [][]string }`
  - `isolate { node }`
  - `heal {}`
  - `crash { node }` (volatile state lost, disk kept)
  - `restart { node }`
  - `pause { node, durationMs }` (GC-style pause: the node processes nothing, incoming messages queue, timers fire late, then everything is processed when the pause ends)
  - `setNetwork { minLatencyMs?, maxLatencyMs?, dropRate?, duplicateRate? }`
  - Stretch: `skewClock { node, driftPpm }`, affecting only that node's local clock, used by lease logic.
- **Event log**: every significant event with time and node: election started, vote granted, became leader (term N), stepped down, commit index advanced, crash, restart, pause start and end, partition, heal, network change.

---

## 6. Raft KV store and bug targets (`pkg/raft`)

Implement Raft faithfully enough that the correct target survives aggressive fault fuzzing:

- Terms, `RequestVote`, `AppendEntries`, randomized election timeouts (default 150 to 300 ms, from the `elections` stream), heartbeats every 50 ms.
- Log matching with conflict truncation, `nextIndex` and `matchIndex` per follower.
- A leader advances `commitIndex` only for entries from its **current term** once a majority has them (Raft section 5.4.2). A new leader appends a no-op on election.
- `currentTerm`, `votedFor` and the log are persisted to `Disk` before responding to RPCs.
- State machine: key to integer value, entries applied strictly in log order after commit.
- Client requests: a non-leader replies `NotLeader` with a leader hint. The leader appends the command and replies only after it is committed and applied. In the correct target, **reads also go through the log**.
- Client sessions deduplicate retried requests by `(clientId, requestSeq)`.

**Targets.** Each target is the same implementation with a set of bug flags. The UI and the agent only ever see the codename. The real bug name is revealed in the UI only after a run ends.

| Codename | Planted bug | Difficulty | Priority |
|---|---|---|---|
| `target-a` | none (correct control) | n/a | must |
| `target-b` | **follower reads**: followers answer reads from their local state machine | easy | must |
| `target-c` | **stale leader reads**: the leader answers reads from local state without going through the log or confirming it is still leader | medium | must |
| `target-d` | **early ack**: the leader acknowledges a write once it is in its own log, before majority replication | medium | must |
| `target-e` | **amnesiac voter**: `votedFor` is not persisted, so a restarted node can vote twice in one term | hard | must |
| `target-f` | **previous-term commit**: the leader commits earlier-term entries by counting replicas (the Raft paper's Figure 8 bug) | very hard | stretch |
| `target-g` | **lease reads under clock skew**: the leader serves reads using a time-based lease that assumes bounded drift | hard | stretch |

Shuffle which codename maps to which bug using one fixed map in a single file, so the mapping is not guessable from ordering. Nothing the agent can see may reveal the mapping. `/api/meta` exposes codenames and difficulty only; the bug name for a target is returned only inside a completed run's final response.

---

## 7. Workload and history (`pkg/workload`)

- `clients` concurrent simulated client processes (default 4), each with at most one outstanding operation, Jepsen style.
- Operations: `read(key)` and `write(key, value)`. Stretch: `cas(key, expected, next)`.
- Small key space (default 2 keys) for contention. Every write value is unique per key (a per-key counter).
- Option `readFrom: "leader" | "any"`. With `"any"`, reads go to a random node.
- Clients cache the leader, follow `NotLeader` hints and retry up to a small limit.
- Operation timeout (default 500 ms virtual). On timeout the operation is **indeterminate**: status `info`, then that process id retires and a fresh process id takes its place.
- Status `fail` only when the operation definitely did not take effect (for example, `NotLeader` before anything was appended).

History record (JSON tags in camelCase, mirrored exactly in `lib/types.ts`):

```go
type Op struct {
    ID         int      `json:"id"`
    Process    int      `json:"process"`
    Kind       string   `json:"kind"`               // read | write | cas
    Key        string   `json:"key"`
    Value      *int     `json:"value,omitempty"`    // write value or cas target
    Expected   *int     `json:"expected,omitempty"` // cas only
    Result     *int     `json:"result,omitempty"`   // read result, nil means unset key
    Status     string   `json:"status"`             // ok | fail | info
    InvokeAt   int64    `json:"invokeAt"`
    CompleteAt *int64   `json:"completeAt"`         // nil for info
    Node       string   `json:"node"`
}
```

---

## 8. Linearizability checking (`pkg/checker`)

- Use porcupine with a register model partitioned per key (initial value unset).
- `fail` operations are excluded. `info` writes get a return time far beyond the end of the history (they may take effect any time after invocation, or never). `info` reads are dropped since they impose no constraint.
- Use the verbose check with a timeout. A timeout produces verdict `unknown`, never a hang.
- Use the linearization info porcupine returns to identify the longest linearizable prefix and the operations that cannot be linearized.
- Output:

```go
type CheckResult struct {
    Verdict        string `json:"verdict"`        // ok | violation | unknown
    Key            string `json:"key,omitempty"`
    LinearizedIDs  []int  `json:"linearizedIds,omitempty"`
    OffendingIDs   []int  `json:"offendingIds,omitempty"`  // ops that cannot be linearized + the ops they conflict with
    Explanation    string `json:"explanation,omitempty"`
}
```

The explanation must be concrete and in plain English, with times, values and process ids, for example: "read(x) by p3 returned 4 at 812 ms, but write(x, 7) by p1 completed at 640 ms and no later write set x back to 4."

---

## 9. Experiments and shrinking (`pkg/experiment`)

- `Spec`: `seed`, `target`, `durationMs` (max 10000), `nodes` (fixed 5), `clients`, `keys`, `readRatio`, `readFrom`, `opTimeoutMs`, network defaults, and `faults: []{ atMs, action, args }` (max 30). Validate strictly with clear error messages; reject unknown fields.
- `Run(spec)` is a pure function returning `{ id, spec, history, events, check, stats, historyHash }`. The `id` is a short hash of the canonical JSON of the spec. Same spec, same result.
- `Summarize(result)` produces a compact, token-efficient text summary for the agent: verdict, op counts by status, leader timeline (term, node, from, to), notable events, and on violation the checker explanation plus offending op ids.
- `Shrink(spec)`: delta-debugging (ddmin) over the fault list, then binary-search `durationMs` down, then reduce `clients` and `keys`. Accept a candidate only if it still violates. Cap at about 80 runs. Return the minimal spec and its result.
- A single `Run` should take milliseconds. Benchmark it (`go test -bench`) and keep `Shrink` well under a few seconds.

---

## 10. HTTP API

Document the contract in `docs/api.md` and mirror every payload in `lib/types.ts`.

- `GET /api/meta` → `{ model, maxSteps, accessCodeRequired, targets: [{ codename, difficulty }], limits }`
- `POST /api/experiment` with `{ spec }` → the full experiment result (history, events, check, stats, historyHash). Used by the UI to render any experiment and by replay to verify determinism.
- `POST /api/agent` → one agent step (see below).

All handlers: method checks, request size cap (around 400 KB), strict JSON decoding (`DisallowUnknownFields`), consistent JSON errors `{ error: { code, message } }`, no panics escaping (recover middleware), never log or return the API key.

---

## 11. The agent (`pkg/agent`, `api/agent.go`)

### 11.1 Loop design: one Claude call per request, tools executed in Go

1. The browser holds the `messages` array and POSTs `{ target, messages, accessCode? }` to `/api/agent`.
2. Go validates the payload, then calls the Anthropic Messages API with a server-owned system prompt, server-owned tools, the model from `ANTHROPIC_MODEL` and a moderate `max_tokens`. The client can never choose the model, system prompt, tools or token limits.
3. If the response contains `tool_use` blocks, Go executes them immediately in-process and returns:
   `{ assistant: <content blocks>, toolResults: <tool_result blocks>, experiments: <summaries of experiments run this step>, usage, done, finding? , revealedBug? }`
4. The browser appends the assistant message and a user message containing the tool results exactly as returned, renders everything, and calls again until `done`.

**Stateless experiment registry.** Experiments are deterministic and their id is the hash of their spec, so Go rebuilds the registry on every request by scanning previous `run_experiment` and `shrink` tool calls in `messages`, recomputing any experiment lazily when a tool needs it. Never trust tool result content sent back by the client for decisions: `report_finding` re-runs the referenced spec server-side and checks the verdict itself.

**Guardrails**: message count at most `2 * AGENT_MAX_STEPS + 2`; only `user` and `assistant` roles; access code enforced when `ACCESS_CODE` is set; prompt caching with cache control on the system prompt and the last tool definition; Anthropic errors mapped to clean JSON errors (the browser retries 429 and 529 with exponential backoff and shows that state).

The loop lives in `pkg/agent` with the transport abstracted, so `cmd/record` runs a full session headlessly with the exact same code.

### 11.2 Tools

Every input is validated; every result includes `budget: { stepsUsed, stepsLeft }`. Keep each tool result compact (aim under about 2,500 tokens).

1. `cluster_info()`: node ids, target codename, defaults, limits, the fault actions and their argument shapes.
2. `run_experiment(spec)`: runs it, returns the summary (not the raw history).
3. `inspect_history({ experimentId, key?, fromMs?, toMs?, limit? })`: one line per op, max 60 lines.
4. `inspect_events({ experimentId, node?, fromMs?, toMs?, limit? })`: event log slice, max 60 lines.
5. `shrink({ experimentId })`: returns the minimal spec, its id and its summary.
6. `report_finding({ experimentId, title, anomaly, causalChain: []string })`: ends the run. **Rejected** unless that experiment's verdict is `violation`.
7. `conclude({ summary })`: ends the run with no violation found.

### 11.3 Adversary system prompt (`pkg/agent/prompt.go`)

Write a strong system prompt that establishes:

- Claude is an expert distributed systems tester in the tradition of Jepsen, attacking a black-box 5-node replicated key-value store that claims linearizability.
- The goal is a checker-confirmed linearizability violation, or an honest conclusion that none was found within budget.
- Before every `run_experiment`, write a short explicit hypothesis: which failure mode, and why this fault schedule should expose it. After each result, state what was learned.
- Think in real failure modes: isolating the leader into a minority, reads during leader changes, crashes right after acknowledged writes, crash and restart during elections, process pauses that create zombie leaders, flapping partitions, high latency and duplicates. Use `cluster_info` and the event logs to aim faults precisely (for example, isolate whichever node is leader at a given time).
- Start with small, cheap, targeted experiments. Never spray random faults.
- Never claim a violation the checker did not report. Always `shrink` before reporting.
- The finding explains the causal chain step by step with times and node ids.
- Be budget-aware and finish cleanly before the steps run out.

---

## 12. Recording and replay

- A run can be exported from the UI as JSON: `{ version, target, model, startedAt, steps: [{ assistant, toolResults }], experiments: [spec], finding, revealedBug, usage, historyHashes }`.
- Replay plays back Claude's messages with realistic pacing and fetches each recorded experiment from `/api/experiment`, comparing the returned `historyHash` with the recording. Show a small "deterministic replay verified" indicator.
- `go run ./cmd/record -target target-c -seed 42` runs a full session headlessly and writes `public/runs/<target>-<seed>.json`.
- Once the agent works end to end, if `ANTHROPIC_API_KEY` is set in the local environment, record one run for each "must" bug target and the control, and commit them. If it is not set, leave the command documented in the README.

---

## 13. UI and design system

### 13.1 Design language: strictly monochrome

- Grayscale only. No hue anywhere: not in status, charts, focus rings, links or highlights. Meaning comes from fill, inversion, weight, stroke style and pattern.
  - ok: outlined shape
  - violation: solid inverted block
  - unknown or indeterminate: dashed stroke or diagonal hatching
- Dark theme by default, light theme via `prefers-color-scheme` plus a manual toggle. All tokens (background, foreground, muted, subtle, border, inverse) defined in the Tailwind v4 `@theme` block and used everywhere.
- Typography: Geist Sans for interface text, Geist Mono for data, operations, tool calls and numbers, with `tabular-nums`. Small uppercase tracked labels for section headers. A tight, deliberate type scale.
- 1px hairline borders, strict grid alignment, border radius at most 4px. No shadows, no gradients, no glassmorphism, no emoji, no stock illustrations, no generic SaaS hero art.
- The feel: a precision instrument. An oscilloscope panel crossed with a Jepsen report and a well-kept terminal. Dense, with clear hierarchy and breathing room.
- Functional motion only, 120 to 200 ms, respecting `prefers-reduced-motion`.
- Accessible: AA contrast, full keyboard navigation, visible grayscale focus states, `aria-label`s on SVG regions.
- Responsive from 375px to wide desktop.

### 13.2 Pages

**`/` Landing**
- Headline along the lines of "An AI that tries to break your distributed system." with a one-line subhead.
- Primary CTA "Open the lab", secondary "Watch a replay".
- "How it works" in four steps: Hypothesize, Inject faults, Check linearizability, Shrink and report.
- A real history timeline rendered from a recorded run as the hero visual.
- A targets ladder: codename, difficulty, and for recorded runs the number of steps Claude needed. Bug names stay hidden.
- Minimal footer with a placeholder repo link.

**`/lab` Live lab**
- Desktop: left control rail, center workspace, right agent log.
- Control rail: target select (codename and difficulty), seed with shuffle, step budget slider (10 to 40), model shown read-only, access code field only when required, Start, Stop, Reset, live token usage (input, output, cached).
- Center workspace:
  - **Cluster view** (SVG): 5 nodes on a circle. Leader filled, followers outlined, crashed nodes crossed out, paused nodes hatched, term under each node. Links solid when connected, dashed or removed when partitioned, partition groups as hatched regions.
  - **Experiment list**: one row per experiment with verdict glyph, op count, fault count and duration. Selecting a row loads the full experiment from `/api/experiment` and drives the views.
  - **History timeline** (porcupine style): one lane per process, each op a bar from invoke to complete labeled `w(x,3)` or `r(x)→2`, indeterminate ops as open-ended hatched bars, faults as labeled vertical markers across all lanes, offending ops inverted and conflicting ops outlined. Horizontal zoom and pan, hover tooltip with full op details, and a time scrubber that also drives the cluster view.
- Agent log: a clean stream of Hypothesis, Tool call (collapsible, monospace JSON), Result summary and Learned entries.
- When the run ends: a Finding panel with title, anomaly, numbered causal chain, the minimal fault schedule, a "Reveal planted bug" button and "Export run".
- Mobile: the three areas become tabs (Agent, Timeline, Cluster), controls in a top sheet.
- Thoughtful empty, loading, error and rate-limited states.

**`/replay/[id]`**: the lab layout, read-only, with play, pause, step and speed controls (1x, 2x, 4x). Empty state with the record command if no runs exist.

---

## 14. Testing

**Go** (`go test -race ./...`, under about 30 seconds total):
- **Determinism**: the same spec run twice yields an identical `historyHash`; a different seed yields a different one. Run it many times to catch any map-order leak.
- **PRNG**: forked streams are independent and reproducible.
- **Checker**: hand-written histories covering sequential ok, concurrent ok, stale read violation, lost write violation, an indeterminate write that lands late (must be ok), `fail` ops ignored, and a timeout producing `unknown`.
- **Raft safety on the control target**: across at least 50 random fault schedules, at most one leader per term, no committed entry ever lost, and the verdict is `ok` every time. This guards against false positives and is non-negotiable.
- **Canaries**: for every bug target, one hand-written fault schedule that reliably produces a violation.
- **Shrinker**: the output still violates and is no larger than the input.
- **Agent executor**: invalid inputs rejected with useful messages; `report_finding` rejected for a non-violating experiment; registry reconstruction from a message history works.
- **Handlers**: `httptest` coverage for validation, size limits, access code and error shapes, with the Anthropic client stubbed behind an interface.

**Frontend**: typecheck and lint clean. Unit test the timeline layout math with vitest.

---

## 15. Git workflow

- If the directory is not a git repository, run `git init`. Work only in local history: never add a remote and never push. Do not change git config.
- Commit in small, modular, logical units as you go, never one large commit at the end. Do not mix unrelated changes in one commit. Every commit should build, and tests should pass from the point tests exist.
- Conventional Commits with a scope: `type(scope): summary`
  - types: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `style`, `chore`, `build`
  - scopes: `sim`, `raft`, `workload`, `checker`, `experiment`, `agent`, `api`, `devserver`, `record`, `ui`, `lab`, `replay`, `landing`, `deps`
  - imperative mood, lowercase, no trailing period, at most 72 characters
  - a short body explaining the why for non-trivial commits
  - examples: `feat(sim): add seeded event queue with deterministic tie-breaking`, `test(checker): cover indeterminate writes completing late`, `fix(raft): persist votedFor before granting a vote`
- **Do not add any `Co-Authored-By` trailer. Do not add "Generated with Claude Code" or any other attribution line.** No emoji in commit messages.
- Never amend, rebase or squash commits that already exist.

---

## 16. Build order

Must-have phases, in order. Commit at the end of every meaningful step inside each phase.

0. Scaffold: `go.mod`, package skeletons, `cmd/devserver`, Next.js app, Tailwind v4 tokens, fonts, strict TypeScript, ESLint, dev rewrites, `Makefile`, `.env.example`.
1. Simulation engine and its tests.
2. Raft KV with the correct target, workload, history recording.
3. Checker integration and its tests.
4. Experiment runner, summarizer, shrinker, control-safety fuzz test, benchmarks.
5. Bug targets b to e with canary tests.
6. Agent: tools, executor, system prompt, loop, the three handlers, `cmd/record`. Verify end to end headlessly: `target-c` must produce a checker-confirmed finding within budget and `target-a` must conclude with none.
7. Lab UI.
8. Landing page and replay.
9. Recorded runs, README, final polish on spacing, typography, states and responsiveness.

Stretch, only after everything above is solid: targets f and g, `cas` operations, streaming agent output, a manual fault schedule editor in the lab, animated message flow in the cluster view.

---

## 17. Definition of done

- `go vet`, `go test -race`, `next build` and lint all pass cleanly.
- With a valid key, a live run against `target-c` finds, shrinks and reports a checker-confirmed violation, and a run against `target-a` concludes with no violation.
- Replays verify deterministic re-execution against the recorded hashes.
- The UI is strictly monochrome, polished and usable at 375px and 1440px.
- README covers what Quorum is, a Mermaid architecture diagram, how the agent loop and checker work, how to run locally (`make dev`), environment variables, how to record runs, and Design decisions. It does not reveal the codename to bug mapping.
- The git history is a clean sequence of small Conventional Commits with no co-author or attribution trailers.
