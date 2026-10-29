package workload

import (
	"fmt"
	"testing"

	"github.com/ayush00git/quorum/pkg/raft"
	"github.com/ayush00git/quorum/pkg/sim"
)

// server answers every request the same way: silently, NotLeader, or OK.
type server struct {
	env  *sim.Env
	mode string
}

func (s *server) OnStart()            {}
func (s *server) OnTimer(sim.TimerID) {}
func (s *server) OnMessage(from string, m any) {
	r := m.(raft.Request)
	switch s.mode {
	case "refuse":
		s.env.Send(from, raft.Reply{Client: r.Op.Client, Seq: r.Op.Seq, Attempt: r.Attempt, NotLeader: true})
	case "ok":
		s.env.Send(from, raft.Reply{Client: r.Op.Client, Seq: r.Op.Seq, Attempt: r.Attempt, OK: true})
	}
}

func runClients(t *testing.T, mode string, dupRate float64) []Op {
	t.Helper()
	w := sim.New(3, sim.Network{MinLatencyMs: 1, MaxLatencyMs: 5, DuplicateRate: dupRate})
	nodes := []string{"n1", "n2"}
	for _, n := range nodes {
		w.AddNode(n, func(e *sim.Env) sim.Handler { return &server{env: e, mode: mode} })
	}
	rec := NewRecorder(Config{Nodes: nodes, Keys: []string{"x", "y"}, ReadRatio: 0.5, ReadFrom: ReadFromLeader,
		OpTimeoutMs: 100, MaxAttempts: 3, ThinkMaxMs: 10, RetryMs: 5}, 1000)
	w.AddClient("c1", rec.Factory())
	w.AddClient("c2", rec.Factory())
	w.RunUntil(1200)
	return rec.History()
}

func TestTimeoutsAreInfoAndRetireTheProcess(t *testing.T) {
	h := runClients(t, "silent", 0)
	if len(h) < 10 {
		t.Fatalf("only %d ops", len(h))
	}
	seen := map[int]bool{}
	for _, op := range h {
		if op.Status != StatusInfo || op.CompleteAt != nil {
			t.Fatalf("op %+v should be info with no completion", op)
		}
		if seen[op.Process] {
			t.Fatalf("process %d reused after an info op", op.Process)
		}
		seen[op.Process] = true
	}
}

func TestRefusedOpsFailUnlessDuplicated(t *testing.T) {
	for _, op := range runClients(t, "refuse", 0) {
		if op.Status != StatusFail {
			t.Fatalf("refused op %+v, want fail", op)
		}
	}
	info := 0
	for _, op := range runClients(t, "refuse", 0.5) {
		if op.Status == StatusInfo {
			info++
		}
	}
	if info == 0 {
		t.Fatal("refused ops with duplicated requests must be info, never a definite fail")
	}
}

func TestWriteValuesAreUniquePerKey(t *testing.T) {
	seen := map[string]bool{}
	writes := 0
	for _, op := range runClients(t, "ok", 0) {
		if op.Status != StatusOK || op.CompleteAt == nil || *op.CompleteAt < op.InvokeAt {
			t.Fatalf("bad op %+v", op)
		}
		if op.Kind != KindWrite {
			continue
		}
		writes++
		k := fmt.Sprintf("%s=%d", op.Key, *op.Value)
		if seen[k] {
			t.Fatalf("value %d written twice to %s", *op.Value, op.Key)
		}
		seen[k] = true
	}
	if writes == 0 {
		t.Fatal("no writes")
	}
}
