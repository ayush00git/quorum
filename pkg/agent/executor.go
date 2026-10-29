package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ayush00git/quorum/pkg/checker"
)

// Input size limits for free-text tool fields.
const (
	maxTitleLen     = 200
	maxAnomalyLen   = 2000
	maxChainSteps   = 20
	maxChainStepLen = 600
	maxSummaryLen   = 4000
)

type runInput struct {
	Spec json.RawMessage `json:"spec"`
}

type idInput struct {
	ExperimentID string `json:"experimentId"`
}

type findingInput struct {
	ExperimentID string   `json:"experimentId"`
	Title        string   `json:"title"`
	Anomaly      string   `json:"anomaly"`
	CausalChain  []string `json:"causalChain"`
}

type concludeInput struct {
	Summary string `json:"summary"`
}

// executor runs the tool calls of one step.
type executor struct {
	engine    Engine
	target    string
	reg       *registry
	stepsUsed int
	stepsLeft int

	experiments []ExperimentSummary
	finding     *Finding
	conclusion  string
	ended       bool
}

func newExecutor(engine Engine, target string, reg *registry, step, budget int) *executor {
	return &executor{engine: engine, target: target, reg: reg, stepsUsed: step, stepsLeft: budget - step}
}

func (x *executor) budgetLine() string {
	return fmt.Sprintf("budget: {\"stepsUsed\":%d,\"stepsLeft\":%d}", x.stepsUsed, x.stepsLeft)
}

// result executes one tool call and renders its tool_result block.
func (x *executor) result(tu toolUse) json.RawMessage {
	text, err := x.execute(tu)
	if err != nil {
		return toolResultBlock(tu.ID, "error: "+err.Error()+"\n\n"+x.budgetLine(), true)
	}
	return toolResultBlock(tu.ID, text+"\n\n"+x.budgetLine(), false)
}

func (x *executor) execute(tu toolUse) (string, error) {
	if x.ended {
		return "", errors.New("the run has already ended; no further tools run")
	}
	switch tu.Name {
	case ToolClusterInfo:
		var in struct{}
		if err := decodeInput(tu.Input, &in); err != nil {
			return "", err
		}
		return clusterInfo(x.engine, x.target), nil
	case ToolRunExperiment:
		return x.runExperiment(tu.Input)
	case ToolInspectHistory:
		return x.inspectHistory(tu.Input)
	case ToolInspectEvents:
		return x.inspectEvents(tu.Input)
	case ToolShrink:
		return x.shrink(tu.Input)
	case ToolReportFinding:
		return x.reportFinding(tu.Input)
	case ToolConclude:
		return x.conclude(tu.Input)
	default:
		return "", fmt.Errorf("unknown tool %q", tu.Name)
	}
}

func (x *executor) runExperiment(input json.RawMessage) (string, error) {
	var in runInput
	if err := decodeInput(input, &in); err != nil {
		return "", err
	}
	if len(in.Spec) == 0 {
		return "", errors.New("spec is required")
	}
	spec, err := parseToolSpec(x.engine, in.Spec, x.target)
	if err != nil {
		return "", err
	}
	r, err := x.reg.run(spec)
	if err != nil {
		return "", err
	}
	x.experiments = append(x.experiments, summarize(r, ""))
	return x.engine.Summarize(r), nil
}

func (x *executor) shrink(input json.RawMessage) (string, error) {
	var in idInput
	if err := decodeInput(input, &in); err != nil {
		return "", err
	}
	if in.ExperimentID == "" {
		return "", errors.New("experimentId is required")
	}
	src, err := x.reg.result(in.ExperimentID)
	if err != nil {
		return "", err
	}
	sr, err := x.reg.shrink(in.ExperimentID)
	if err != nil {
		return "", err
	}
	min := sr.Result
	x.experiments = append(x.experiments, summarize(min, in.ExperimentID))
	spec, _ := json.Marshal(min.Spec)
	return fmt.Sprintf("shrunk %s to experiment %s in %d runs (faults %d->%d, durationMs %d->%d, clients %d->%d, keys %d->%d)\nminimal spec: %s\n%s",
		in.ExperimentID, min.ID, sr.Runs,
		len(src.Spec.Faults), len(min.Spec.Faults),
		src.Spec.DurationMs, min.Spec.DurationMs,
		src.Spec.Clients, min.Spec.Clients,
		src.Spec.Keys, min.Spec.Keys,
		spec, x.engine.Summarize(min)), nil
}

func (x *executor) reportFinding(input json.RawMessage) (string, error) {
	var in findingInput
	if err := decodeInput(input, &in); err != nil {
		return "", err
	}
	if err := validateFinding(&in); err != nil {
		return "", err
	}
	// Re-run server-side: the verdict is never taken from the client.
	r, err := x.reg.result(in.ExperimentID)
	if err != nil {
		return "", err
	}
	if r.Check.Verdict != checker.VerdictViolation {
		return "", fmt.Errorf("rejected: experiment %s has verdict %q; only checker-confirmed violations can be reported", in.ExperimentID, r.Check.Verdict)
	}
	x.finding = &Finding{
		ExperimentID: r.ID,
		Title:        in.Title,
		Anomaly:      in.Anomaly,
		CausalChain:  in.CausalChain,
		Spec:         r.Spec,
		Check:        r.Check,
	}
	x.ended = true
	return fmt.Sprintf("finding recorded for experiment %s; the run is over", r.ID), nil
}

func validateFinding(in *findingInput) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Anomaly = strings.TrimSpace(in.Anomaly)
	switch {
	case in.ExperimentID == "":
		return errors.New("experimentId is required")
	case in.Title == "":
		return errors.New("title is required")
	case len(in.Title) > maxTitleLen:
		return fmt.Errorf("title is longer than %d characters", maxTitleLen)
	case in.Anomaly == "":
		return errors.New("anomaly is required")
	case len(in.Anomaly) > maxAnomalyLen:
		return fmt.Errorf("anomaly is longer than %d characters", maxAnomalyLen)
	case len(in.CausalChain) == 0:
		return errors.New("causalChain needs at least one step")
	case len(in.CausalChain) > maxChainSteps:
		return fmt.Errorf("causalChain has more than %d steps", maxChainSteps)
	}
	for i, s := range in.CausalChain {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("causalChain[%d] is empty", i)
		}
		if len(s) > maxChainStepLen {
			return fmt.Errorf("causalChain[%d] is longer than %d characters", i, maxChainStepLen)
		}
		in.CausalChain[i] = s
	}
	return nil
}

func (x *executor) conclude(input json.RawMessage) (string, error) {
	var in concludeInput
	if err := decodeInput(input, &in); err != nil {
		return "", err
	}
	in.Summary = strings.TrimSpace(in.Summary)
	if in.Summary == "" {
		return "", errors.New("summary is required")
	}
	if len(in.Summary) > maxSummaryLen {
		return "", fmt.Errorf("summary is longer than %d characters", maxSummaryLen)
	}
	x.conclusion = in.Summary
	x.ended = true
	return "conclusion recorded; the run is over", nil
}
