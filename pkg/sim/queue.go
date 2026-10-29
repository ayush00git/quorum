package sim

import "container/heap"

type item struct {
	at  int64
	seq uint64
	fn  func()
}

// queue is a min-heap ordered by (at, seq), so events at the same virtual
// time run in the order they were scheduled.
type queue []item

func (q queue) Len() int { return len(q) }
func (q queue) Less(i, j int) bool {
	if q[i].at != q[j].at {
		return q[i].at < q[j].at
	}
	return q[i].seq < q[j].seq
}
func (q queue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)   { *q = append(*q, x.(item)) }
func (q *queue) Pop() any {
	old := *q
	n := len(old)
	it := old[n-1]
	old[n-1] = item{}
	*q = old[:n-1]
	return it
}

var _ heap.Interface = (*queue)(nil)
