// Package workload drives simulated clients and records the operation history.
package workload

// Op is one client operation in the recorded history.
type Op struct {
	ID         int    `json:"id"`
	Process    int    `json:"process"`
	Kind       string `json:"kind"` // read | write | cas
	Key        string `json:"key"`
	Value      *int   `json:"value,omitempty"`    // write value or cas target
	Expected   *int   `json:"expected,omitempty"` // cas only
	Result     *int   `json:"result,omitempty"`   // read result, nil means unset key
	Status     string `json:"status"`             // ok | fail | info
	InvokeAt   int64  `json:"invokeAt"`
	CompleteAt *int64 `json:"completeAt"` // nil for info
	Node       string `json:"node"`
}

// Op kinds and statuses.
const (
	KindRead  = "read"
	KindWrite = "write"
	KindCas   = "cas"

	StatusOK   = "ok"
	StatusFail = "fail"
	StatusInfo = "info"
)
