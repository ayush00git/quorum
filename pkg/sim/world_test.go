package sim

import (
	"reflect"
	"testing"
)

type ping struct{ n int }

// recorder logs every message it receives.
type recorder struct {
	env         *Env
	got         *[]string
	peer        string
	sendAtStart int
}

func (r *recorder) OnStart() {
	for i := 0; i < r.sendAtStart; i++ {
		r.env.Send(r.peer, ping{i})
	}
}
func (r *recorder) OnMessage(from string, msg any) {
	*r.got = append(*r.got, from+"->"+r.env.ID())
}
func (r *recorder) OnTimer(TimerID) { *r.got = append(*r.got, "timer@"+r.env.ID()) }

func fixed(lat int64) Network { return Network{MinLatencyMs: lat, MaxLatencyMs: lat} }

func TestEventsAtSameTimeRunInScheduleOrder(t *testing.T) {
	w := New(1, fixed(1))
	var order []int
	for i := 0; i < 20; i++ {
		w.At(5, func() { order = append(order, i) })
	}
	w.At(3, func() { order = append(order, -1) })
	w.RunUntil(10)
	want := []int{-1}
	for i := 0; i < 20; i++ {
		want = append(want, i)
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v", order)
	}
}

func TestPartitionCutsInFlightMessages(t *testing.T) {
	var got []string
	w := New(1, fixed(10))
	w.AddNode("n1", func(e *Env) Handler { return &recorder{env: e, got: &got, peer: "n2", sendAtStart: 1} })
	w.AddNode("n2", func(e *Env) Handler { return &recorder{env: e, got: &got} })
	w.At(5, func() { w.Isolate("n2") }) // message sent at 0 arrives at 10
	w.RunUntil(20)
	if len(got) != 0 {
		t.Fatalf("in-flight message delivered across partition: %v", got)
	}
	if sent, dropped := w.MessageStats(); sent != 1 || dropped != 1 {
		t.Fatalf("stats sent=%d dropped=%d", sent, dropped)
	}
}

func TestClientsAreNeverPartitioned(t *testing.T) {
	var got []string
	w := New(1, fixed(2))
	w.AddNode("n1", func(e *Env) Handler { return &recorder{env: e, got: &got} })
	w.AddNode("n2", func(e *Env) Handler { return &recorder{env: e, got: &got} })
	w.AddClient("c1", func(e *Env) Handler { return &recorder{env: e, got: &got, peer: "n1", sendAtStart: 1} })
	w.Isolate("n1")
	w.RunUntil(10)
	if !reflect.DeepEqual(got, []string{"c1->n1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestCrashDropsMessagesAndRestartKeepsDisk(t *testing.T) {
	var got []string
	w := New(1, fixed(10))
	starts := 0
	w.AddNode("n1", func(e *Env) Handler { return &recorder{env: e, got: &got, peer: "n2", sendAtStart: 1} })
	w.AddNode("n2", func(e *Env) Handler {
		starts++
		if starts == 1 {
			e.Disk().State = "persisted"
		} else if e.Disk().State != "persisted" {
			t.Errorf("disk lost across restart: %v", e.Disk().State)
		}
		return &recorder{env: e, got: &got}
	})
	w.At(5, func() { w.Crash("n2") })
	w.At(15, func() { w.Restart("n2") })
	w.RunUntil(30)
	if len(got) != 0 || starts != 2 {
		t.Fatalf("got=%v starts=%d", got, starts)
	}
}

type timerNode struct {
	env   *Env
	fired *[]int64
}

func (n *timerNode) OnStart()              { n.env.SetTimer(10) }
func (n *timerNode) OnMessage(string, any) {}
func (n *timerNode) OnTimer(TimerID)       { *n.fired = append(*n.fired, n.env.w.Now()) }

func TestPauseDelaysTimers(t *testing.T) {
	var fired []int64
	w := New(1, fixed(1))
	w.AddNode("n1", func(e *Env) Handler { return &timerNode{env: e, fired: &fired} })
	w.At(5, func() { w.Pause("n1", 20) })
	w.RunUntil(50)
	if !reflect.DeepEqual(fired, []int64{25}) {
		t.Fatalf("timer fired at %v, want [25]", fired)
	}
	kinds := []string{}
	for _, e := range w.Events() {
		kinds = append(kinds, e.Kind)
	}
	if !reflect.DeepEqual(kinds, []string{EventPauseStart, EventPauseEnd}) {
		t.Fatalf("events %v", kinds)
	}
}

func TestCrashKillsTimers(t *testing.T) {
	var fired []int64
	w := New(1, fixed(1))
	w.AddNode("n1", func(e *Env) Handler { return &timerNode{env: e, fired: &fired} })
	w.At(5, func() { w.Crash("n1") })
	w.At(8, func() { w.Restart("n1") })
	w.RunUntil(50)
	if !reflect.DeepEqual(fired, []int64{18}) {
		t.Fatalf("timers fired at %v, want only the restarted incarnation's [18]", fired)
	}
}

func TestClockSkew(t *testing.T) {
	c := clock{}
	c = clock{fromGlobal: 100, baseLocal: c.local(100), driftPpm: 500_000}
	if got := c.local(200); got != 250 {
		t.Fatalf("local(200) = %d, want 250", got)
	}
	if got := c.globalDelay(150); got != 100 {
		t.Fatalf("globalDelay(150) = %d, want 100", got)
	}
}
