package sim

import (
	"fmt"
	"strings"
)

func (w *World) nodeIDs() []string {
	var ids []string
	for _, a := range w.order {
		if a.node {
			ids = append(ids, a.id)
		}
	}
	return ids
}

// Partition splits nodes into groups. Nodes not listed form one extra group.
func (w *World) Partition(groups [][]string) {
	w.groups = map[string]int{}
	var parts []string
	for i, g := range groups {
		for _, id := range g {
			w.groups[id] = i
		}
		parts = append(parts, strings.Join(g, ","))
	}
	var rest []string
	for _, id := range w.nodeIDs() {
		if _, ok := w.groups[id]; !ok {
			w.groups[id] = len(groups)
			rest = append(rest, id)
		}
	}
	if len(rest) > 0 {
		parts = append(parts, strings.Join(rest, ","))
	}
	w.log("", EventPartition, 0, strings.Join(parts, " | "))
}

// Isolate cuts one node off from every other node.
func (w *World) Isolate(node string) { w.Partition([][]string{{node}}) }

// Heal removes all partitions.
func (w *World) Heal() {
	w.groups = nil
	w.log("", EventHeal, 0, "")
}

// Crash stops a node. Volatile state, timers and queued input are lost; the
// disk is kept.
func (w *World) Crash(node string) {
	a := w.actors[node]
	if a == nil || !a.node || !a.up {
		return
	}
	a.up = false
	a.paused = false
	a.pending = nil
	a.handler = nil
	a.incarnation++
	w.log(node, EventCrash, 0, "")
}

// Restart boots a crashed node from its disk.
func (w *World) Restart(node string) {
	a := w.actors[node]
	if a == nil || !a.node || a.up {
		return
	}
	w.log(node, EventRestart, 0, "")
	w.boot(a)
}

// Pause freezes a node for durationMs: it processes nothing, input queues
// up and timers fire late, then everything queued runs when it resumes.
func (w *World) Pause(node string, durationMs int64) {
	a := w.actors[node]
	if a == nil || !a.node || !a.up {
		return
	}
	a.paused = true
	a.pauseGen++
	gen, inc := a.pauseGen, a.incarnation
	w.log(node, EventPauseStart, 0, fmt.Sprintf("%dms", durationMs))
	w.At(w.now+durationMs, func() {
		if !a.up || a.incarnation != inc || a.pauseGen != gen {
			return
		}
		a.paused = false
		w.log(node, EventPauseEnd, 0, fmt.Sprintf("%d queued", len(a.pending)))
		queued := a.pending
		a.pending = nil
		for _, fn := range queued {
			fn()
		}
	})
}

// SetNetwork changes the network model from now on.
func (w *World) SetNetwork(p NetworkPatch) {
	if p.MinLatencyMs != nil {
		w.net.MinLatencyMs = *p.MinLatencyMs
	}
	if p.MaxLatencyMs != nil {
		w.net.MaxLatencyMs = *p.MaxLatencyMs
	}
	if p.DropRate != nil {
		w.net.DropRate = *p.DropRate
	}
	if p.DuplicateRate != nil {
		w.net.DuplicateRate = *p.DuplicateRate
	}
	n := w.net
	w.log("", EventNetworkChange, 0, fmt.Sprintf("latency %d-%dms drop %g dup %g",
		n.MinLatencyMs, n.MaxLatencyMs, n.DropRate, n.DuplicateRate))
}

// SkewClock makes a node's local clock drift by driftPpm from now on.
func (w *World) SkewClock(node string, driftPpm int64) {
	a := w.actors[node]
	if a == nil || !a.node {
		return
	}
	a.clock = clock{fromGlobal: w.now, baseLocal: a.clock.local(w.now), driftPpm: driftPpm}
	w.log(node, EventClockSkew, 0, fmt.Sprintf("%+dppm", driftPpm))
}
