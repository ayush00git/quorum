package agent

import "github.com/ayush00git/quorum/pkg/experiment"

// Engine is the slice of the experiment core the agent and handlers use.
// It exists so tests can swap in a fake core.
type Engine interface {
	ParseSpec(data []byte) (experiment.Spec, error)
	SpecID(s experiment.Spec) string
	Run(s experiment.Spec) (experiment.Result, error)
	Summarize(r experiment.Result) string
	Shrink(s experiment.Spec) (experiment.ShrinkResult, error)
	Targets() []experiment.TargetInfo
	RevealBug(codename string) (string, bool)
	DefaultSpec(target string) experiment.Spec
}

// CoreEngine delegates to the real experiment package.
type CoreEngine struct{}

var _ Engine = CoreEngine{}

// ParseSpec implements Engine.
func (CoreEngine) ParseSpec(data []byte) (experiment.Spec, error) { return experiment.ParseSpec(data) }

// SpecID implements Engine.
func (CoreEngine) SpecID(s experiment.Spec) string { return experiment.SpecID(s) }

// Run implements Engine.
func (CoreEngine) Run(s experiment.Spec) (experiment.Result, error) { return experiment.Run(s) }

// Summarize implements Engine.
func (CoreEngine) Summarize(r experiment.Result) string { return experiment.Summarize(r) }

// Shrink implements Engine.
func (CoreEngine) Shrink(s experiment.Spec) (experiment.ShrinkResult, error) {
	return experiment.Shrink(s)
}

// Targets implements Engine.
func (CoreEngine) Targets() []experiment.TargetInfo { return experiment.Targets() }

// RevealBug implements Engine.
func (CoreEngine) RevealBug(codename string) (string, bool) { return experiment.RevealBug(codename) }

// DefaultSpec implements Engine.
func (CoreEngine) DefaultSpec(target string) experiment.Spec { return experiment.DefaultSpec(target) }

// KnownTarget reports whether codename is a public target.
func KnownTarget(e Engine, codename string) bool {
	for _, t := range e.Targets() {
		if t.Codename == codename {
			return true
		}
	}
	return false
}
