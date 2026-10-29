package raft

import "sort"

// Bugs are the planted-bug flags. The zero value is correct Raft.
type Bugs struct {
	FollowerReads    bool // followers answer reads from local state
	StaleLeaderReads bool // the leader answers reads locally, bypassing the log
	EarlyAck         bool // writes are acknowledged before majority replication
	AmnesiacVoter    bool // votedFor is not persisted
	PrevTermCommit   bool // earlier-term entries commit by counting replicas
	LeaseReads       bool // reads served under a lease that assumes no clock drift
}

// Target is one cluster variant. Only Codename and Difficulty are public;
// Bug is revealed only after a run ends.
type Target struct {
	Codename   string
	Difficulty string
	Bug        string
	Flags      Bugs
	Enabled    bool // listed publicly; stretch targets stay off until verified
}

// targets is the single source of the codename-to-bug mapping. The bug
// targets are deliberately shuffled relative to the plan's ordering.
var targets = map[string]Target{
	"target-a": {Enabled: true, Difficulty: "n/a", Bug: "none"},
	"target-b": {Enabled: true, Difficulty: "medium", Bug: "stale leader reads: the leader answers reads from local state without going through the log or confirming it is still leader", Flags: Bugs{StaleLeaderReads: true}},
	"target-c": {Enabled: true, Difficulty: "medium", Bug: "early ack: the leader acknowledges a write once it is in its own log, before majority replication", Flags: Bugs{EarlyAck: true}},
	"target-d": {Enabled: true, Difficulty: "hard", Bug: "amnesiac voter: votedFor is not persisted, so a restarted node can vote twice in one term", Flags: Bugs{AmnesiacVoter: true}},
	"target-e": {Enabled: true, Difficulty: "easy", Bug: "follower reads: followers answer reads from their local state machine", Flags: Bugs{FollowerReads: true}},
	"target-f": {Enabled: true, Difficulty: "hard", Bug: "lease reads under clock skew: the leader serves reads under a time-based lease that assumes clocks never drift", Flags: Bugs{LeaseReads: true}},
	"target-g": {Difficulty: "very hard", Bug: "previous-term commit: the leader commits earlier-term entries by counting replicas (the Raft paper's Figure 8 bug)", Flags: Bugs{PrevTermCommit: true}},
}

// LookupTarget returns a target by codename.
func LookupTarget(codename string) (Target, bool) {
	t, ok := targets[codename]
	t.Codename = codename
	return t, ok
}

// Codenames lists every target codename in sorted order.
func Codenames() []string {
	out := make([]string, 0, len(targets))
	for c := range targets {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
