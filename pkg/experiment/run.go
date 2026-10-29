package experiment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/raft"
	"github.com/ayush00git/quorum/pkg/sim"
	"github.com/ayush00git/quorum/pkg/workload"
)

// CheckTimeout bounds the linearizability check of one Run. The verdict is
// unknown if it runs out; the history and its hash never depend on it.
const CheckTimeout = 3 * time.Second

// Workload tuning shared by every run.
const (
	maxAttempts = 3
	thinkMaxMs  = 10
	retryMs     = 20
)

// SpecID is a short hash of the canonical JSON of a normalized spec.
func SpecID(s Spec) string { return shortHash(s, 8) }

func shortHash(v any, n int) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("experiment: hashing %T: %v", v, err))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:n]
}

// Run executes a spec. It is pure and deterministic: the same spec always
// yields the same history, events and historyHash.
func Run(s Spec) (Result, error) { return run(s, CheckTimeout, nil) }

func run(s Spec, timeout time.Duration, mon *raft.Monitor) (Result, error) {
	s, err := Normalize(s)
	if err != nil {
		return Result{}, err
	}
	target, _ := raft.LookupTarget(s.Target)
	nodes := NodeIDs(s.Nodes)

	w := sim.New(s.Seed, sim.Network{
		MinLatencyMs: s.Network.MinLatencyMs, MaxLatencyMs: s.Network.MaxLatencyMs,
		DropRate: s.Network.DropRate, DuplicateRate: s.Network.DuplicateRate,
	})
	cfg := raft.DefaultConfig(nodes, target.Flags)
	cfg.Monitor = mon
	for _, id := range nodes {
		w.AddNode(id, raft.NewFactory(cfg))
	}
	rec := workload.NewRecorder(workload.Config{
		Nodes: nodes, Keys: KeyNames(s.Keys), ReadRatio: s.ReadRatio, ReadFrom: s.ReadFrom,
		OpTimeoutMs: s.OpTimeoutMs, MaxAttempts: maxAttempts, ThinkMaxMs: thinkMaxMs, RetryMs: retryMs,
	}, s.DurationMs)
	for i := 0; i < s.Clients; i++ {
		w.AddClient(fmt.Sprintf("c%d", i+1), rec.Factory())
	}
	schedule(w, s.Faults)
	w.RunUntil(s.DurationMs)

	history := rec.History()
	if history == nil {
		history = []workload.Op{}
	}
	events := w.Events()
	if events == nil {
		events = []sim.Event{}
	}
	sent, dropped := w.MessageStats()
	return Result{
		ID:      SpecID(s),
		Spec:    s,
		History: history,
		Events:  events,
		Check:   checker.Check(history, timeout),
		Stats:   stats(history, events, sent, dropped, s.DurationMs),
		HistoryHash: shortHash(struct {
			H []workload.Op `json:"h"`
			E []sim.Event   `json:"e"`
		}{history, events}, 16),
	}, nil
}

func stats(history []workload.Op, events []sim.Event, sent, dropped int, end int64) Stats {
	st := Stats{Ops: len(history), MessagesSent: sent, MessagesDropped: dropped, Leaders: leaderSpans(events, end)}
	for _, op := range history {
		switch op.Status {
		case workload.StatusOK:
			st.OK++
		case workload.StatusFail:
			st.Fail++
		default:
			st.Info++
		}
		if op.Kind == workload.KindRead {
			st.Reads++
		} else {
			st.Writes++
		}
	}
	return st
}

// leaderSpans reports when each node believed it was leader. Spans of an old
// leader and a new one can overlap: that is a stale leader that has not yet
// heard of the newer term.
func leaderSpans(events []sim.Event, end int64) []LeaderSpan {
	spans := []LeaderSpan{}
	open := map[string]int{} // node -> index into spans
	for _, e := range events {
		switch e.Kind {
		case sim.EventBecameLeader:
			open[e.Node] = len(spans)
			spans = append(spans, LeaderSpan{Term: e.Term, Node: e.Node, FromMs: e.AtMs, ToMs: end})
		case sim.EventSteppedDown, sim.EventCrash:
			if i, ok := open[e.Node]; ok {
				spans[i].ToMs = e.AtMs
				delete(open, e.Node)
			}
		}
	}
	return spans
}
