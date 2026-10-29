package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ayush00git/quorum/pkg/experiment"
)

var faultDocs = []struct{ action, args, effect string }{
	{experiment.ActionPartition, `{"groups":[["n1","n2"],["n3","n4","n5"]]}`, "split the network into groups; messages between groups are dropped, including in-flight ones"},
	{experiment.ActionIsolate, `{"node":"n1"}`, "cut one node off from all others"},
	{experiment.ActionHeal, `{}`, "remove every partition"},
	{experiment.ActionCrash, `{"node":"n1"}`, "crash a node: volatile state is lost, whatever it persisted to disk survives"},
	{experiment.ActionRestart, `{"node":"n1"}`, "restart a crashed node from its disk"},
	{experiment.ActionPause, `{"node":"n1","durationMs":400}`, "freeze a node (GC-style): it processes nothing, messages queue and timers fire late, then it resumes"},
	{experiment.ActionSetNetwork, `{"minLatencyMs":1,"maxLatencyMs":80,"dropRate":0.1,"duplicateRate":0.05}`, "change the network model from this time on; every field is optional"},
}

// clusterInfo describes the system under test. It never reveals which bug
// (if any) a target carries.
func clusterInfo(engine Engine, target string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "target: %s\n", target)
	fmt.Fprintf(&b, "nodes: %s\n", strings.Join(nodeIDs(engine, target), " "))
	def, _ := json.Marshal(engine.DefaultSpec(target))
	fmt.Fprintf(&b, "default spec: %s\n", def)
	fmt.Fprintf(&b, "limits: durationMs <= %d, faults <= %d, inspect output <= %d lines\n",
		experiment.MaxDurationMs, experiment.MaxFaults, maxInspectLines)
	b.WriteString("fault actions (faults: [{atMs, action, args}]):\n")
	for _, f := range faultDocs {
		fmt.Fprintf(&b, "  %s %s: %s\n", f.action, f.args, f.effect)
	}
	b.WriteString("workload: each client process has at most one outstanding op; ops are reads and writes on keys x, y, z, ...; write values are unique per key. ")
	b.WriteString("Clients follow leader hints and retry. An op that times out is indeterminate (status info) and its process retires; status fail means the op definitely did not take effect.\n")
	b.WriteString("checker: per-key linearizability of the recorded history; verdict ok, violation or unknown (checker timeout).")
	return b.String()
}
