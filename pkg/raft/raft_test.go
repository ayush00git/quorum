package raft

import (
	"testing"

	"github.com/ayush00git/quorum/pkg/sim"
)

func TestStateMachineDeduplicatesRetries(t *testing.T) {
	sm := newStateMachine()
	sm.apply(Command{Kind: CmdWrite, Key: "x", Value: 1, Client: 1, Seq: 1})
	sm.apply(Command{Kind: CmdWrite, Key: "x", Value: 2, Client: 2, Seq: 1})
	sm.apply(Command{Kind: CmdWrite, Key: "x", Value: 1, Client: 1, Seq: 1}) // retry
	if v := sm.get("x"); v == nil || *v != 2 {
		t.Fatalf("retried write applied twice: x=%v", v)
	}
	r := sm.apply(Command{Kind: CmdRead, Key: "x", Client: 3, Seq: 1})
	if r.result == nil || *r.result != 2 {
		t.Fatalf("read %v", r.result)
	}
	if c, ok := sm.cached(3, 1); !ok || *c.result != 2 {
		t.Fatal("cached read result missing")
	}
	if !sm.apply(Command{Kind: CmdCas, Key: "x", Expected: 2, Value: 5, Client: 4, Seq: 1}).casOK {
		t.Fatal("matching cas failed")
	}
	if sm.apply(Command{Kind: CmdCas, Key: "x", Expected: 2, Value: 6, Client: 4, Seq: 2}).casOK {
		t.Fatal("stale cas succeeded")
	}
}

func TestMonitorDetectsViolations(t *testing.T) {
	m := NewMonitor()
	m.observeLeader("n1", 1)
	m.observeLeader("n1", 1)
	m.observeLeader("n2", 2)
	if len(m.Violations) != 0 {
		t.Fatalf("false positives: %v", m.Violations)
	}
	m.observeLeader("n3", 2)
	a := Entry{Term: 1, Cmd: Command{Kind: CmdWrite, Key: "x", Value: 1}}
	b := Entry{Term: 1, Cmd: Command{Kind: CmdWrite, Key: "x", Value: 2}}
	m.observeApply("n1", 1, a)
	m.observeApply("n2", 1, a)
	m.observeApply("n3", 1, b)
	if len(m.Violations) != 2 || m.Committed() != 1 {
		t.Fatalf("violations %v committed %d", m.Violations, m.Committed())
	}
}

func cluster(seed int64, bugs Bugs) (*sim.World, *Monitor) {
	w := sim.New(seed, sim.Network{MinLatencyMs: 1, MaxLatencyMs: 10})
	peers := []string{"n1", "n2", "n3", "n4", "n5"}
	cfg := DefaultConfig(peers, bugs)
	cfg.Monitor = NewMonitor()
	for _, p := range peers {
		w.AddNode(p, NewFactory(cfg))
	}
	return w, cfg.Monitor
}

func leaders(w *sim.World) map[int]string {
	out := map[int]string{}
	for _, e := range w.Events() {
		if e.Kind == sim.EventBecameLeader {
			out[e.Term] = e.Node
		}
	}
	return out
}

func TestElectsOneLeaderAndReelectsAfterCrash(t *testing.T) {
	w, mon := cluster(1, Bugs{})
	w.RunUntil(1000)
	first := leaders(w)
	if len(first) != 1 {
		t.Fatalf("leaders after 1s: %v", first)
	}
	var old string
	for _, n := range first {
		old = n
	}
	if mon.Committed() == 0 {
		t.Fatal("the election no-op was never committed")
	}
	w.Crash(old)
	w.RunUntil(2000)
	all := leaders(w)
	if len(all) < 2 {
		t.Fatalf("no new leader after crashing %s: %v", old, all)
	}
	if len(mon.Violations) > 0 {
		t.Fatal(mon.Violations)
	}
}

// client is a minimal test client that sends one request and keeps the reply.
type client struct {
	env   *sim.Env
	to    string
	req   Request
	reply *Reply
}

func (c *client) OnStart()            { c.env.Send(c.to, c.req) }
func (c *client) OnTimer(sim.TimerID) {}
func (c *client) OnMessage(_ string, m any) {
	if r, ok := m.(Reply); ok {
		*c.reply = r
	}
}

func TestFollowerRefusesUnlessFollowerReadsBug(t *testing.T) {
	for _, bug := range []bool{false, true} {
		w, _ := cluster(1, Bugs{FollowerReads: bug})
		w.RunUntil(1000)
		var leader string
		for _, n := range leaders(w) {
			leader = n
		}
		follower := "n1"
		if leader == "n1" {
			follower = "n2"
		}
		var got Reply
		w.AddClient("c1", func(e *sim.Env) sim.Handler {
			return &client{env: e, to: follower, req: Request{Attempt: 1, Op: Command{Kind: CmdRead, Key: "x", Client: 1, Seq: 1}}, reply: &got}
		})
		w.RunUntil(1100)
		if bug && !got.OK {
			t.Fatalf("follower-reads bug: follower did not answer: %+v", got)
		}
		if !bug && (!got.NotLeader || got.Leader != leader || got.Attempt != 1) {
			t.Fatalf("follower reply %+v, want NotLeader with hint %s", got, leader)
		}
	}
}
