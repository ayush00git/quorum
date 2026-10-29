package experiment

import (
	"fmt"
	"strings"

	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/sim"
	"github.com/ayush00git/quorum/pkg/workload"
)

const (
	maxSummaryEvents = 24
	maxSummaryOps    = 8
)

// Summarize renders a compact, token-efficient text summary for the agent:
// the setup, verdict, op counts, leader timeline, notable events and, on a
// violation, the explanation and offending ops.
func Summarize(r Result) string {
	s := r.Spec
	var b strings.Builder
	fmt.Fprintf(&b, "experiment %s: %s, seed %d, %d ms, %d clients, %d keys, reads %.0f%% from %s, op timeout %d ms\n",
		r.ID, s.Target, s.Seed, s.DurationMs, s.Clients, s.Keys, s.ReadRatio*100, s.ReadFrom, s.OpTimeoutMs)
	n := s.Network
	fmt.Fprintf(&b, "network: latency %d-%d ms, drop %g, duplicate %g\n", n.MinLatencyMs, n.MaxLatencyMs, n.DropRate, n.DuplicateRate)
	if len(s.Faults) == 0 {
		b.WriteString("faults: none\n")
	} else {
		parts := make([]string, len(s.Faults))
		for i, f := range s.Faults {
			parts[i] = DescribeFault(f)
		}
		fmt.Fprintf(&b, "faults (%d): %s\n", len(s.Faults), strings.Join(parts, "; "))
	}
	fmt.Fprintf(&b, "verdict: %s\n", verdictLine(r.Check))
	st := r.Stats
	fmt.Fprintf(&b, "ops: %d (ok %d, fail %d, info %d; reads %d, writes %d); messages %d sent, %d dropped\n",
		st.Ops, st.OK, st.Fail, st.Info, st.Reads, st.Writes, st.MessagesSent, st.MessagesDropped)
	b.WriteString("leaders: ")
	if len(st.Leaders) == 0 {
		b.WriteString("none elected")
	}
	for i, l := range st.Leaders {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "t%d %s %d-%d", l.Term, l.Node, l.FromMs, l.ToMs)
	}
	b.WriteString("\n")
	writeEvents(&b, r.Events)
	if r.Check.Verdict == checker.VerdictViolation {
		fmt.Fprintf(&b, "explanation: %s\n", r.Check.Explanation)
		writeOffending(&b, r)
	}
	return b.String()
}

func verdictLine(c checker.CheckResult) string {
	switch c.Verdict {
	case checker.VerdictViolation:
		return "VIOLATION (not linearizable) on key " + c.Key
	case checker.VerdictUnknown:
		return "unknown (checker timed out; no violation confirmed)"
	}
	return "ok (linearizable)"
}

// DescribeFault renders one fault as a short line, e.g. "1000 isolate n2".
func DescribeFault(f Fault) string {
	a := f.Args
	switch f.Action {
	case ActionPartition:
		groups := make([]string, len(a.Groups))
		for i, g := range a.Groups {
			groups[i] = strings.Join(g, ",")
		}
		return fmt.Sprintf("%d partition %s", f.AtMs, strings.Join(groups, " | "))
	case ActionHeal:
		return fmt.Sprintf("%d heal", f.AtMs)
	case ActionPause:
		return fmt.Sprintf("%d pause %s %dms", f.AtMs, a.Node, a.DurationMs)
	case ActionSkewClock:
		return fmt.Sprintf("%d skewClock %s %+dppm", f.AtMs, a.Node, a.DriftPpm)
	case ActionSetNetwork:
		var parts []string
		if a.MinLatencyMs != nil {
			parts = append(parts, fmt.Sprintf("minLatency %d", *a.MinLatencyMs))
		}
		if a.MaxLatencyMs != nil {
			parts = append(parts, fmt.Sprintf("maxLatency %d", *a.MaxLatencyMs))
		}
		if a.DropRate != nil {
			parts = append(parts, fmt.Sprintf("drop %g", *a.DropRate))
		}
		if a.DuplicateRate != nil {
			parts = append(parts, fmt.Sprintf("dup %g", *a.DuplicateRate))
		}
		return fmt.Sprintf("%d setNetwork %s", f.AtMs, strings.Join(parts, " "))
	}
	return fmt.Sprintf("%d %s %s", f.AtMs, f.Action, a.Node)
}

// writeEvents lists notable events. Votes and commit advances are only
// counted; elections are listed since they show instability.
func writeEvents(b *strings.Builder, events []sim.Event) {
	var lines []string
	elections, votes := 0, 0
	for _, e := range events {
		switch e.Kind {
		case sim.EventCommitAdvanced:
			continue
		case sim.EventVoteGranted:
			votes++
			continue
		case sim.EventElectionStarted:
			elections++
		}
		lines = append(lines, FormatEvent(e))
	}
	fmt.Fprintf(b, "events: %d elections started, %d votes granted\n", elections, votes)
	shown := lines
	if len(lines) > maxSummaryEvents {
		shown = lines[:maxSummaryEvents]
	}
	for _, l := range shown {
		fmt.Fprintf(b, "  %s\n", l)
	}
	if len(lines) > len(shown) {
		fmt.Fprintf(b, "  ... %d more (use inspect_events)\n", len(lines)-len(shown))
	}
}

// FormatEvent renders one event as a short line.
func FormatEvent(e sim.Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d", e.AtMs)
	if e.Node != "" {
		fmt.Fprintf(&b, " %s", e.Node)
	}
	fmt.Fprintf(&b, " %s", e.Kind)
	if e.Term > 0 {
		fmt.Fprintf(&b, " t%d", e.Term)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, " (%s)", e.Detail)
	}
	return b.String()
}

func writeOffending(b *strings.Builder, r Result) {
	illegal := map[int]bool{}
	for _, id := range r.Check.IllegalIDs {
		illegal[id] = true
	}
	fmt.Fprintf(b, "offending ops (* cannot be linearized): %d\n", len(r.Check.OffendingIDs))
	for i, id := range r.Check.OffendingIDs {
		if i == maxSummaryOps {
			fmt.Fprintf(b, "  ... %d more\n", len(r.Check.OffendingIDs)-i)
			break
		}
		if id < 0 || id >= len(r.History) {
			continue
		}
		mark := " "
		if illegal[id] {
			mark = "*"
		}
		fmt.Fprintf(b, "  %s %s\n", mark, FormatOp(r.History[id]))
	}
}

// FormatOp renders one op as a short line, e.g.
// "#12 p3 r(x)->4 [805..812] n3 ok".
func FormatOp(op workload.Op) string {
	var what string
	switch op.Kind {
	case workload.KindWrite:
		what = fmt.Sprintf("w(%s,%d)", op.Key, intOr(op.Value))
	case workload.KindCas:
		what = fmt.Sprintf("cas(%s,%d->%d)", op.Key, intOr(op.Expected), intOr(op.Value))
	default:
		what = fmt.Sprintf("r(%s)", op.Key)
		if op.Status == workload.StatusOK {
			if op.Result == nil {
				what += "->nil"
			} else {
				what += fmt.Sprintf("->%d", *op.Result)
			}
		}
	}
	end := "?"
	if op.CompleteAt != nil {
		end = fmt.Sprintf("%d", *op.CompleteAt)
	}
	return fmt.Sprintf("#%d p%d %s [%d..%s] %s %s", op.ID, op.Process, what, op.InvokeAt, end, op.Node, op.Status)
}

func intOr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
