package raft

import "fmt"

// Monitor checks Raft safety across a whole cluster: at most one leader per
// term, and every node applies the same entry at each index (so no committed
// entry is ever lost or replaced). It observes only; it never changes
// behavior.
type Monitor struct {
	leaders    map[int]string
	committed  []Entry // committed[i] is the entry at log index i+1
	Violations []string
}

// maxViolations caps how many violations are recorded; one is enough to
// fail, the rest are usually the same divergence repeated.
const maxViolations = 10

func (m *Monitor) report(format string, args ...any) {
	if len(m.Violations) < maxViolations {
		m.Violations = append(m.Violations, fmt.Sprintf(format, args...))
	}
}

// NewMonitor returns an empty monitor.
func NewMonitor() *Monitor { return &Monitor{leaders: map[int]string{}} }

func (m *Monitor) observeLeader(node string, term int) {
	if prev, ok := m.leaders[term]; ok && prev != node {
		m.report("term %d has two leaders: %s and %s", term, prev, node)
		return
	}
	m.leaders[term] = node
}

func (m *Monitor) observeApply(node string, index int, e Entry) {
	switch {
	case index == len(m.committed)+1:
		m.committed = append(m.committed, e)
	case index <= len(m.committed):
		if m.committed[index-1] != e {
			m.report("%s applied %+v at index %d, but %+v was committed there", node, e, index, m.committed[index-1])
		}
	default:
		m.report("%s applied index %d past the committed prefix %d", node, index, len(m.committed))
	}
}

// Committed is the number of entries known to be committed.
func (m *Monitor) Committed() int { return len(m.committed) }
