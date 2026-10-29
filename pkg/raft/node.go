package raft

import "github.com/ayush00git/quorum/pkg/sim"

type role int

const (
	follower role = iota
	candidate
	leader
)

// Config configures every node of a cluster.
type Config struct {
	Peers         []string // all node ids, including this one, in a fixed order
	Bugs          Bugs
	ElectionMinMs int64
	ElectionMaxMs int64
	HeartbeatMs   int64
	MaxBatch      int
	Monitor       *Monitor // optional safety monitor, used by tests
}

// DefaultConfig returns the standard timing for peers.
func DefaultConfig(peers []string, bugs Bugs) Config {
	return Config{Peers: peers, Bugs: bugs, ElectionMinMs: 150, ElectionMaxMs: 300, HeartbeatMs: 50, MaxBatch: 64}
}

// persistent is the state kept on Disk. Handlers run atomically, so writing
// it before sending a reply is the same as fsyncing before responding.
type persistent struct {
	Term     int
	VotedFor string
	Log      []Entry // Log[0] is a sentinel with term 0
}

type pendingReq struct {
	from    string
	term    int
	attempt int
	cmd     Command
}

// Node is one Raft server.
type Node struct {
	env  *sim.Env
	cfg  Config
	id   string
	disk *persistent

	votedFor string // volatile copy; the only copy under the AmnesiacVoter bug
	role     role
	leader   string
	commit   int
	applied  int
	sm       *stateMachine
	votes    map[string]bool
	next     map[string]int
	match    map[string]int
	pending  map[int]pendingReq
	lease    leaseState

	heardLeaderAt int64 // local time of the last AppendEntries from a leader

	electionTimer    sim.TimerID
	electionDeadline int64
	heartbeatTimer   sim.TimerID
}

// NewFactory returns a sim.Factory that builds nodes of this cluster.
func NewFactory(cfg Config) sim.Factory {
	return func(env *sim.Env) sim.Handler { return newNode(env, cfg) }
}

func newNode(env *sim.Env, cfg Config) *Node {
	d := env.Disk()
	if d.State == nil {
		d.State = &persistent{Log: []Entry{{}}}
	}
	n := &Node{env: env, cfg: cfg, id: env.ID(), disk: d.State.(*persistent), sm: newStateMachine(), pending: map[int]pendingReq{}}
	if !cfg.Bugs.AmnesiacVoter {
		n.votedFor = n.disk.VotedFor
	}
	return n
}

// OnStart begins as a follower waiting for a leader.
func (n *Node) OnStart() { n.resetElectionDeadline() }

// OnMessage dispatches an incoming message.
func (n *Node) OnMessage(from string, msg any) {
	switch m := msg.(type) {
	case RequestVote:
		n.onRequestVote(from, m)
	case VoteReply:
		n.onVoteReply(from, m)
	case AppendEntries:
		n.onAppendEntries(from, m)
	case AppendReply:
		n.onAppendReply(from, m)
	case Request:
		n.onRequest(from, m)
	}
}

// OnTimer dispatches timers; stale timer ids are ignored.
func (n *Node) OnTimer(id sim.TimerID) {
	switch id {
	case n.electionTimer:
		n.onElectionTimer()
	case n.heartbeatTimer:
		n.onHeartbeat()
	}
}

func (n *Node) term() int      { return n.disk.Term }
func (n *Node) lastIndex() int { return len(n.disk.Log) - 1 }
func (n *Node) lastTerm() int  { return n.disk.Log[n.lastIndex()].Term }
func (n *Node) majority() int  { return len(n.cfg.Peers)/2 + 1 }

func (n *Node) setVotedFor(id string) {
	n.votedFor = id
	if !n.cfg.Bugs.AmnesiacVoter {
		n.disk.VotedFor = id
	}
}

// stepDown moves to term (if higher) as a follower.
func (n *Node) stepDown(term int) {
	if term > n.term() {
		n.disk.Term = term
		n.setVotedFor("")
	}
	if n.role == leader {
		n.env.Log(sim.EventSteppedDown, n.term(), "")
		n.heartbeatTimer = 0
		n.resetElectionDeadline()
	}
	n.role = follower
}

func (n *Node) send(to string, msg any) { n.env.Send(to, msg) }

func (n *Node) peers() []string {
	out := make([]string, 0, len(n.cfg.Peers)-1)
	for _, p := range n.cfg.Peers {
		if p != n.id {
			out = append(out, p)
		}
	}
	return out
}

// applyCommitted applies newly committed entries in log order and answers
// any client waiting on them.
func (n *Node) applyCommitted() {
	for n.applied < n.commit {
		n.applied++
		e := n.disk.Log[n.applied]
		res := n.sm.apply(e.Cmd)
		if n.cfg.Monitor != nil {
			n.cfg.Monitor.observeApply(n.id, n.applied, e)
		}
		p, ok := n.pending[n.applied]
		if !ok {
			continue
		}
		delete(n.pending, n.applied)
		if p.term == e.Term && p.cmd == e.Cmd {
			n.reply(p.from, p.cmd, p.attempt, res)
		}
	}
}

func (n *Node) reply(to string, c Command, attempt int, res session) {
	n.send(to, Reply{Client: c.Client, Seq: c.Seq, Attempt: attempt, OK: true, Result: res.result, CasOK: res.casOK})
}
