package experiment

import (
	"testing"

	"github.com/ayush00git/quorum/pkg/raft"
)

func BenchmarkRun(b *testing.B) {
	s := determinismSpec()
	for i := 0; i < b.N; i++ {
		if _, err := Run(s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunMaxDuration(b *testing.B) {
	s := determinismSpec()
	s.DurationMs = MaxDurationMs
	s.Clients = MaxClients
	for i := 0; i < b.N; i++ {
		if _, err := Run(s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShrink(b *testing.B) {
	target := bugTarget(b, func(bg raft.Bugs) bool { return bg.EarlyAck })
	s := isolateLeader(b, target, 1)
	s.Faults = append(s.Faults,
		Fault{AtMs: 300, Action: ActionSetNetwork, Args: FaultArgs{MaxLatencyMs: i64(15)}},
		Fault{AtMs: 2500, Action: ActionPause, Args: FaultArgs{Node: "n5", DurationMs: 100}},
		Fault{AtMs: 2700, Action: ActionCrash, Args: FaultArgs{Node: "n4"}},
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := Shrink(s)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(res.Runs), "runs/op")
	}
}
