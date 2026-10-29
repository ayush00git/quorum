package raft

import (
	"fmt"

	"github.com/ayush00git/quorum/pkg/sim"
)

// resetElectionDeadline picks a fresh randomized election timeout. A single
// timer is re-armed lazily instead of setting a new one per heartbeat.
func (n *Node) resetElectionDeadline() {
	timeout := n.env.Rand().Range(n.cfg.ElectionMinMs, n.cfg.ElectionMaxMs)
	n.electionDeadline = n.env.Now() + timeout
	n.electionTimer = n.env.SetTimer(timeout)
}

func (n *Node) onElectionTimer() {
	if n.role == leader {
		return
	}
	if now := n.env.Now(); now < n.electionDeadline {
		n.electionTimer = n.env.SetTimer(n.electionDeadline - now)
		return
	}
	n.startElection()
}

func (n *Node) startElection() {
	n.role = candidate
	n.leader = ""
	n.disk.Term++
	n.setVotedFor(n.id)
	n.votes = map[string]bool{n.id: true}
	n.env.Log(sim.EventElectionStarted, n.term(), "")
	n.resetElectionDeadline()
	rv := RequestVote{Term: n.term(), Candidate: n.id, LastIndex: n.lastIndex(), LastTerm: n.lastTerm()}
	for _, p := range n.peers() {
		n.send(p, rv)
	}
}

func (n *Node) onRequestVote(from string, m RequestVote) {
	if m.Term > n.term() && !n.sticky() {
		n.stepDown(m.Term)
	}
	upToDate := m.LastTerm > n.lastTerm() || (m.LastTerm == n.lastTerm() && m.LastIndex >= n.lastIndex())
	grant := m.Term == n.term() && (n.votedFor == "" || n.votedFor == m.Candidate) && upToDate && !n.sticky()
	if grant {
		n.setVotedFor(m.Candidate)
		n.resetElectionDeadline()
		n.env.Log(sim.EventVoteGranted, n.term(), "for "+m.Candidate)
	}
	n.send(from, VoteReply{Term: n.term(), Granted: grant})
}

func (n *Node) onVoteReply(from string, m VoteReply) {
	if m.Term > n.term() {
		n.stepDown(m.Term)
		return
	}
	if n.role != candidate || m.Term != n.term() || !m.Granted {
		return
	}
	n.votes[from] = true
	if len(n.votes) >= n.majority() {
		n.becomeLeader()
	}
}

func (n *Node) becomeLeader() {
	n.role = leader
	n.leader = n.id
	n.lease = leaseState{}
	n.next = map[string]int{}
	n.match = map[string]int{}
	for _, p := range n.peers() {
		n.next[p] = n.lastIndex() + 1
	}
	n.env.Log(sim.EventBecameLeader, n.term(), fmt.Sprintf("log %d, commit %d", n.lastIndex(), n.commit))
	if n.cfg.Monitor != nil {
		n.cfg.Monitor.observeLeader(n.id, n.term())
	}
	if !n.cfg.Bugs.PrevTermCommit {
		n.appendEntry(Command{Kind: CmdNoop})
	}
	n.broadcast()
	n.heartbeatTimer = n.env.SetTimer(n.cfg.HeartbeatMs)
}

func (n *Node) onHeartbeat() {
	if n.role != leader {
		return
	}
	n.lease.startRound(n.env.Now())
	n.broadcast()
	n.heartbeatTimer = n.env.SetTimer(n.cfg.HeartbeatMs)
}
