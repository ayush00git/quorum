package experiment

import (
	"testing"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/raft"
)

// leaderAt returns the node leading at time t in a run, if any.
func leaderAt(res Result, t int64) string {
	for _, l := range res.Stats.Leaders {
		if l.FromMs <= t && t < l.ToMs {
			return l.Node
		}
	}
	return ""
}

// leaderOf runs a target fault-free up to t and returns the leader then.
// Faults scheduled at or after t do not change the run before t.
func leaderOf(t testing.TB, target string, seed, at int64) string {
	t.Helper()
	res, err := Run(Spec{Target: target, Seed: seed, DurationMs: at + 1})
	if err != nil {
		t.Fatal(err)
	}
	l := leaderAt(res, at)
	if l == "" {
		t.Fatalf("no leader at %d ms for seed %d", at, seed)
	}
	return l
}

func bugTarget(t testing.TB, flag func(raft.Bugs) bool) string {
	t.Helper()
	for _, c := range raft.Codenames() {
		if tg, _ := raft.LookupTarget(c); tg.Enabled && flag(tg.Flags) {
			return c
		}
	}
	t.Fatal("no enabled target with that bug")
	return ""
}

// isolateLeader cuts the leader at 1000 ms off for one second.
func isolateLeader(t testing.TB, target string, seed int64) Spec {
	l := leaderOf(t, target, seed, 1000)
	return Spec{Target: target, Seed: seed, DurationMs: 3000, Faults: []Fault{
		{AtMs: 1000, Action: ActionIsolate, Args: FaultArgs{Node: l}},
		{AtMs: 2000, Action: ActionHeal},
	}}
}

// amnesiaSchedule crashes and immediately restarts the leader to force an
// election, slows the network so vote requests linger in flight, and flaps
// one voter through the election so it forgets whom it voted for.
func amnesiaSchedule(t testing.TB, target string) Spec {
	const seed = 37
	l := leaderOf(t, target, seed, 999)
	s := Spec{Target: target, Seed: seed, DurationMs: 3000, Faults: []Fault{
		{AtMs: 990, Action: ActionSetNetwork, Args: FaultArgs{MinLatencyMs: i64(21), MaxLatencyMs: i64(136)}},
		{AtMs: 1000, Action: ActionCrash, Args: FaultArgs{Node: l}},
		{AtMs: 1005, Action: ActionRestart, Args: FaultArgs{Node: l}},
	}}
	for k := int64(0); k < 13; k++ {
		at := 1180 + k*15
		s.Faults = append(s.Faults,
			Fault{AtMs: at, Action: ActionCrash, Args: FaultArgs{Node: "n4"}},
			Fault{AtMs: at + 1, Action: ActionRestart, Args: FaultArgs{Node: "n4"}})
	}
	s.Faults = append(s.Faults, Fault{AtMs: 1600, Action: ActionSetNetwork, Args: FaultArgs{MinLatencyMs: i64(1), MaxLatencyMs: i64(10)}})
	return s
}

// TestCanaries checks that every bug target has a hand-written schedule
// that produces a checker-confirmed violation, and that the same schedule
// is linearizable on the control.
func TestCanaries(t *testing.T) {
	cases := []struct {
		name string
		flag func(raft.Bugs) bool
		spec func(t testing.TB, target string) Spec
	}{
		{"follower reads", func(b raft.Bugs) bool { return b.FollowerReads }, func(t testing.TB, target string) Spec {
			return Spec{Target: target, Seed: 7, ReadFrom: "any"}
		}},
		{"stale leader reads", func(b raft.Bugs) bool { return b.StaleLeaderReads }, func(t testing.TB, target string) Spec {
			// Reads must be able to reach the old leader in its minority.
			s := isolateLeader(t, target, 3)
			s.ReadFrom = "any"
			return s
		}},
		{"early ack", func(b raft.Bugs) bool { return b.EarlyAck }, func(t testing.TB, target string) Spec {
			return isolateLeader(t, target, 1)
		}},
		{"amnesiac voter", func(b raft.Bugs) bool { return b.AmnesiacVoter }, amnesiaSchedule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := bugTarget(t, tc.flag)
			s := tc.spec(t, target)
			res, err := Run(s)
			if err != nil {
				t.Fatal(err)
			}
			if res.Check.Verdict != checker.VerdictViolation {
				t.Fatalf("%s: verdict %s, want violation", target, res.Check.Verdict)
			}
			if res.Check.Explanation == "" || len(res.Check.IllegalIDs) == 0 {
				t.Fatalf("violation without explanation or illegal ops: %+v", res.Check)
			}
			s.Target = "target-a"
			ctl, err := Run(s)
			if err != nil {
				t.Fatal(err)
			}
			if ctl.Check.Verdict != checker.VerdictOK {
				t.Fatalf("control verdict %s on the canary schedule: %s", ctl.Check.Verdict, ctl.Check.Explanation)
			}
		})
	}
}
