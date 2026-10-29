// Package experiment defines experiment specs and runs, summarizes and shrinks them.
package experiment

import (
	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/sim"
	"github.com/ayush00git/quorum/pkg/workload"
)

// Limits enforced by spec validation.
const (
	MaxDurationMs = 10000
	MaxFaults     = 30
	NodeCount     = 5
)

// Fault actions.
const (
	ActionPartition  = "partition"
	ActionIsolate    = "isolate"
	ActionHeal       = "heal"
	ActionCrash      = "crash"
	ActionRestart    = "restart"
	ActionPause      = "pause"
	ActionSetNetwork = "setNetwork"
	ActionSkewClock  = "skewClock" // stretch
)

// Network holds the network model parameters.
type Network struct {
	MinLatencyMs  int64   `json:"minLatencyMs"`
	MaxLatencyMs  int64   `json:"maxLatencyMs"`
	DropRate      float64 `json:"dropRate"`
	DuplicateRate float64 `json:"duplicateRate"`
}

// FaultArgs is the union of all fault action arguments. Which fields are
// allowed depends on the action; validation rejects the rest.
type FaultArgs struct {
	Groups        [][]string `json:"groups,omitempty"`        // partition
	Node          string     `json:"node,omitempty"`          // isolate, crash, restart, pause, skewClock
	DurationMs    int64      `json:"durationMs,omitempty"`    // pause
	MinLatencyMs  *int64     `json:"minLatencyMs,omitempty"`  // setNetwork
	MaxLatencyMs  *int64     `json:"maxLatencyMs,omitempty"`  // setNetwork
	DropRate      *float64   `json:"dropRate,omitempty"`      // setNetwork
	DuplicateRate *float64   `json:"duplicateRate,omitempty"` // setNetwork
	DriftPpm      int64      `json:"driftPpm,omitempty"`      // skewClock
}

// Fault is one scheduled fault.
type Fault struct {
	AtMs   int64     `json:"atMs"`
	Action string    `json:"action"`
	Args   FaultArgs `json:"args"`
}

// Spec fully determines an experiment. Same Spec, byte-identical Result.
type Spec struct {
	Seed        int64   `json:"seed"`
	Target      string  `json:"target"`
	DurationMs  int64   `json:"durationMs"`
	Nodes       int     `json:"nodes"`
	Clients     int     `json:"clients"`
	Keys        int     `json:"keys"`
	ReadRatio   float64 `json:"readRatio"`
	ReadFrom    string  `json:"readFrom"` // leader | any
	OpTimeoutMs int64   `json:"opTimeoutMs"`
	Network     Network `json:"network"`
	Faults      []Fault `json:"faults"`
}

// LeaderSpan records which node led in which term, over [FromMs, ToMs).
type LeaderSpan struct {
	Term   int    `json:"term"`
	Node   string `json:"node"`
	FromMs int64  `json:"fromMs"`
	ToMs   int64  `json:"toMs"`
}

// Stats are aggregate counters for a run.
type Stats struct {
	Ops             int          `json:"ops"`
	OK              int          `json:"ok"`
	Fail            int          `json:"fail"`
	Info            int          `json:"info"`
	Reads           int          `json:"reads"`
	Writes          int          `json:"writes"`
	MessagesSent    int          `json:"messagesSent"`
	MessagesDropped int          `json:"messagesDropped"`
	Leaders         []LeaderSpan `json:"leaders"`
}

// Result is the full output of Run.
type Result struct {
	ID          string              `json:"id"`
	Spec        Spec                `json:"spec"`
	History     []workload.Op       `json:"history"`
	Events      []sim.Event         `json:"events"`
	Check       checker.CheckResult `json:"check"`
	Stats       Stats               `json:"stats"`
	HistoryHash string              `json:"historyHash"`
}

// ShrinkResult is the output of Shrink.
type ShrinkResult struct {
	Spec   Spec   `json:"spec"`
	Result Result `json:"result"`
	Runs   int    `json:"runs"` // experiments executed while shrinking
}

// TargetInfo is the public description of a target. Never includes the bug.
type TargetInfo struct {
	Codename   string `json:"codename"`
	Difficulty string `json:"difficulty"`
}
