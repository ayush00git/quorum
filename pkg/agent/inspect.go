package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/experiment"
	"github.com/ayush00git/quorum/pkg/workload"
)

type windowInput struct {
	ExperimentID string `json:"experimentId"`
	FromMs       *int64 `json:"fromMs"`
	ToMs         *int64 `json:"toMs"`
	Limit        *int   `json:"limit"`
}

type historyInput struct {
	windowInput
	Key *string `json:"key"`
}

type eventsInput struct {
	windowInput
	Node *string `json:"node"`
}

// bounds validates the window and returns [from, to] and the line limit.
func (w windowInput) bounds() (from, to int64, limit int, err error) {
	if w.ExperimentID == "" {
		return 0, 0, 0, errors.New("experimentId is required")
	}
	from, to, limit = 0, int64(^uint64(0)>>1), maxInspectLines
	if w.FromMs != nil {
		from = *w.FromMs
	}
	if w.ToMs != nil {
		to = *w.ToMs
	}
	if from < 0 || to < from {
		return 0, 0, 0, fmt.Errorf("invalid window: fromMs %d, toMs %d", from, to)
	}
	if w.Limit != nil {
		if *w.Limit < 1 || *w.Limit > maxInspectLines {
			return 0, 0, 0, fmt.Errorf("limit must be between 1 and %d", maxInspectLines)
		}
		limit = *w.Limit
	}
	return from, to, limit, nil
}

func (x *executor) inspectHistory(input json.RawMessage) (string, error) {
	var in historyInput
	if err := decodeInput(input, &in); err != nil {
		return "", err
	}
	from, to, limit, err := in.bounds()
	if err != nil {
		return "", err
	}
	r, err := x.reg.result(in.ExperimentID)
	if err != nil {
		return "", err
	}
	var lines []string
	matched := 0
	for _, op := range r.History {
		if in.Key != nil && op.Key != *in.Key {
			continue
		}
		if op.InvokeAt > to || (op.CompleteAt != nil && *op.CompleteAt < from) {
			continue
		}
		matched++
		if len(lines) < limit {
			lines = append(lines, opLine(op, r.Check))
		}
	}
	return renderLines(fmt.Sprintf("experiment %s ops", r.ID), lines, matched, "narrow with key, fromMs or toMs"), nil
}

func (x *executor) inspectEvents(input json.RawMessage) (string, error) {
	var in eventsInput
	if err := decodeInput(input, &in); err != nil {
		return "", err
	}
	from, to, limit, err := in.bounds()
	if err != nil {
		return "", err
	}
	if in.Node != nil && !slices.Contains(nodeIDs(x.engine, x.target), *in.Node) {
		return "", fmt.Errorf("unknown node %q", *in.Node)
	}
	r, err := x.reg.result(in.ExperimentID)
	if err != nil {
		return "", err
	}
	var lines []string
	matched := 0
	for _, ev := range r.Events {
		if in.Node != nil && ev.Node != *in.Node {
			continue
		}
		if ev.AtMs < from || ev.AtMs > to {
			continue
		}
		matched++
		if len(lines) < limit {
			lines = append(lines, experiment.FormatEvent(ev))
		}
	}
	return renderLines(fmt.Sprintf("experiment %s events", r.ID), lines, matched, "narrow with node, fromMs or toMs"), nil
}

func renderLines(title string, lines []string, matched int, hint string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: showing %d of %d", title, len(lines), matched)
	if len(lines) < matched {
		fmt.Fprintf(&b, " (%s)", hint)
	}
	for _, l := range lines {
		b.WriteString("\n")
		b.WriteString(l)
	}
	return b.String()
}

// opLine marks ops the checker flagged: '*' cannot be linearized, '!'
// conflicts with one that cannot.
func opLine(op workload.Op, c checker.CheckResult) string {
	mark := " "
	switch {
	case slices.Contains(c.IllegalIDs, op.ID):
		mark = "*"
	case slices.Contains(c.OffendingIDs, op.ID):
		mark = "!"
	}
	return mark + " " + experiment.FormatOp(op)
}

func nodeIDs(engine Engine, target string) []string {
	n := engine.DefaultSpec(target).Nodes
	if n <= 0 {
		n = experiment.NodeCount
	}
	return experiment.NodeIDs(n)
}
