// Package agent runs the adversary: one Claude call per step, with the
// tools executed in-process against the deterministic experiment core.
// Payload types mirror docs/api.md exactly.
package agent

import (
	"encoding/json"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/experiment"
)

// Message roles accepted from the browser.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is one Anthropic Messages API message. Content blocks stay raw
// JSON so every block (including thinking blocks) round-trips untouched.
type Message struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

// Request is the body of POST /api/agent.
type Request struct {
	Target     string    `json:"target"`
	MaxSteps   int       `json:"maxSteps,omitempty"`
	Seed       *int64    `json:"seed,omitempty"`
	Messages   []Message `json:"messages"`
	AccessCode string    `json:"accessCode,omitempty"`
}

// Response is the body returned by POST /api/agent.
type Response struct {
	Kickoff     *Message            `json:"kickoff,omitempty"`
	Assistant   []json.RawMessage   `json:"assistant"`
	ToolResults []json.RawMessage   `json:"toolResults"`
	Experiments []ExperimentSummary `json:"experiments"`
	Usage       Usage               `json:"usage"`
	Step        int                 `json:"step"`
	Done        bool                `json:"done"`
	Finding     *Finding            `json:"finding,omitempty"`
	Conclusion  string              `json:"conclusion,omitempty"`
	RevealedBug string              `json:"revealedBug,omitempty"`
}

// ExperimentSummary describes one experiment run (or shrunk) in a step.
type ExperimentSummary struct {
	ID         string          `json:"id"`
	Spec       experiment.Spec `json:"spec"`
	Verdict    string          `json:"verdict"`
	Ops        int             `json:"ops"`
	Faults     int             `json:"faults"`
	DurationMs int64           `json:"durationMs"`
	ShrunkFrom string          `json:"shrunkFrom"`
}

// Usage counts tokens for one step (or a whole recorded run).
type Usage struct {
	InputTokens         int64 `json:"inputTokens"`
	OutputTokens        int64 `json:"outputTokens"`
	CacheReadTokens     int64 `json:"cacheReadTokens"`
	CacheCreationTokens int64 `json:"cacheCreationTokens"`
}

// Add accumulates u2 into u.
func (u *Usage) Add(u2 Usage) {
	u.InputTokens += u2.InputTokens
	u.OutputTokens += u2.OutputTokens
	u.CacheReadTokens += u2.CacheReadTokens
	u.CacheCreationTokens += u2.CacheCreationTokens
}

// Finding is a checker-confirmed violation reported by the agent.
type Finding struct {
	ExperimentID string              `json:"experimentId"`
	Title        string              `json:"title"`
	Anomaly      string              `json:"anomaly"`
	CausalChain  []string            `json:"causalChain"`
	Spec         experiment.Spec     `json:"spec"`
	Check        checker.CheckResult `json:"check"`
}

func summarize(r experiment.Result, shrunkFrom string) ExperimentSummary {
	return ExperimentSummary{
		ID:         r.ID,
		Spec:       r.Spec,
		Verdict:    r.Check.Verdict,
		Ops:        len(r.History),
		Faults:     len(r.Spec.Faults),
		DurationMs: r.Spec.DurationMs,
		ShrunkFrom: shrunkFrom,
	}
}
