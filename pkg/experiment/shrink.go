package experiment

import (
	"fmt"
	"time"

	"github.com/ayush00git/quorum/pkg/checker"
)

// Shrink limits. Candidates whose check times out count as not violating.
const (
	MaxShrinkRuns      = 80
	shrinkCheckTimeout = time.Second
	shrinkBudget       = 25 * time.Second
	durationResolution = 25
)

type shrinker struct {
	runs     int
	deadline time.Time
	best     Result
}

func (sh *shrinker) exhausted() bool {
	return sh.runs >= MaxShrinkRuns || time.Now().After(sh.deadline)
}

// try runs a candidate and keeps it as the best if it still violates.
func (sh *shrinker) try(s Spec) bool {
	if sh.exhausted() {
		return false
	}
	sh.runs++
	res, err := run(s, shrinkCheckTimeout, nil)
	if err != nil || res.Check.Verdict != checker.VerdictViolation {
		return false
	}
	sh.best = res
	return true
}

// Shrink minimizes a violating spec: delta debugging (ddmin) over the fault
// list, then a binary search on durationMs, then the fewest clients and keys
// that still violate. Every accepted candidate still violates. It stops
// after MaxShrinkRuns experiments.
func Shrink(s Spec) (ShrinkResult, error) {
	s, err := Normalize(s)
	if err != nil {
		return ShrinkResult{}, err
	}
	first, err := run(s, CheckTimeout, nil)
	if err != nil {
		return ShrinkResult{}, err
	}
	if first.Check.Verdict != checker.VerdictViolation {
		return ShrinkResult{}, fmt.Errorf("spec %s does not violate linearizability (verdict %s), so there is nothing to shrink", first.ID, first.Check.Verdict)
	}
	sh := &shrinker{runs: 1, deadline: time.Now().Add(shrinkBudget), best: first}
	sh.faults()
	sh.duration()
	sh.reduce(func(s *Spec) *int { return &s.Clients })
	sh.reduce(func(s *Spec) *int { return &s.Keys })
	return ShrinkResult{Spec: sh.best.Spec, Result: sh.best, Runs: sh.runs}, nil
}

func withFaults(s Spec, faults []Fault) Spec {
	s.Faults = append([]Fault{}, faults...)
	return s
}

// faults runs ddmin over the fault list.
func (sh *shrinker) faults() {
	cur := sh.best.Spec
	if len(cur.Faults) == 0 || sh.try(withFaults(cur, nil)) {
		return
	}
	faults := cur.Faults
	n := 2
	for len(faults) >= 2 && !sh.exhausted() {
		chunks := split(faults, n)
		reduced := false
		for _, c := range chunks {
			if sh.try(withFaults(cur, c)) {
				faults, n, reduced = c, 2, true
				break
			}
		}
		if !reduced && n > 2 {
			for i := range chunks {
				comp := complement(chunks, i)
				if sh.try(withFaults(cur, comp)) {
					faults, n, reduced = comp, max(n-1, 2), true
					break
				}
			}
		}
		if !reduced {
			if n >= len(faults) {
				break
			}
			n = min(2*n, len(faults))
		}
	}
}

func split(faults []Fault, n int) [][]Fault {
	var out [][]Fault
	for i := 0; i < n; i++ {
		lo, hi := i*len(faults)/n, (i+1)*len(faults)/n
		if lo < hi {
			out = append(out, faults[lo:hi])
		}
	}
	return out
}

func complement(chunks [][]Fault, skip int) []Fault {
	var out []Fault
	for i, c := range chunks {
		if i != skip {
			out = append(out, c...)
		}
	}
	return out
}

// duration binary-searches the shortest run that still violates. Faults
// must stay inside the run, so the lower bound is just after the last one.
func (sh *shrinker) duration() {
	cur := sh.best.Spec
	lo := int64(MinDurationMs)
	for _, f := range cur.Faults {
		lo = max(lo, f.AtMs+1)
	}
	hi := cur.DurationMs
	for hi-lo > durationResolution && !sh.exhausted() {
		mid := lo + (hi-lo)/2
		cand := sh.best.Spec
		cand.DurationMs = mid
		if sh.try(cand) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
}

// reduce tries the smallest values of a count field first and keeps the
// first that still violates.
func (sh *shrinker) reduce(field func(*Spec) *int) {
	cur := sh.best.Spec
	for v := 1; v < *field(&cur) && !sh.exhausted(); v++ {
		cand := sh.best.Spec
		*field(&cand) = v
		if sh.try(cand) {
			return
		}
	}
}
