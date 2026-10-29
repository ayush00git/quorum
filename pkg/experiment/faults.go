package experiment

import (
	"fmt"
	"strings"

	"github.com/ayush00git/quorum/pkg/sim"
)

// Actions lists the fault actions in documentation order.
var Actions = []string{ActionPartition, ActionIsolate, ActionHeal, ActionCrash, ActionRestart, ActionPause, ActionSetNetwork, ActionSkewClock}

// allowedArgs maps each action to the argument fields it accepts.
var allowedArgs = map[string][]string{
	ActionPartition:  {"groups"},
	ActionIsolate:    {"node"},
	ActionHeal:       {},
	ActionCrash:      {"node"},
	ActionRestart:    {"node"},
	ActionPause:      {"node", "durationMs"},
	ActionSetNetwork: {"minLatencyMs", "maxLatencyMs", "dropRate", "duplicateRate"},
	ActionSkewClock:  {"node", "driftPpm"},
}

// NodeIDs returns n1..nN.
func NodeIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i+1)
	}
	return ids
}

// KeyNames returns x, y, z, then a, b, ...
func KeyNames(n int) []string {
	names := []string{"x", "y", "z", "a", "b", "c", "d", "e"}
	return names[:n]
}

func setArgs(a FaultArgs) []string {
	var out []string
	add := func(set bool, name string) {
		if set {
			out = append(out, name)
		}
	}
	add(a.Groups != nil, "groups")
	add(a.Node != "", "node")
	add(a.DurationMs != 0, "durationMs")
	add(a.MinLatencyMs != nil, "minLatencyMs")
	add(a.MaxLatencyMs != nil, "maxLatencyMs")
	add(a.DropRate != nil, "dropRate")
	add(a.DuplicateRate != nil, "duplicateRate")
	add(a.DriftPpm != 0, "driftPpm")
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// validateFault checks one fault. net is the network state so far and is
// updated by setNetwork, so combined settings stay consistent.
func validateFault(s Spec, f Fault, net *Network) error {
	if f.AtMs < 0 || f.AtMs >= s.DurationMs {
		return fmt.Errorf("atMs: must be between 0 and durationMs-1 (%d), got %d", s.DurationMs-1, f.AtMs)
	}
	allowed, ok := allowedArgs[f.Action]
	if !ok {
		return fmt.Errorf("action: unknown action %q (want one of %s)", f.Action, strings.Join(Actions, ", "))
	}
	for _, name := range setArgs(f.Args) {
		if !contains(allowed, name) {
			return fmt.Errorf("args.%s: not allowed for %s", name, f.Action)
		}
	}
	nodes := NodeIDs(s.Nodes)
	a := f.Args
	checkNode := func() error {
		if a.Node == "" {
			return fmt.Errorf("args.node: required for %s", f.Action)
		}
		if !contains(nodes, a.Node) {
			return fmt.Errorf("args.node: unknown node %q (want one of %s)", a.Node, strings.Join(nodes, ", "))
		}
		return nil
	}
	switch f.Action {
	case ActionIsolate, ActionCrash, ActionRestart:
		return checkNode()
	case ActionPause:
		if err := checkNode(); err != nil {
			return err
		}
		if a.DurationMs < 1 || a.DurationMs > MaxPauseMs {
			return fmt.Errorf("args.durationMs: must be between 1 and %d, got %d", MaxPauseMs, a.DurationMs)
		}
	case ActionSkewClock:
		if err := checkNode(); err != nil {
			return err
		}
		if a.DriftPpm < -MaxDriftPpm || a.DriftPpm > MaxDriftPpm {
			return fmt.Errorf("args.driftPpm: must be between %d and %d, got %d", -MaxDriftPpm, MaxDriftPpm, a.DriftPpm)
		}
	case ActionPartition:
		return validateGroups(a.Groups, nodes)
	case ActionSetNetwork:
		return applyPatch(net, a)
	}
	return nil
}

func validateGroups(groups [][]string, nodes []string) error {
	if len(groups) < 2 {
		return fmt.Errorf("args.groups: need at least 2 groups, got %d", len(groups))
	}
	seen := map[string]bool{}
	for i, g := range groups {
		if len(g) == 0 {
			return fmt.Errorf("args.groups[%d]: empty group", i)
		}
		for _, id := range g {
			if !contains(nodes, id) {
				return fmt.Errorf("args.groups[%d]: unknown node %q (want one of %s)", i, id, strings.Join(nodes, ", "))
			}
			if seen[id] {
				return fmt.Errorf("args.groups[%d]: node %q appears in more than one group", i, id)
			}
			seen[id] = true
		}
	}
	return nil
}

func applyPatch(net *Network, a FaultArgs) error {
	if a.MinLatencyMs == nil && a.MaxLatencyMs == nil && a.DropRate == nil && a.DuplicateRate == nil {
		return fmt.Errorf("args: setNetwork needs at least one of minLatencyMs, maxLatencyMs, dropRate, duplicateRate")
	}
	n := *net
	if a.MinLatencyMs != nil {
		n.MinLatencyMs = *a.MinLatencyMs
	}
	if a.MaxLatencyMs != nil {
		n.MaxLatencyMs = *a.MaxLatencyMs
	}
	if a.DropRate != nil {
		n.DropRate = *a.DropRate
	}
	if a.DuplicateRate != nil {
		n.DuplicateRate = *a.DuplicateRate
	}
	if err := validateNetwork("args", n.MinLatencyMs, n.MaxLatencyMs, n.DropRate, n.DuplicateRate); err != nil {
		return err
	}
	*net = n
	return nil
}

// schedule registers every fault with the world.
func schedule(w *sim.World, faults []Fault) {
	for _, f := range faults {
		a := f.Args
		var fn func()
		switch f.Action {
		case ActionPartition:
			fn = func() { w.Partition(a.Groups) }
		case ActionIsolate:
			fn = func() { w.Isolate(a.Node) }
		case ActionHeal:
			fn = w.Heal
		case ActionCrash:
			fn = func() { w.Crash(a.Node) }
		case ActionRestart:
			fn = func() { w.Restart(a.Node) }
		case ActionPause:
			fn = func() { w.Pause(a.Node, a.DurationMs) }
		case ActionSetNetwork:
			fn = func() {
				w.SetNetwork(sim.NetworkPatch{MinLatencyMs: a.MinLatencyMs, MaxLatencyMs: a.MaxLatencyMs, DropRate: a.DropRate, DuplicateRate: a.DuplicateRate})
			}
		case ActionSkewClock:
			fn = func() { w.SkewClock(a.Node, a.DriftPpm) }
		}
		w.At(f.AtMs, fn)
	}
}
