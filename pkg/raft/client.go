package raft

// onRequest serves a client request. In the correct implementation only the
// leader serves requests, every request (reads included) goes through the
// log, and the reply is sent once the entry is committed and applied.
func (n *Node) onRequest(from string, r Request) {
	c := r.Op
	if n.role != leader {
		if c.Kind == CmdRead && n.cfg.Bugs.FollowerReads {
			n.reply(from, c, r.Attempt, session{result: n.sm.get(c.Key)})
			return
		}
		n.send(from, Reply{Client: c.Client, Seq: c.Seq, Attempt: r.Attempt, NotLeader: true, Leader: n.leader})
		return
	}
	if c.Kind == CmdRead && (n.cfg.Bugs.StaleLeaderReads || (n.cfg.Bugs.LeaseReads && n.lease.valid(n))) {
		n.reply(from, c, r.Attempt, session{result: n.sm.get(c.Key)})
		return
	}
	if res, ok := n.sm.cached(c.Client, c.Seq); ok {
		n.reply(from, c, r.Attempt, res)
		return
	}
	idx := n.appendEntry(c)
	if c.Kind == CmdWrite && n.cfg.Bugs.EarlyAck {
		n.reply(from, c, r.Attempt, session{})
	} else {
		n.pending[idx] = pendingReq{from: from, term: n.term(), attempt: r.Attempt, cmd: c}
	}
	n.broadcast()
}
