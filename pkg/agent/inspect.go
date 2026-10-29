package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ayush00git/quorum/pkg/experiment"
	"github.com/ayush00git/quorum/pkg/sim"
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
			lines = append(lines, opLine(op, slices.Contains(r.Check.OffendingIDs, op.ID)))
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
			lines = append(lines, eventLine(ev))
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

func intStr(v *int) string {
	if v == nil {
		return "nil"
	}
	return strconv.Itoa(*v)
}

// opLabel renders an op like w(x,3), r(x)->2 or cas(x,1->2).
func opLabel(op workload.Op) string {
	switch op.Kind {
	case workload.KindWrite:
		return fmt.Sprintf("w(%s,%s)", op.Key, intStr(op.Value))
	case workload.KindRead:
		if op.Status == workload.StatusOK {
			return fmt.Sprintf("r(%s)->%s", op.Key, intStr(op.Result))
		}
		return fmt.Sprintf("r(%s)", op.Key)
	case workload.KindCas:
		return fmt.Sprintf("cas(%s,%s->%s)", op.Key, intStr(op.Expected), intStr(op.Value))
	default:
		return fmt.Sprintf("%s(%s)", op.Kind, op.Key)
	}
}

func opLine(op workload.Op, offending bool) string {
	mark := " "
	if offending {
		mark = "!"
	}
	done := "?"
	if op.CompleteAt != nil {
		done = strconv.FormatInt(*op.CompleteAt, 10)
	}
	return fmt.Sprintf("%s#%d p%d %s %s [%d-%s] %s", mark, op.ID, op.Process, opLabel(op), op.Status, op.InvokeAt, done, op.Node)
}

func eventLine(ev sim.Event) string {
	node := ev.Node
	if node == "" {
		node = "-"
	}
	line := fmt.Sprintf("%dms %s %s", ev.AtMs, node, ev.Kind)
	if ev.Term != 0 {
		line += fmt.Sprintf(" term=%d", ev.Term)
	}
	if ev.Detail != "" {
		line += " " + ev.Detail
	}
	return line
}

func nodeIDs(engine Engine, target string) []string {
	n := engine.DefaultSpec(target).Nodes
	if n <= 0 {
		n = experiment.NodeCount
	}
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i+1)
	}
	return ids
}
