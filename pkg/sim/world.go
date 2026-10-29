package sim

import (
	"container/heap"
	"fmt"
)

// Handler is the behavior of a simulated process. Handlers run one at a
// time on the simulation loop and must not start goroutines.
type Handler interface {
	OnStart()
	OnMessage(from string, msg any)
	OnTimer(id TimerID)
}

// Factory builds a fresh handler. It is called at start and again on every
// restart, so anything the handler keeps outside its Disk is volatile.
type Factory func(env *Env) Handler

// TimerID identifies a timer set through Env.SetTimer.
type TimerID uint64

// Disk is storage that survives crashes. State is owned by the node.
type Disk struct {
	State any
}

// maxEvents bounds a single run so a livelock cannot hang a request.
const maxEvents = 4_000_000

type actor struct {
	id          string
	node        bool // nodes can crash and be partitioned; clients cannot
	factory     Factory
	handler     Handler
	up          bool
	incarnation int
	paused      bool
	pauseGen    int
	pending     []func()
	disk        Disk
	rand        *Rand
	clock       clock
}

// World is one deterministic simulation.
type World struct {
	now       int64
	seq       uint64
	processed int
	q         queue
	root      *Rand
	netRand   *Rand
	net       Network
	groups    map[string]int // node -> partition group; nil when healed
	actors    map[string]*actor
	order     []*actor
	nextTimer TimerID
	events    []Event
	sent      int
	dropped   int
}

// New creates a world with a seed and initial network model.
func New(seed int64, net Network) *World {
	root := NewRand(uint64(seed))
	return &World{
		root:    root,
		netRand: root.Fork("network"),
		net:     net,
		actors:  map[string]*actor{},
	}
}

// Rand returns the named stream derived from the world seed.
func (w *World) Rand(name string) *Rand { return w.root.Fork(name) }

// AddNode registers a crashable, partitionable process.
func (w *World) AddNode(id string, f Factory) { w.add(id, f, true) }

// AddClient registers a process that never crashes and is never partitioned.
func (w *World) AddClient(id string, f Factory) { w.add(id, f, false) }

func (w *World) add(id string, f Factory, node bool) {
	if _, dup := w.actors[id]; dup {
		panic(fmt.Sprintf("sim: duplicate actor %q", id))
	}
	stream := "workload/" + id
	if node {
		stream = "elections/" + id
	}
	a := &actor{id: id, node: node, factory: f, rand: w.root.Fork(stream)}
	w.actors[id] = a
	w.order = append(w.order, a)
	w.At(0, func() { w.boot(a) })
}

func (w *World) boot(a *actor) {
	a.up = true
	a.incarnation++
	a.handler = a.factory(&Env{w: w, a: a, inc: a.incarnation})
	a.handler.OnStart()
}

// At schedules fn at virtual time t (clamped to now).
func (w *World) At(t int64, fn func()) {
	if t < w.now {
		t = w.now
	}
	w.seq++
	heap.Push(&w.q, item{at: t, seq: w.seq, fn: fn})
}

// Now is the global virtual time in milliseconds.
func (w *World) Now() int64 { return w.now }

// RunUntil processes every event scheduled at or before end.
func (w *World) RunUntil(end int64) {
	for w.q.Len() > 0 && w.q[0].at <= end && w.processed < maxEvents {
		it := heap.Pop(&w.q).(item)
		w.now = it.at
		w.processed++
		it.fn()
	}
	if w.now < end {
		w.now = end
	}
}

// Events returns the event log.
func (w *World) Events() []Event { return w.events }

// MessageStats returns how many messages were sent and dropped.
func (w *World) MessageStats() (sent, dropped int) { return w.sent, w.dropped }

// Up reports whether a node is running.
func (w *World) Up(id string) bool {
	a := w.actors[id]
	return a != nil && a.up
}

func (w *World) log(node, kind string, term int, detail string) {
	w.events = append(w.events, Event{AtMs: w.now, Node: node, Kind: kind, Term: term, Detail: detail})
}

// deliver runs fn for actor a now, or queues it if a is paused.
func (w *World) deliver(a *actor, fn func()) {
	if a.paused {
		a.pending = append(a.pending, fn)
		return
	}
	fn()
}
