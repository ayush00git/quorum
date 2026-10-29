package sim

import "math/bits"

// Rand is a xoshiro256** generator seeded through SplitMix64. It is owned by
// the simulation and never shared with global state.
type Rand struct {
	seed uint64
	s    [4]uint64
}

func splitmix64(x *uint64) uint64 {
	*x += 0x9e3779b97f4a7c15
	z := *x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// NewRand returns a generator for the given seed.
func NewRand(seed uint64) *Rand {
	r := &Rand{seed: seed}
	x := seed
	for i := range r.s {
		r.s[i] = splitmix64(&x)
	}
	return r
}

// Fork derives an independent named stream. It depends only on this
// generator's seed and the name, never on how much of this stream was used,
// so forks are reproducible regardless of call order.
func (r *Rand) Fork(name string) *Rand {
	h := uint64(14695981039346656037)
	for i := 0; i < len(name); i++ {
		h ^= uint64(name[i])
		h *= 1099511628211
	}
	x := r.seed ^ bits.RotateLeft64(h, 17)
	return NewRand(splitmix64(&x))
}

// Uint64 returns the next 64 random bits.
func (r *Rand) Uint64() uint64 {
	s := &r.s
	result := bits.RotateLeft64(s[1]*5, 7) * 9
	t := s[1] << 17
	s[2] ^= s[0]
	s[3] ^= s[1]
	s[1] ^= s[2]
	s[0] ^= s[3]
	s[2] ^= t
	s[3] = bits.RotateLeft64(s[3], 45)
	return result
}

// Uint64n returns a uniform value in [0, n). It panics if n is zero.
func (r *Rand) Uint64n(n uint64) uint64 {
	if n == 0 {
		panic("sim: Uint64n with n == 0")
	}
	hi, lo := bits.Mul64(r.Uint64(), n)
	if lo < n {
		threshold := -n % n
		for lo < threshold {
			hi, lo = bits.Mul64(r.Uint64(), n)
		}
	}
	return hi
}

// Intn returns a uniform value in [0, n).
func (r *Rand) Intn(n int) int { return int(r.Uint64n(uint64(n))) }

// Range returns a uniform value in the inclusive range [lo, hi].
func (r *Rand) Range(lo, hi int64) int64 {
	if hi <= lo {
		return lo
	}
	return lo + int64(r.Uint64n(uint64(hi-lo)+1))
}

// Float64 returns a uniform value in [0, 1).
func (r *Rand) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }
