// Package sim is the deterministic discrete-event simulator.
package sim

// Event is one entry in the simulation event log.
type Event struct {
	AtMs   int64  `json:"atMs"`
	Node   string `json:"node,omitempty"` // empty for cluster-wide events (partition, heal, network)
	Kind   string `json:"kind"`           // see the Event* constants
	Term   int    `json:"term,omitempty"`
	Detail string `json:"detail,omitempty"` // short human-readable detail
}

// Event kinds. The UI keys off these strings, so keep them stable.
const (
	EventElectionStarted = "election-started"
	EventVoteGranted     = "vote-granted"
	EventBecameLeader    = "became-leader"
	EventSteppedDown     = "stepped-down"
	EventCommitAdvanced  = "commit-advanced"
	EventCrash           = "crash"
	EventRestart         = "restart"
	EventPauseStart      = "pause-start"
	EventPauseEnd        = "pause-end"
	EventPartition       = "partition"
	EventHeal            = "heal"
	EventNetworkChange   = "network-change"
	EventClockSkew       = "clock-skew"
)
