package agent

import (
	"maps"

	"github.com/ayush00git/quorum/pkg/experiment"
)

// Tool names.
const (
	ToolClusterInfo    = "cluster_info"
	ToolRunExperiment  = "run_experiment"
	ToolInspectHistory = "inspect_history"
	ToolInspectEvents  = "inspect_events"
	ToolShrink         = "shrink"
	ToolReportFinding  = "report_finding"
	ToolConclude       = "conclude"
)

// Inspection output caps.
const (
	maxInspectLines = 60
)

func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

var experimentIDProp = strProp("Id of an experiment run earlier in this session.")

var specSchema = map[string]any{
	"type":        "object",
	"description": "Experiment spec. Omitted fields take the defaults from cluster_info. target defaults to this session's target.",
	"properties": map[string]any{
		"seed":        intProp("PRNG seed. Same spec and seed always give the same result."),
		"durationMs":  intProp("Virtual duration in ms (max 10000)."),
		"clients":     intProp("Concurrent client processes."),
		"keys":        intProp("Number of keys (x, y, z, ...)."),
		"readRatio":   map[string]any{"type": "number", "description": "Fraction of operations that are reads, 0..1."},
		"readFrom":    map[string]any{"type": "string", "enum": []string{"leader", "any"}, "description": "leader: reads go to the believed leader; any: reads go to a random node."},
		"opTimeoutMs": intProp("Client operation timeout in ms; timed-out ops are indeterminate (info)."),
		"network": map[string]any{
			"type":        "object",
			"description": "Initial network model.",
			"properties": map[string]any{
				"minLatencyMs":  intProp("Minimum one-way latency."),
				"maxLatencyMs":  intProp("Maximum one-way latency."),
				"dropRate":      map[string]any{"type": "number", "description": "Message drop probability 0..1."},
				"duplicateRate": map[string]any{"type": "number", "description": "Message duplication probability 0..1."},
			},
		},
		"faults": map[string]any{
			"type":        "array",
			"description": "Fault schedule (max 30). See cluster_info for each action's args.",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"atMs", "action", "args"},
				"properties": map[string]any{
					"atMs": intProp("Virtual time the fault fires."),
					"action": map[string]any{"type": "string", "enum": []string{
						experiment.ActionPartition, experiment.ActionIsolate, experiment.ActionHeal,
						experiment.ActionCrash, experiment.ActionRestart, experiment.ActionPause,
						experiment.ActionSetNetwork,
					}},
					"args": map[string]any{"type": "object", "description": "Action arguments, e.g. {\"node\":\"n2\"}; {} for heal."},
				},
			},
		},
	},
}

var rangeProps = map[string]any{
	"experimentId": experimentIDProp,
	"fromMs":       intProp("Only include entries at or after this time."),
	"toMs":         intProp("Only include entries at or before this time."),
	"limit":        intProp("Maximum lines to return (max 60)."),
}

func withProps(base map[string]any, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	maps.Copy(out, base)
	maps.Copy(out, extra)
	return out
}

// Tools returns the server-owned tool definitions in a fixed order (the
// order is part of the prompt cache prefix).
func Tools() []Tool {
	return []Tool{
		{
			Name:        ToolClusterInfo,
			Description: "Describe the cluster under test: node ids, target codename, default spec, limits, and every fault action with its argument shape. Call this first.",
			Properties:  map[string]any{},
		},
		{
			Name:        ToolRunExperiment,
			Description: "Run one deterministic experiment: a workload against the 5-node cluster under your fault schedule, then a linearizability check. Returns the experiment id and a compact summary (verdict, op counts, leader timeline, notable events, and on violation the checker explanation and offending op ids).",
			Properties:  map[string]any{"spec": specSchema},
			Required:    []string{"spec"},
		},
		{
			Name:        ToolInspectHistory,
			Description: "List client operations of an experiment, one line per op (max 60 lines): id, process, op, status, invoke and complete times, node. Ops the checker flagged are marked: '*' cannot be linearized, '!' conflicts with one that cannot.",
			Properties:  withProps(rangeProps, map[string]any{"key": strProp("Only include ops on this key.")}),
			Required:    []string{"experimentId"},
		},
		{
			Name:        ToolInspectEvents,
			Description: "List the simulator event log of an experiment (max 60 lines): elections, votes, leadership changes, commits, crashes, restarts, pauses, partitions, network changes.",
			Properties:  withProps(rangeProps, map[string]any{"node": strProp("Only include events on this node.")}),
			Required:    []string{"experimentId"},
		},
		{
			Name:        ToolShrink,
			Description: "Minimize a violating experiment: drop faults, shorten the duration, reduce clients and keys while the violation persists. Returns the minimal spec, its experiment id and its summary. Only works on experiments whose verdict is violation.",
			Properties:  map[string]any{"experimentId": experimentIDProp},
			Required:    []string{"experimentId"},
		},
		{
			Name:        ToolReportFinding,
			Description: "End the run by reporting a checker-confirmed linearizability violation. Rejected unless the experiment's verdict is violation. Report the shrunk experiment.",
			Properties: map[string]any{
				"experimentId": experimentIDProp,
				"title":        strProp("One-line title of the bug."),
				"anomaly":      strProp("What the clients observed that is not linearizable, with op ids, values and times."),
				"causalChain": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Ordered steps from fault to anomaly, each with times and node ids.",
				},
			},
			Required: []string{"experimentId", "title", "anomaly", "causalChain"},
		},
		{
			Name:        ToolConclude,
			Description: "End the run without a finding: no checker-confirmed violation was found within budget. Summarize what you tried and why you believe the system held up.",
			Properties:  map[string]any{"summary": strProp("What was tested and what was learned.")},
			Required:    []string{"summary"},
		},
	}
}
