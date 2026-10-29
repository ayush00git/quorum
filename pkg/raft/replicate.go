package raft

import (
	"fmt"

	"github.com/ayush00git/quorum/pkg/sim"
)

func (n *Node) appendEntry(c Command) int {
	n.disk.Log = append(n.disk.Log, Entry{Term: n.term(), Cmd: c})
	return n.lastIndex()
}

func (n *Node) broadcast() {
	for _, p := range n.peers() {
		n.sendAppend(p)
	}
}

func (n *Node) sendAppend(peer string) {
	next := n.next[peer]
	if next < 1 {
		next = 1
	}
	prev := next - 1
	end := min(n.lastIndex(), prev+n.cfg.MaxBatch)
	// Copy: the log may be truncated and reused while the message is in flight.
	entries := append([]Entry(nil), n.disk.Log[next:end+1]...)
	n.send(peer, AppendEntries{
		Term: n.term(), Leader: n.id, PrevIndex: prev, PrevTerm: n.disk.Log[prev].Term,
		Entries: entries, Commit: n.commit, Round: n.lease.round,
	})
}

func (n *Node) onAppendEntries(from string, m AppendEntries) {
	if m.Term < n.term() {
		n.send(from, AppendReply{Term: n.term()})
		return
	}
	if m.Term == n.term() && n.role == leader {
		// Election safety rules out two leaders in one term, so a correct
		// cluster never gets here. A leader does not defer to a rival of its
		// own term; it ignores it.
		return
	}
	if m.Term > n.term() || n.role != follower {
		n.stepDown(m.Term)
	}
	n.leader = m.Leader
	n.heardLeaderAt = n.env.Now()
	n.resetElectionDeadline()
	if m.PrevIndex > n.lastIndex() {
		n.send(from, AppendReply{Term: n.term(), Conflict: n.lastIndex() + 1, Round: m.Round})
		return
	}
	if t := n.disk.Log[m.PrevIndex].Term; t != m.PrevTerm {
		i := m.PrevIndex
		for i > 1 && n.disk.Log[i-1].Term == t {
			i--
		}
		n.send(from, AppendReply{Term: n.term(), Conflict: i, Round: m.Round})
		return
	}
	for j, e := range m.Entries {
		idx := m.PrevIndex + 1 + j
		if idx <= n.lastIndex() {
			if n.disk.Log[idx].Term == e.Term {
				continue
			}
			n.disk.Log = n.disk.Log[:idx]
		}
		n.disk.Log = append(n.disk.Log, e)
	}
	match := m.PrevIndex + len(m.Entries)
	if c := min(m.Commit, match); c > n.commit {
		n.commit = c
		n.applyCommitted()
	}
	n.send(from, AppendReply{Term: n.term(), Success: true, Match: match, Round: m.Round})
}

func (n *Node) onAppendReply(from string, m AppendReply) {
	if m.Term > n.term() {
		n.stepDown(m.Term)
		return
	}
	if n.role != leader || m.Term != n.term() {
		return
	}
	if m.Success {
		n.lease.ack(from, m.Round, n)
		if m.Match > n.match[from] {
			n.match[from] = m.Match
		}
		n.next[from] = max(n.next[from], n.match[from]+1)
		n.advanceCommit()
		if n.next[from] <= n.lastIndex() {
			n.sendAppend(from)
		}
		return
	}
	next := max(m.Conflict, 1)
	if next <= n.match[from] {
		next = n.match[from] + 1
	}
	n.next[from] = next
	n.sendAppend(from)
}

// advanceCommit commits the highest index stored on a majority. Only
// entries from the current term are counted (Raft section 5.4.2) unless the
// PrevTermCommit bug is planted.
func (n *Node) advanceCommit() {
	for idx := n.lastIndex(); idx > n.commit; idx-- {
		if n.disk.Log[idx].Term != n.term() && !n.cfg.Bugs.PrevTermCommit {
			break
		}
		count := 1
		for _, p := range n.peers() {
			if n.match[p] >= idx {
				count++
			}
		}
		if count >= n.majority() {
			n.commit = idx
			n.env.Log(sim.EventCommitAdvanced, n.term(), fmt.Sprintf("commit %d", idx))
			n.applyCommitted()
			return
		}
	}
}
