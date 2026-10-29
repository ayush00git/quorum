package experiment

import (
	"fmt"
	"testing"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/raft"
	"github.com/ayush00git/quorum/pkg/sim"
)

func i64(v int64) *int64     { return &v }
func f64(v float64) *float64 { return &v }

// randomFaults builds an aggressive random schedule: partitions, isolation,
// crashes and restarts, pauses, lossy and duplicating networks.
func randomFaults(r *sim.Rand, durationMs int64, n int) []Fault {
	nodes := NodeIDs(NodeCount)
	node := func() string { return nodes[r.Intn(len(nodes))] }
	var faults []Fault
	for i := 0; i < n; i++ {
		at := r.Range(0, durationMs-1)
		var f Fault
		switch r.Intn(9) {
		case 0:
			perm := append([]string(nil), nodes...)
			for j := len(perm) - 1; j > 0; j-- {
				k := r.Intn(j + 1)
				perm[j], perm[k] = perm[k], perm[j]
			}
			cut := 1 + r.Intn(len(perm)-1)
			f = Fault{Action: ActionPartition, Args: FaultArgs{Groups: [][]string{perm[:cut], perm[cut:]}}}
		case 1:
			f = Fault{Action: ActionIsolate, Args: FaultArgs{Node: node()}}
		case 2, 3:
			f = Fault{Action: ActionHeal}
		case 4:
			f = Fault{Action: ActionCrash, Args: FaultArgs{Node: node()}}
		case 5, 6:
			f = Fault{Action: ActionRestart, Args: FaultArgs{Node: node()}}
		case 7:
			f = Fault{Action: ActionPause, Args: FaultArgs{Node: node(), DurationMs: r.Range(50, 800)}}
		case 8:
			f = Fault{Action: ActionSetNetwork, Args: FaultArgs{
				MinLatencyMs: i64(r.Range(0, 5)), MaxLatencyMs: i64(r.Range(10, 60)),
				DropRate: f64(float64(r.Intn(3)) / 10), DuplicateRate: f64(float64(r.Intn(3)) / 10),
			}}
		}
		f.AtMs = at
		faults = append(faults, f)
	}
	return faults
}

// TestControlTargetSafety is non-negotiable: across many random fault
// schedules the correct target never has two leaders in a term, never loses
// or replaces a committed entry, and is always linearizable.
func TestControlTargetSafety(t *testing.T) {
	const schedules = 60
	r := sim.NewRand(2026)
	for i := 0; i < schedules; i++ {
		s := DefaultSpec("target-a")
		s.Seed = int64(r.Uint64() >> 1)
		s.DurationMs = 2000 + r.Range(0, 2000)
		s.Clients = 2 + r.Intn(4)
		s.Keys = 1 + r.Intn(3)
		if r.Intn(2) == 0 {
			s.ReadFrom = "any"
		}
		s.Faults = randomFaults(r, s.DurationMs, 4+r.Intn(14))
		t.Run(fmt.Sprintf("schedule-%02d", i), func(t *testing.T) {
			mon := raft.NewMonitor()
			res, err := run(s, CheckTimeout, mon)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			for _, v := range mon.Violations {
				t.Errorf("raft safety: %s", v)
			}
			if res.Check.Verdict != checker.VerdictOK {
				t.Errorf("verdict %s: %s\nspec id %s", res.Check.Verdict, res.Check.Explanation, res.ID)
			}
			if mon.Committed() == 0 {
				t.Errorf("nothing committed in %d ms", s.DurationMs)
			}
		})
	}
}
