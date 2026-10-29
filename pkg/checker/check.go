package checker

import (
	"sort"
	"strconv"
	"time"

	"github.com/anishathalye/porcupine"

	"github.com/ayush00git/quorum/pkg/workload"
)

const (
	kindWrite = workload.KindWrite
	kindCas   = workload.KindCas
)

func itoa(v int) string { return strconv.Itoa(v) }

// keyHistory is the checkable part of one key's history. ops[i] is the
// workload op behind porcupine operation i.
type keyHistory struct {
	key  string
	ops  []workload.Op
	pops []porcupine.Operation
}

// Check verifies a history for linearizability, one key at a time in sorted
// key order. fail ops are excluded; info writes may take effect any time
// after invocation, or never; info reads constrain nothing and are dropped.
// If checking exceeds timeout the verdict is unknown.
func Check(history []workload.Op, timeout time.Duration) CheckResult {
	deadline := time.Now().Add(timeout)
	unknown := false
	for _, kh := range splitByKey(history) {
		left := time.Until(deadline)
		if left <= 0 {
			unknown = true
			break
		}
		res, info := porcupine.CheckOperationsVerbose(registerModel, kh.pops, left)
		switch res {
		case porcupine.Illegal:
			return explainViolation(kh, info)
		case porcupine.Unknown:
			unknown = true
		}
	}
	if unknown {
		return CheckResult{Verdict: VerdictUnknown, Explanation: "the checker timed out before deciding; no violation was confirmed"}
	}
	return CheckResult{Verdict: VerdictOK}
}

func splitByKey(history []workload.Op) []keyHistory {
	var end int64
	for _, op := range history {
		end = max(end, op.InvokeAt)
		if op.CompleteAt != nil {
			end = max(end, *op.CompleteAt)
		}
	}
	never := end + 1_000_000_000

	byKey := map[string]*keyHistory{}
	for _, op := range history {
		if op.Status == workload.StatusFail || (op.Status == workload.StatusInfo && op.Kind == workload.KindRead) {
			continue
		}
		kh := byKey[op.Key]
		if kh == nil {
			kh = &keyHistory{key: op.Key}
			byKey[op.Key] = kh
		}
		ret := never
		if op.Status == workload.StatusOK && op.CompleteAt != nil {
			ret = *op.CompleteAt
		}
		in := input{kind: op.Kind, info: op.Status == workload.StatusInfo}
		if op.Value != nil {
			in.value = *op.Value
		}
		if op.Expected != nil {
			in.expected = *op.Expected
		}
		out := output{result: op.Result}
		kh.ops = append(kh.ops, op)
		kh.pops = append(kh.pops, porcupine.Operation{ClientId: op.Process, Input: in, Call: op.InvokeAt, Output: out, Return: ret})
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]keyHistory, 0, len(keys))
	for _, k := range keys {
		out = append(out, *byKey[k])
	}
	return out
}
