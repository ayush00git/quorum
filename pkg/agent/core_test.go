package agent

import (
	"strings"
	"testing"

	"github.com/ayush00git/quorum/pkg/experiment"
)

// TestCoreEngineSmoke drives the executor against the real experiment core.
func TestCoreEngineSmoke(t *testing.T) {
	e := CoreEngine{}
	targets := e.Targets()
	if len(targets) == 0 {
		t.Fatal("core exposes no targets")
	}
	var bugs []string
	for _, tg := range targets {
		if bug, ok := e.RevealBug(tg.Codename); ok && bug != "none" {
			bugs = append(bugs, strings.ToLower(bug))
		}
	}
	for _, tg := range targets {
		info := strings.ToLower(clusterInfo(e, tg.Codename))
		for _, bug := range bugs {
			if strings.Contains(info, bug) {
				t.Errorf("cluster_info for %s mentions bug %q", tg.Codename, bug)
			}
		}
		if strings.Contains(info, "difficulty") {
			t.Errorf("cluster_info for %s mentions difficulty", tg.Codename)
		}

		x := newExecutor(e, tg.Codename, newRegistry(e, tg.Codename), 1, 30)
		spec := map[string]any{"seed": 3, "durationMs": 1500, "faults": []any{
			map[string]any{"atMs": 400, "action": experiment.ActionIsolate, "args": map[string]any{"node": "n1"}},
			map[string]any{"atMs": 900, "action": experiment.ActionHeal, "args": map[string]any{}},
		}}
		out, err := x.execute(toolUse{Name: ToolRunExperiment, Input: mustJSON(map[string]any{"spec": spec})})
		if err != nil {
			t.Fatalf("%s: run_experiment: %v", tg.Codename, err)
		}
		id := x.experiments[0].ID
		if !strings.HasPrefix(out, "experiment "+id) {
			t.Fatalf("%s: result must start with the experiment id:\n%s", tg.Codename, out)
		}
		for _, tool := range []string{ToolInspectHistory, ToolInspectEvents} {
			if _, err := x.execute(toolUse{Name: tool, Input: mustJSON(map[string]any{"experimentId": id, "limit": 10})}); err != nil {
				t.Fatalf("%s: %s: %v", tg.Codename, tool, err)
			}
		}
	}
}
