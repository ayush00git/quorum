package checker

import (
	"fmt"

	"github.com/ayush00git/quorum/pkg/workload"
)

func opLabel(op workload.Op) string {
	switch op.Kind {
	case workload.KindWrite:
		return fmt.Sprintf("write(%s, %d) by p%d", op.Key, deref(op.Value), op.Process)
	case workload.KindCas:
		return fmt.Sprintf("cas(%s, %d->%d) by p%d", op.Key, deref(op.Expected), deref(op.Value), op.Process)
	}
	return fmt.Sprintf("read(%s) by p%d", op.Key, op.Process)
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func valueText(v *int) string {
	if v == nil {
		return "nothing (unset)"
	}
	return itoa(*v)
}

// completion describes when a write finished, or that its outcome is unknown.
func completion(op workload.Op) string {
	if op.Status == workload.StatusOK && op.CompleteAt != nil {
		return fmt.Sprintf("completed at %d ms", *op.CompleteAt)
	}
	return fmt.Sprintf("was invoked at %d ms with an unknown outcome", op.InvokeAt)
}

func isWrite(op workload.Op) bool {
	return op.Kind == workload.KindWrite || op.Kind == workload.KindCas
}

// describe explains why bad cannot be linearized and returns the ops it
// conflicts with. state is the register after the longest linearizable
// prefix and lastWriter the porcupine index of the op that set it.
func describe(kh keyHistory, bad workload.Op, state register, lastWriter int) (string, []workload.Op) {
	key := kh.key
	if bad.Kind == workload.KindRead {
		if text, conflicts, ok := describeStaleRead(kh, bad); ok {
			return text, conflicts
		}
	}
	var conflicts []workload.Op
	holder := fmt.Sprintf("%s held %s", key, state)
	if lastWriter >= 0 {
		w := kh.ops[lastWriter]
		conflicts = append(conflicts, w)
		holder += fmt.Sprintf(" (set by %s, which %s)", opLabel(w), completion(w))
	}
	what := fmt.Sprintf("%s returned %s at %d ms", opLabel(bad), valueText(bad.Result), deref64(bad.CompleteAt))
	if bad.Kind == workload.KindCas {
		what = fmt.Sprintf("%s succeeded at %d ms", opLabel(bad), deref64(bad.CompleteAt))
	}
	return fmt.Sprintf("%s (invoked at %d ms), but in every ordering consistent with real time %s at that point.",
		what, bad.InvokeAt, holder), conflicts
}

func deref64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// describeStaleRead finds the classic patterns: a read of a value that was
// never written or not yet written, or a read of a value that a completed
// later write had already replaced.
func describeStaleRead(kh keyHistory, r workload.Op) (string, []workload.Op, bool) {
	var source *workload.Op
	if r.Result != nil {
		for i := range kh.ops {
			if w := kh.ops[i]; isWrite(w) && w.Value != nil && *w.Value == *r.Result {
				source = &kh.ops[i]
				break
			}
		}
		if source == nil {
			return fmt.Sprintf("%s returned %d at %d ms, but no write of %d to %s could have taken effect.",
				opLabel(r), *r.Result, deref64(r.CompleteAt), *r.Result, r.Key), nil, true
		}
		if source.InvokeAt > deref64(r.CompleteAt) {
			return fmt.Sprintf("%s returned %d at %d ms, but %s was not invoked until %d ms.",
				opLabel(r), *r.Result, deref64(r.CompleteAt), opLabel(*source), source.InvokeAt), []workload.Op{*source}, true
		}
	}
	// The latest write that completed before the read began and strictly
	// after the source write completed must be visible to the read.
	var newer *workload.Op
	for i := range kh.ops {
		w := kh.ops[i]
		if !isWrite(w) || w.Status != workload.StatusOK || w.CompleteAt == nil || *w.CompleteAt >= r.InvokeAt {
			continue
		}
		if source != nil && (source.CompleteAt == nil || source.Status != workload.StatusOK || w.InvokeAt <= *source.CompleteAt) {
			continue
		}
		if newer == nil || *w.CompleteAt > *newer.CompleteAt {
			newer = &kh.ops[i]
		}
	}
	if newer == nil {
		return "", nil, false
	}
	if source == nil {
		return fmt.Sprintf("%s returned nothing (unset) at %d ms, but %s had already completed at %d ms.",
			opLabel(r), deref64(r.CompleteAt), opLabel(*newer), *newer.CompleteAt), []workload.Op{*newer}, true
	}
	return fmt.Sprintf("%s returned %d at %d ms, but %s completed at %d ms, after %s %s, and no later write set %s back to %d.",
		opLabel(r), *r.Result, deref64(r.CompleteAt), opLabel(*newer), *newer.CompleteAt,
		opLabel(*source), completion(*source), r.Key, *r.Result), []workload.Op{*newer, *source}, true
}
