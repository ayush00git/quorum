package sim

import "testing"

func draw(r *Rand, n int) []uint64 {
	out := make([]uint64, n)
	for i := range out {
		out[i] = r.Uint64()
	}
	return out
}

func TestRandReproducible(t *testing.T) {
	a, b := draw(NewRand(42), 100), draw(NewRand(42), 100)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("draw %d differs: %d vs %d", i, a[i], b[i])
		}
	}
	c := draw(NewRand(43), 100)
	same := 0
	for i := range a {
		if a[i] == c[i] {
			same++
		}
	}
	if same > 0 {
		t.Fatalf("seeds 42 and 43 share %d draws", same)
	}
}

func TestForkIndependentOfParentUsage(t *testing.T) {
	fresh := NewRand(7).Fork("network")
	used := NewRand(7)
	draw(used, 1000)
	usedFork := used.Fork("network")
	a, b := draw(fresh, 50), draw(usedFork, 50)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("fork depends on parent usage at draw %d", i)
		}
	}
}

func TestForkStreamsDiffer(t *testing.T) {
	root := NewRand(7)
	names := []string{"network", "elections", "workload", "elections/n1", "elections/n2"}
	seen := map[uint64]string{}
	for _, n := range names {
		v := root.Fork(n).Uint64()
		if prev, ok := seen[v]; ok {
			t.Fatalf("streams %q and %q start identically", prev, n)
		}
		seen[v] = n
	}
	// Using one stream must not perturb another.
	r1 := NewRand(9)
	w1 := r1.Fork("workload")
	draw(r1.Fork("network"), 500)
	r2 := NewRand(9)
	w2 := r2.Fork("workload")
	if w1.Uint64() != w2.Uint64() {
		t.Fatal("workload stream perturbed by network usage")
	}
}

func TestRangeBounds(t *testing.T) {
	r := NewRand(1)
	counts := map[int64]int{}
	for i := 0; i < 10000; i++ {
		v := r.Range(150, 154)
		if v < 150 || v > 154 {
			t.Fatalf("Range out of bounds: %d", v)
		}
		counts[v]++
	}
	if len(counts) != 5 {
		t.Fatalf("expected all 5 values, got %v", counts)
	}
	for i := 0; i < 1000; i++ {
		if f := r.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 out of range: %g", f)
		}
	}
}
