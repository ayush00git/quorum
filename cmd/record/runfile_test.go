package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ayush00git/quorum/pkg/agent"
	"github.com/ayush00git/quorum/pkg/agent/agenttest"
	"github.com/ayush00git/quorum/pkg/experiment"
)

func TestSaveRun(t *testing.T) {
	engine := &agenttest.FakeEngine{}
	spec := experiment.Spec{Target: "target-c", Seed: 1, DurationMs: 3000}
	res, _ := engine.Run(spec)
	summary := agent.ExperimentSummary{ID: res.ID, Spec: spec, Verdict: res.Check.Verdict}
	rf := runFile{Version: 1, Target: "target-c", Steps: []recordedStep{
		{Experiments: []agent.ExperimentSummary{summary}},
		{Experiments: []agent.ExperimentSummary{summary}},
	}, Finding: &agent.Finding{ExperimentID: res.ID}}
	if err := collectExperiments(engine, &rf); err != nil {
		t.Fatal(err)
	}
	if len(rf.Experiments) != 1 || rf.HistoryHashes[res.ID] != res.HistoryHash {
		t.Fatalf("experiments not deduplicated or hashed: %+v %v", rf.Experiments, rf.HistoryHashes)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`[{"id":"target-a-1","target":"target-a","steps":9,"found":false},{"id":"target-c-42","target":"target-c","steps":1,"found":false}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := saveRun(dir, "target-c-42", rf)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &back); err != nil || back["conclusion"] != nil || back["version"] != float64(1) {
		t.Fatalf("bad run file: %s", data)
	}
	var index []indexEntry
	data, _ = os.ReadFile(filepath.Join(dir, "index.json"))
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	want := []indexEntry{{"target-a-1", "target-a", 9, false}, {"target-c-42", "target-c", 2, true}}
	if len(index) != 2 || index[0] != want[0] || index[1] != want[1] {
		t.Fatalf("index not upserted: %+v", index)
	}
}
