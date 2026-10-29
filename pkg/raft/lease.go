package raft

// leaseState implements leader leases for the LeaseReads bug. Each
// heartbeat starts a round; once a majority acknowledged the round, the
// leader may serve reads locally until roundStart + ElectionMinMs - margin on
// its own clock. Followers refuse votes for ElectionMinMs after hearing from
// a leader, so without clock skew no other leader can exist during the
// lease. The bug is the assumption that all clocks advance at the same rate.
type leaseState struct {
	round      int
	roundStart int64
	acks       map[string]bool
	expires    int64
}

const leaseMarginMs = 10

func (l *leaseState) startRound(now int64) {
	l.round++
	l.roundStart = now
	l.acks = map[string]bool{}
}

func (l *leaseState) ack(from string, round int, n *Node) {
	if !n.cfg.Bugs.LeaseReads || round != l.round || l.acks == nil {
		return
	}
	l.acks[from] = true
	if len(l.acks)+1 >= n.majority() {
		l.expires = max(l.expires, l.roundStart+n.cfg.ElectionMinMs-leaseMarginMs)
	}
}

func (l *leaseState) valid(n *Node) bool { return n.env.Now() < l.expires }

// sticky reports whether a lease-aware follower must refuse to vote because
// it heard from a leader too recently.
func (n *Node) sticky() bool {
	return n.cfg.Bugs.LeaseReads && n.heardLeaderAt > 0 && n.env.Now() < n.heardLeaderAt+n.cfg.ElectionMinMs
}
