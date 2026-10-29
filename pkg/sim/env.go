package sim

// Env is a handler's view of the world for one incarnation.
type Env struct {
	w   *World
	a   *actor
	inc int
}

// ID is this process's id.
func (e *Env) ID() string { return e.a.id }

// Now is this process's local clock, which drifts under skewClock.
func (e *Env) Now() int64 { return e.a.clock.local(e.w.now) }

// Rand is this process's private stream ("elections/<id>"). It continues
// across restarts.
func (e *Env) Rand() *Rand { return e.a.rand }

// Disk is storage that survives crashes.
func (e *Env) Disk() *Disk { return &e.a.disk }

// Log appends an event to the simulation log.
func (e *Env) Log(kind string, term int, detail string) { e.w.log(e.a.id, kind, term, detail) }

func (e *Env) live() bool { return e.a.up && e.a.incarnation == e.inc }

// Send sends msg to another process. It reports whether the network
// duplicated the message.
func (e *Env) Send(to string, msg any) (duplicated bool) {
	if !e.live() {
		return false
	}
	return e.w.send(e.a, to, msg)
}

// SetTimer fires OnTimer after delayMs of local time. Timers die with the
// incarnation that set them and fire late if the process is paused.
func (e *Env) SetTimer(delayMs int64) TimerID {
	if delayMs < 0 {
		delayMs = 0
	}
	e.w.nextTimer++
	id := e.w.nextTimer
	a, inc := e.a, e.inc
	e.w.At(e.w.now+a.clock.globalDelay(delayMs), func() {
		if !a.up || a.incarnation != inc {
			return
		}
		e.w.deliver(a, func() {
			if a.up && a.incarnation == inc {
				a.handler.OnTimer(id)
			}
		})
	})
	return id
}
