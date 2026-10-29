package checker

import (
	"strings"
	"testing"
	"time"

	"github.com/ayush00git/quorum/pkg/workload"
)

func ip(v int) *int     { return &v }
func tp(v int64) *int64 { return &v }

func w(id, p int, key string, v int, inv, done int64) workload.Op {
	return workload.Op{ID: id, Process: p, Kind: workload.KindWrite, Key: key, Value: ip(v), Status: workload.StatusOK, InvokeAt: inv, CompleteAt: tp(done)}
}

func r(id, p int, key string, v *int, inv, done int64) workload.Op {
	return workload.Op{ID: id, Process: p, Kind: workload.KindRead, Key: key, Result: v, Status: workload.StatusOK, InvokeAt: inv, CompleteAt: tp(done)}
}

func info(op workload.Op) workload.Op {
	op.Status, op.CompleteAt, op.Result = workload.StatusInfo, nil, nil
	return op
}

func fail(op workload.Op) workload.Op {
	op.Status = workload.StatusFail
	return op
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name    string
		history []workload.Op
		verdict string
		illegal []int
		explain string
	}{
		{"empty", nil, VerdictOK, nil, ""},
		{"sequential ok", []workload.Op{
			r(0, 0, "x", nil, 0, 5), w(1, 0, "x", 1, 10, 20), r(2, 1, "x", ip(1), 30, 40), w(3, 1, "x", 2, 50, 60), r(4, 0, "x", ip(2), 70, 80),
		}, VerdictOK, nil, ""},
		{"concurrent ok", []workload.Op{
			w(0, 0, "x", 1, 0, 100), r(1, 1, "x", nil, 10, 20), r(2, 2, "x", ip(1), 30, 40), r(3, 1, "x", ip(1), 50, 60),
		}, VerdictOK, nil, ""},
		{"stale read", []workload.Op{
			w(0, 0, "x", 4, 0, 10), w(1, 1, "x", 7, 600, 640), r(2, 3, "x", ip(4), 805, 812),
		}, VerdictViolation, []int{2}, "read(x) by p3 returned 4 at 812 ms, but write(x, 7) by p1 completed at 640 ms"},
		{"lost write", []workload.Op{
			w(0, 0, "x", 1, 0, 10), r(1, 1, "x", nil, 20, 30),
		}, VerdictViolation, []int{1}, "returned nothing (unset) at 30 ms, but write(x, 1) by p0 had already completed at 10 ms"},
		{"read of a future write", []workload.Op{
			r(0, 0, "x", ip(5), 0, 10), w(1, 1, "x", 5, 50, 60),
		}, VerdictViolation, []int{0}, "was not invoked until 50 ms"},
		{"indeterminate write landing late is ok", []workload.Op{
			info(w(0, 0, "x", 1, 0, 0)), r(1, 1, "x", nil, 100, 110), w(2, 2, "x", 2, 200, 210), r(3, 1, "x", ip(2), 300, 310), r(4, 1, "x", ip(1), 400, 410),
		}, VerdictOK, nil, ""},
		{"fail ops are ignored", []workload.Op{
			fail(w(0, 0, "x", 1, 0, 10)), r(1, 1, "x", nil, 20, 30), fail(r(2, 1, "x", ip(9), 40, 50)),
		}, VerdictOK, nil, ""},
		{"keys are independent", []workload.Op{
			w(0, 0, "x", 1, 0, 10), r(1, 1, "y", nil, 20, 30), r(2, 1, "x", ip(1), 40, 50),
		}, VerdictOK, nil, ""},
		{"violation on second key", []workload.Op{
			w(0, 0, "x", 1, 0, 10), r(1, 1, "x", ip(1), 20, 30), w(2, 0, "y", 1, 0, 10), w(3, 0, "y", 2, 20, 30), r(4, 1, "y", ip(1), 40, 50),
		}, VerdictViolation, []int{4}, "write(y, 2) by p0 completed at 30 ms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.history, 5*time.Second)
			if got.Verdict != tc.verdict {
				t.Fatalf("verdict %q, want %q (%s)", got.Verdict, tc.verdict, got.Explanation)
			}
			if tc.verdict != VerdictViolation {
				return
			}
			if !equalInts(got.IllegalIDs, tc.illegal) {
				t.Errorf("illegal ids %v, want %v", got.IllegalIDs, tc.illegal)
			}
			for _, id := range tc.illegal {
				if !contains(got.OffendingIDs, id) {
					t.Errorf("offending ids %v miss %d", got.OffendingIDs, id)
				}
			}
			if !strings.Contains(got.Explanation, tc.explain) {
				t.Errorf("explanation %q lacks %q", got.Explanation, tc.explain)
			}
		})
	}
}

func TestStaleReadMarksConflictingWrites(t *testing.T) {
	got := Check([]workload.Op{w(0, 0, "x", 4, 0, 10), w(1, 1, "x", 7, 600, 640), r(2, 3, "x", ip(4), 805, 812)}, time.Second)
	if !equalInts(got.OffendingIDs, []int{0, 1, 2}) {
		t.Fatalf("offending %v", got.OffendingIDs)
	}
	if got.Key != "x" || len(got.LinearizedIDs) == 0 {
		t.Fatalf("key %q linearized %v", got.Key, got.LinearizedIDs)
	}
}

// Many concurrent indeterminate writes followed by a read of a value nobody
// wrote forces porcupine through every subset of the writes. The checker
// must answer unknown within its timeout instead of hanging.
func TestTimeoutIsUnknown(t *testing.T) {
	var h []workload.Op
	for i := 0; i < 60; i++ {
		h = append(h, info(w(i, i, "x", i+1, int64(i), 0)))
	}
	for i := 0; i < 60; i++ {
		h = append(h, r(60+i, 60+i, "x", ip(60-i), int64(1000+i*10), int64(1005+i*10)))
	}
	h = append(h, r(120, 120, "x", ip(9999), 5000, 5001))
	start := time.Now()
	got := Check(h, 100*time.Millisecond)
	if got.Verdict != VerdictUnknown {
		t.Fatalf("verdict %q, want unknown", got.Verdict)
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("check took %v despite a 100ms timeout", el)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
