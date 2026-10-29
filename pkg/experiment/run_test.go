package experiment

import (
	"bytes"
	"encoding/json"
	"testing"
)

func determinismSpec() Spec {
	return Spec{Target: "target-a", Seed: 42, DurationMs: 2500, ReadFrom: "any", Faults: []Fault{
		{AtMs: 400, Action: ActionSetNetwork, Args: FaultArgs{DropRate: f64(0.1), DuplicateRate: f64(0.2), MaxLatencyMs: i64(30)}},
		{AtMs: 800, Action: ActionPartition, Args: FaultArgs{Groups: [][]string{{"n1", "n2"}, {"n3", "n4", "n5"}}}},
		{AtMs: 1200, Action: ActionCrash, Args: FaultArgs{Node: "n3"}},
		{AtMs: 1300, Action: ActionPause, Args: FaultArgs{Node: "n4", DurationMs: 200}},
		{AtMs: 1600, Action: ActionHeal},
		{AtMs: 1700, Action: ActionRestart, Args: FaultArgs{Node: "n3"}},
	}}
}

// TestRunDeterministic runs the same spec many times to catch any map
// iteration order or other nondeterminism leaking into the simulation.
func TestRunDeterministic(t *testing.T) {
	first, err := Run(determinismSpec())
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(struct {
		R Result
	}{first})
	if first.Stats.Ops == 0 || len(first.Events) == 0 {
		t.Fatalf("empty run: %+v", first.Stats)
	}
	for i := 0; i < 25; i++ {
		got, err := Run(determinismSpec())
		if err != nil {
			t.Fatal(err)
		}
		if got.HistoryHash != first.HistoryHash {
			t.Fatalf("run %d: historyHash %s, want %s", i, got.HistoryHash, first.HistoryHash)
		}
		b, _ := json.Marshal(struct {
			R Result
		}{got})
		if !bytes.Equal(b, firstJSON) {
			t.Fatalf("run %d: result JSON differs", i)
		}
	}
}

func TestDifferentSeedDifferentHistory(t *testing.T) {
	a, _ := Run(determinismSpec())
	s := determinismSpec()
	s.Seed = 43
	b, _ := Run(s)
	if a.HistoryHash == b.HistoryHash {
		t.Fatal("seeds 42 and 43 produced the same history")
	}
	if a.ID == b.ID {
		t.Fatal("different specs share an id")
	}
}

// Changing a fault late in the run must not change anything before it:
// this is what keeps shrinking stable.
func TestLaterFaultDoesNotPerturbEarlierHistory(t *testing.T) {
	a, _ := Run(determinismSpec())
	s := determinismSpec()
	s.Faults = append(s.Faults, Fault{AtMs: 2400, Action: ActionCrash, Args: FaultArgs{Node: "n1"}})
	b, _ := Run(s)
	for i := range a.History {
		if a.History[i].InvokeAt >= 2300 {
			break
		}
		if i >= len(b.History) || a.History[i].Kind != b.History[i].Kind || a.History[i].InvokeAt != b.History[i].InvokeAt {
			t.Fatalf("op %d differs before the added fault", i)
		}
	}
}

func TestSpecIDIsStableAndShort(t *testing.T) {
	s, err := Normalize(determinismSpec())
	if err != nil {
		t.Fatal(err)
	}
	id := SpecID(s)
	if len(id) != 8 || SpecID(s) != id {
		t.Fatalf("id %q", id)
	}
	again, _ := Normalize(s)
	if SpecID(again) != id {
		t.Fatal("Normalize is not idempotent")
	}
}

func TestStatsAndLeaders(t *testing.T) {
	r, _ := Run(determinismSpec())
	st := r.Stats
	if st.OK+st.Fail+st.Info != st.Ops || st.Reads+st.Writes != st.Ops {
		t.Fatalf("inconsistent counts %+v", st)
	}
	if len(st.Leaders) == 0 || st.MessagesSent == 0 || st.MessagesDropped == 0 {
		t.Fatalf("stats %+v", st)
	}
	for _, l := range st.Leaders {
		if l.ToMs < l.FromMs || l.Term < 1 {
			t.Fatalf("bad span %+v", l)
		}
	}
}
