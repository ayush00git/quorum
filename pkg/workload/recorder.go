package workload

import "github.com/ayush00git/quorum/pkg/sim"

// Config describes the workload of one run.
type Config struct {
	Nodes       []string
	Keys        []string
	ReadRatio   float64
	ReadFrom    string // leader | any
	OpTimeoutMs int64
	MaxAttempts int   // attempts per op before giving up on NotLeader replies
	ThinkMaxMs  int64 // clients wait uniform [0, ThinkMaxMs] between ops
	RetryMs     int64 // backoff before retrying without a leader hint
}

// Read sources.
const (
	ReadFromLeader = "leader"
	ReadFromAny    = "any"
)

// Recorder owns the shared history of a run: op ids, process ids and the
// per-key write counters that make every written value unique.
type Recorder struct {
	cfg         Config
	ops         []Op
	nextProcess int
	nextValue   map[string]int
	stopAt      int64
}

// NewRecorder returns a recorder that stops invoking new ops at stopAt.
func NewRecorder(cfg Config, stopAt int64) *Recorder {
	return &Recorder{cfg: cfg, nextValue: map[string]int{}, stopAt: stopAt}
}

// History returns the recorded ops. Ops still in flight are left with
// status info and no completion time.
func (r *Recorder) History() []Op { return r.ops }

// Factory builds the client for one slot. Clients are not crashable, so the
// factory runs once per slot.
func (r *Recorder) Factory() sim.Factory {
	return func(env *sim.Env) sim.Handler {
		c := &client{env: env, rec: r, rand: env.Rand()}
		c.retire()
		return c
	}
}

func (r *Recorder) newProcess() int {
	p := r.nextProcess
	r.nextProcess++
	return p
}

func (r *Recorder) invoke(op Op) int {
	op.ID = len(r.ops)
	op.Status = StatusInfo
	r.ops = append(r.ops, op)
	return op.ID
}

func (r *Recorder) uniqueValue(key string) int {
	r.nextValue[key]++
	return r.nextValue[key]
}

func (r *Recorder) complete(id int, status, node string, at int64, result *int) {
	op := &r.ops[id]
	op.Status = status
	op.Node = node
	if status == StatusInfo {
		return
	}
	op.CompleteAt = &at
	op.Result = result
}
