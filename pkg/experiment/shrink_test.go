package experiment

import (
	"strings"
	"testing"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/raft"
)

func TestShrinkKeepsViolationAndNeverGrows(t *testing.T) {
	target := bugTarget(t, func(b raft.Bugs) bool { return b.StaleLeaderReads })
	s := isolateLeader(t, target, 3)
	s.ReadFrom = "any"
	// Pad the schedule with noise the shrinker should remove.
	s.Faults = append(s.Faults,
		Fault{AtMs: 300, Action: ActionSetNetwork, Args: FaultArgs{MaxLatencyMs: i64(15)}},
		Fault{AtMs: 2500, Action: ActionPause, Args: FaultArgs{Node: "n5", DurationMs: 100}},
		Fault{AtMs: 2700, Action: ActionCrash, Args: FaultArgs{Node: "n4"}},
	)
	in, err := Normalize(s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Shrink(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Result.Check.Verdict != checker.VerdictViolation {
		t.Fatalf("shrunk spec verdict %s", out.Result.Check.Verdict)
	}
	again, _ := Run(out.Spec)
	if again.Check.Verdict != checker.VerdictViolation || again.HistoryHash != out.Result.HistoryHash {
		t.Fatal("shrunk spec does not reproduce its result")
	}
	o := out.Spec
	if len(o.Faults) > len(in.Faults) || o.DurationMs > in.DurationMs || o.Clients > in.Clients || o.Keys > in.Keys {
		t.Fatalf("shrunk spec grew: %+v", o)
	}
	if len(o.Faults) >= len(in.Faults) {
		t.Fatalf("noise faults not removed: %d -> %d", len(in.Faults), len(o.Faults))
	}
	if out.Runs > MaxShrinkRuns {
		t.Fatalf("%d runs exceeds cap", out.Runs)
	}
	t.Logf("shrunk in %d runs: %d faults, %d ms, %d clients, %d keys", out.Runs, len(o.Faults), o.DurationMs, o.Clients, o.Keys)
}

func TestShrinkRejectsNonViolatingSpec(t *testing.T) {
	_, err := Shrink(Spec{Target: "target-a", Seed: 1, DurationMs: 500})
	if err == nil || !strings.Contains(err.Error(), "does not violate") {
		t.Fatalf("err %v", err)
	}
}

func TestSummarize(t *testing.T) {
	target := bugTarget(t, func(b raft.Bugs) bool { return b.FollowerReads })
	r, err := Run(Spec{Target: target, Seed: 7, ReadFrom: "any", Faults: []Fault{{AtMs: 1500, Action: ActionIsolate, Args: FaultArgs{Node: "n1"}}}})
	if err != nil {
		t.Fatal(err)
	}
	sum := Summarize(r)
	for _, want := range []string{"experiment " + r.ID, "verdict: VIOLATION", "faults (1): 1500 isolate n1", "leaders: t1", "explanation: ", "offending ops", "* #"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary lacks %q:\n%s", want, sum)
		}
	}
	if strings.Contains(sum, "follower") {
		t.Error("summary leaks the bug name")
	}
	if len(sum) > 4000 {
		t.Errorf("summary is %d bytes, want it compact", len(sum))
	}
	t.Log("\n" + sum)
}
