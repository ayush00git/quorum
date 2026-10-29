package sim

// Network is the message delivery model.
type Network struct {
	MinLatencyMs  int64
	MaxLatencyMs  int64
	DropRate      float64
	DuplicateRate float64
}

// NetworkPatch changes some network parameters; nil fields are unchanged.
type NetworkPatch struct {
	MinLatencyMs  *int64
	MaxLatencyMs  *int64
	DropRate      *float64
	DuplicateRate *float64
}

// cut reports whether a and b are in different partition groups. Clients
// are never partitioned.
func (w *World) cut(a, b string) bool {
	if w.groups == nil {
		return false
	}
	ga, okA := w.groups[a]
	gb, okB := w.groups[b]
	return okA && okB && ga != gb
}

func (w *World) send(from *actor, to string, msg any) bool {
	w.sent++
	dst := w.actors[to]
	if dst == nil || w.cut(from.id, to) {
		w.dropped++
		return false
	}
	if w.net.DropRate > 0 && w.netRand.Float64() < w.net.DropRate {
		w.dropped++
		return false
	}
	w.schedule(from.id, dst, msg)
	if w.net.DuplicateRate > 0 && w.netRand.Float64() < w.net.DuplicateRate {
		w.schedule(from.id, dst, msg)
		return true
	}
	return false
}

func (w *World) schedule(from string, dst *actor, msg any) {
	lat := w.netRand.Range(w.net.MinLatencyMs, w.net.MaxLatencyMs)
	w.At(w.now+lat, func() {
		// Partitions cut messages that are already in flight.
		if !dst.up || w.cut(from, dst.id) {
			w.dropped++
			return
		}
		inc := dst.incarnation
		w.deliver(dst, func() {
			if dst.up && dst.incarnation == inc {
				dst.handler.OnMessage(from, msg)
			}
		})
	})
}
