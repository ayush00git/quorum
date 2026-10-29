package agent

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/experiment"
)

// registry maps experiment ids to specs for one request. It is rebuilt
// from the tool_use blocks in the message history on every request:
// experiments are deterministic and their id is the hash of their spec,
// so tool results sent back by the client are never trusted.
type registry struct {
	engine  Engine
	target  string
	specs   map[string]experiment.Spec
	results map[string]experiment.Result
	shrunk  map[string]experiment.ShrinkResult // keyed by source id
	pending []string                           // shrink source ids not yet resolved
}

func newRegistry(engine Engine, target string) *registry {
	return &registry{
		engine:  engine,
		target:  target,
		specs:   map[string]experiment.Spec{},
		results: map[string]experiment.Result{},
		shrunk:  map[string]experiment.ShrinkResult{},
	}
}

// scan registers every run_experiment spec and queues every shrink call
// found in assistant messages. Invalid calls are skipped: they failed
// when they were made, too.
func (g *registry) scan(messages []Message) {
	for _, m := range messages {
		if m.Role != RoleAssistant {
			continue
		}
		for _, tu := range toolUses(m.Content) {
			switch tu.Name {
			case ToolRunExperiment:
				var in runInput
				if decodeInput(tu.Input, &in) != nil {
					continue
				}
				if spec, err := parseToolSpec(g.engine, in.Spec, g.target); err == nil {
					g.register(spec)
				}
			case ToolShrink:
				var in idInput
				if decodeInput(tu.Input, &in) == nil && in.ExperimentID != "" {
					g.pending = append(g.pending, in.ExperimentID)
				}
			}
		}
	}
}

func (g *registry) register(spec experiment.Spec) string {
	id := g.engine.SpecID(spec)
	g.specs[id] = spec
	return id
}

// spec looks up an id, lazily replaying earlier shrink calls if needed.
func (g *registry) spec(id string) (experiment.Spec, bool) {
	if s, ok := g.specs[id]; ok {
		return s, true
	}
	for len(g.pending) > 0 {
		src := g.pending[0]
		g.pending = g.pending[1:]
		if _, err := g.shrink(src); err != nil {
			continue
		}
		if s, ok := g.specs[id]; ok {
			return s, true
		}
	}
	return experiment.Spec{}, false
}

func errUnknownID(id string) error {
	return fmt.Errorf("unknown experimentId %q: use an id returned by run_experiment or shrink in this session", id)
}

func (g *registry) result(id string) (experiment.Result, error) {
	if r, ok := g.results[id]; ok {
		return r, nil
	}
	spec, ok := g.spec(id)
	if !ok {
		return experiment.Result{}, errUnknownID(id)
	}
	r, err := g.engine.Run(spec)
	if err != nil {
		return experiment.Result{}, fmt.Errorf("run experiment %s: %w", id, err)
	}
	g.results[id] = r
	return r, nil
}

func (g *registry) run(spec experiment.Spec) (experiment.Result, error) {
	return g.result(g.register(spec))
}

// shrink minimizes the experiment with the given id and registers the result.
func (g *registry) shrink(id string) (experiment.ShrinkResult, error) {
	if sr, ok := g.shrunk[id]; ok {
		return sr, nil
	}
	src, err := g.result(id)
	if err != nil {
		return experiment.ShrinkResult{}, err
	}
	if src.Check.Verdict != checker.VerdictViolation {
		return experiment.ShrinkResult{}, fmt.Errorf("experiment %s has verdict %q; shrink only works on violating experiments", id, src.Check.Verdict)
	}
	sr, err := g.engine.Shrink(src.Spec)
	if err != nil {
		return experiment.ShrinkResult{}, fmt.Errorf("shrink %s: %w", id, err)
	}
	g.shrunk[id] = sr
	newID := g.register(sr.Spec)
	g.results[newID] = sr.Result
	return sr, nil
}

// parseToolSpec strictly parses a spec from a tool input, pinning it to
// the session target.
func parseToolSpec(engine Engine, raw json.RawMessage, target string) (experiment.Spec, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return experiment.Spec{}, errors.New("spec must be a JSON object")
	}
	if t, ok := fields["target"]; ok {
		var got string
		if err := json.Unmarshal(t, &got); err != nil {
			return experiment.Spec{}, errors.New("spec.target must be a string")
		}
		if got != target {
			return experiment.Spec{}, fmt.Errorf("spec.target is %q but this session tests %q; omit target", got, target)
		}
	}
	fields["target"] = mustJSON(target)
	spec, err := engine.ParseSpec(mustJSON(fields))
	if err != nil {
		return experiment.Spec{}, fmt.Errorf("invalid spec: %w", err)
	}
	return spec, nil
}
