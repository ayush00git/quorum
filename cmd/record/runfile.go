package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/ayush00git/quorum/pkg/agent"
	"github.com/ayush00git/quorum/pkg/experiment"
)

// runFile is the recorded run format documented in docs/api.md.
type runFile struct {
	Version       int               `json:"version"`
	Target        string            `json:"target"`
	Model         string            `json:"model"`
	StartedAt     string            `json:"startedAt"`
	Steps         []recordedStep    `json:"steps"`
	Experiments   []experiment.Spec `json:"experiments"`
	Finding       *agent.Finding    `json:"finding"`
	Conclusion    *string           `json:"conclusion"`
	RevealedBug   string            `json:"revealedBug"`
	Usage         agent.Usage       `json:"usage"`
	HistoryHashes map[string]string `json:"historyHashes"`
}

type recordedStep struct {
	Assistant   []json.RawMessage         `json:"assistant"`
	ToolResults []json.RawMessage         `json:"toolResults"`
	Experiments []agent.ExperimentSummary `json:"experiments"`
}

// indexEntry is one row of public/runs/index.json.
type indexEntry struct {
	ID     string `json:"id"`
	Target string `json:"target"`
	Steps  int    `json:"steps"`
	Found  bool   `json:"found"`
}

// collectExperiments lists every distinct experiment spec in step order
// and recomputes its history hash so replays can verify determinism.
func collectExperiments(engine agent.Engine, rf *runFile) error {
	rf.Experiments = []experiment.Spec{}
	rf.HistoryHashes = map[string]string{}
	for _, st := range rf.Steps {
		for _, ex := range st.Experiments {
			if _, seen := rf.HistoryHashes[ex.ID]; seen {
				continue
			}
			res, err := engine.Run(ex.Spec)
			if err != nil {
				return fmt.Errorf("rerun experiment %s: %w", ex.ID, err)
			}
			rf.Experiments = append(rf.Experiments, ex.Spec)
			rf.HistoryHashes[ex.ID] = res.HistoryHash
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// saveRun writes <dir>/<id>.json and upserts the run into <dir>/index.json.
func saveRun(dir, id string, rf runFile) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, id+".json")
	if err := writeJSON(path, rf); err != nil {
		return "", err
	}

	indexPath := filepath.Join(dir, "index.json")
	var index []indexEntry
	data, err := os.ReadFile(indexPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", fmt.Errorf("read %s: %w", indexPath, err)
	default:
		if err := json.Unmarshal(data, &index); err != nil {
			return "", fmt.Errorf("parse %s: %w", indexPath, err)
		}
	}
	entry := indexEntry{ID: id, Target: rf.Target, Steps: len(rf.Steps), Found: rf.Finding != nil}
	replaced := false
	for i := range index {
		if index[i].ID == id {
			index[i], replaced = entry, true
		}
	}
	if !replaced {
		index = append(index, entry)
	}
	sort.Slice(index, func(i, j int) bool { return index[i].ID < index[j].ID })
	return path, writeJSON(indexPath, index)
}
