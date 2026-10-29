// Package agenttest provides a deterministic fake of the experiment core
// for agent and handler tests.
package agenttest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/experiment"
	"github.com/ayush00git/quorum/pkg/sim"
	"github.com/ayush00git/quorum/pkg/workload"
)

// FakeEngine stands in for the experiment core: an experiment violates
// exactly when its schedule contains an isolate fault. Targets are
// target-a (control, bug "none") and target-c.
type FakeEngine struct {
	Runs    int // calls to Run
	Shrinks int // calls to Shrink
}

// ParseSpec implements agent.Engine.
func (f *FakeEngine) ParseSpec(data []byte) (experiment.Spec, error) {
	var s experiment.Spec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return experiment.Spec{}, err
	}
	if s.DurationMs == 0 {
		s.DurationMs = 3000
	}
	if s.DurationMs > experiment.MaxDurationMs {
		return experiment.Spec{}, fmt.Errorf("durationMs %d exceeds %d", s.DurationMs, experiment.MaxDurationMs)
	}
	if s.Nodes == 0 {
		s.Nodes = experiment.NodeCount
	}
	if s.Clients == 0 {
		s.Clients = 4
	}
	if s.Keys == 0 {
		s.Keys = 2
	}
	return s, nil
}

// SpecID implements agent.Engine.
func (f *FakeEngine) SpecID(s experiment.Spec) string {
	b, _ := json.Marshal(s)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:4])
}

// Violates reports whether the fake core says s violates linearizability.
func Violates(s experiment.Spec) bool {
	for _, fl := range s.Faults {
		if fl.Action == experiment.ActionIsolate {
			return true
		}
	}
	return false
}

// Run implements agent.Engine.
func (f *FakeEngine) Run(s experiment.Spec) (experiment.Result, error) {
	f.Runs++
	r := experiment.Result{ID: f.SpecID(s), Spec: s, HistoryHash: "h-" + f.SpecID(s)}
	for i := range 80 {
		op := workload.Op{ID: i, Process: i % 4, Kind: workload.KindWrite, Key: "x", Value: new(i), Status: workload.StatusOK, InvokeAt: int64(i * 10), CompleteAt: new(int64(i*10 + 5)), Node: "n1"}
		if i%2 == 1 {
			op.Kind, op.Value, op.Result, op.Key = workload.KindRead, nil, new(i-1), "y"
		}
		r.History = append(r.History, op)
	}
	r.Events = []sim.Event{
		{AtMs: 0, Node: "n1", Kind: sim.EventElectionStarted, Term: 1},
		{AtMs: 150, Node: "n1", Kind: sim.EventBecameLeader, Term: 1},
		{AtMs: 500, Kind: sim.EventPartition, Detail: "n1 | n2 n3 n4 n5"},
	}
	r.Check.Verdict = checker.VerdictOK
	if Violates(s) {
		r.Check = checker.CheckResult{Verdict: checker.VerdictViolation, Key: "x", IllegalIDs: []int{3}, OffendingIDs: []int{3, 4}, Explanation: "stale read"}
	}
	return r, nil
}

// Summarize implements agent.Engine.
func (f *FakeEngine) Summarize(r experiment.Result) string {
	return fmt.Sprintf("experiment %s: verdict %s, ops %d", r.ID, r.Check.Verdict, len(r.History))
}

// Shrink implements agent.Engine.
func (f *FakeEngine) Shrink(s experiment.Spec) (experiment.ShrinkResult, error) {
	f.Shrinks++
	if !Violates(s) {
		return experiment.ShrinkResult{}, errors.New("not violating")
	}
	min := s
	min.Faults = nil
	for _, fl := range s.Faults {
		if fl.Action == experiment.ActionIsolate {
			min.Faults = []experiment.Fault{fl}
			break
		}
	}
	min.DurationMs = 1000
	r, _ := f.Run(min)
	return experiment.ShrinkResult{Spec: min, Result: r, Runs: 7}, nil
}

// Targets implements agent.Engine.
func (f *FakeEngine) Targets() []experiment.TargetInfo {
	return []experiment.TargetInfo{{Codename: "target-a", Difficulty: "n/a"}, {Codename: "target-c", Difficulty: "medium"}}
}

// RevealBug implements agent.Engine.
func (f *FakeEngine) RevealBug(codename string) (string, bool) {
	if codename == "target-c" {
		return "stale leader reads", true
	}
	return "none", true
}

// DefaultSpec implements agent.Engine.
func (f *FakeEngine) DefaultSpec(target string) experiment.Spec {
	return experiment.Spec{Target: target, DurationMs: 3000, Nodes: 5, Clients: 4, Keys: 2}
}
