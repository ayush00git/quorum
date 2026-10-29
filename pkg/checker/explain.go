package checker

import (
	"fmt"
	"slices"
	"sort"

	"github.com/anishathalye/porcupine"

	"github.com/ayush00git/quorum/pkg/workload"
)

// explainViolation turns porcupine's partial linearizations into the
// longest linearizable prefix, the ops that cannot follow it, the ops they
// conflict with, and a plain-English account.
func explainViolation(kh keyHistory, info porcupine.LinearizationInfo) CheckResult {
	prefix := longestPrefix(info)
	in := make([]bool, len(kh.ops))
	state := register{}
	var lastWriter = -1
	for _, i := range prefix {
		in[i] = true
		ok, next := registerModel.Step(state, kh.pops[i].Input, kh.pops[i].Output)
		if ok && next.(register) != state {
			lastWriter = i
		}
		state = next.(register)
	}
	// Ops that could come next in real time: invoked before the earliest
	// return among the ops not yet linearized. None of them fits the state.
	minReturn := int64(-1)
	for i, p := range kh.pops {
		if !in[i] && (minReturn < 0 || p.Return < minReturn) {
			minReturn = p.Return
		}
	}
	var illegal []int
	for i, p := range kh.pops {
		if in[i] || p.Call > minReturn {
			continue
		}
		if ok, _ := registerModel.Step(state, p.Input, p.Output); !ok {
			illegal = append(illegal, i)
		}
	}
	sort.Slice(illegal, func(a, b int) bool { return kh.pops[illegal[a]].Return < kh.pops[illegal[b]].Return })

	res := CheckResult{Verdict: VerdictViolation, Key: kh.key}
	for _, i := range prefix {
		res.LinearizedIDs = append(res.LinearizedIDs, kh.ops[i].ID)
	}
	offending := map[int]bool{}
	for _, i := range illegal {
		res.IllegalIDs = append(res.IllegalIDs, kh.ops[i].ID)
		offending[kh.ops[i].ID] = true
	}
	var conflicts []workload.Op
	if len(illegal) > 0 {
		var text string
		text, conflicts = describe(kh, kh.ops[illegal[0]], state, lastWriter)
		res.Explanation = text
	} else {
		res.Explanation = fmt.Sprintf("the history of key %s cannot be linearized after %d operations", kh.key, len(prefix))
	}
	// Conflicting ops are reported only if they are in the linearized
	// prefix, so "offending and not linearized" always means illegal.
	for _, op := range conflicts {
		for _, i := range prefix {
			if kh.ops[i].ID == op.ID {
				offending[op.ID] = true
			}
		}
	}
	for id := range offending {
		res.OffendingIDs = append(res.OffendingIDs, id)
	}
	sort.Ints(res.IllegalIDs)
	sort.Ints(res.OffendingIDs)
	return res
}

// longestPrefix picks the longest partial linearization, breaking ties by
// the lexicographically smallest sequence so the choice is deterministic.
func longestPrefix(info porcupine.LinearizationInfo) []int {
	var best []int
	for _, part := range info.PartialLinearizations() {
		for _, seq := range part {
			if len(seq) > len(best) || (len(seq) == len(best) && slices.Compare(seq, best) < 0) {
				best = seq
			}
		}
	}
	return best
}
