// Package checker verifies histories for linearizability using porcupine.
package checker

// CheckResult is the checker verdict for one history.
type CheckResult struct {
	Verdict       string `json:"verdict"` // ok | violation | unknown
	Key           string `json:"key,omitempty"`
	LinearizedIDs []int  `json:"linearizedIds,omitempty"`
	OffendingIDs  []int  `json:"offendingIds,omitempty"` // IllegalIDs plus the ops they conflict with
	IllegalIDs    []int  `json:"illegalIds,omitempty"`   // ops that cannot follow the longest linearizable prefix
	Explanation   string `json:"explanation,omitempty"`
}

// Verdicts.
const (
	VerdictOK        = "ok"
	VerdictViolation = "violation"
	VerdictUnknown   = "unknown"
)
