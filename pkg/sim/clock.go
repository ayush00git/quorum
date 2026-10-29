package sim

// clock maps global virtual time to a process's local time. With drift d
// ppm, local time advances (1e6+d)/1e6 times as fast as global time.
type clock struct {
	fromGlobal int64 // global time the current drift started
	baseLocal  int64 // local time at fromGlobal
	driftPpm   int64
}

func (c clock) local(global int64) int64 {
	return c.baseLocal + (global-c.fromGlobal)*(1_000_000+c.driftPpm)/1_000_000
}

// globalDelay converts a local-time delay to global time, rounding up so a
// timer never fires early by local time.
func (c clock) globalDelay(localMs int64) int64 {
	rate := 1_000_000 + c.driftPpm
	return (localMs*1_000_000 + rate - 1) / rate
}
