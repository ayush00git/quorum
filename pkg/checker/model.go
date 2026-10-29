package checker

import "github.com/anishathalye/porcupine"

// register is the state of one key: unset, or holding an integer.
type register struct {
	set bool
	v   int
}

type input struct {
	kind     string
	value    int
	expected int
	info     bool // outcome unknown: a cas may have applied or not
}

type output struct {
	result *int // read result
}

func (r register) String() string {
	if !r.set {
		return "unset"
	}
	return itoa(r.v)
}

// registerModel is a single-key register. Histories are split per key
// before checking, so the model never needs to look at keys.
var registerModel = porcupine.Model{
	Init: func() any { return register{} },
	Step: func(state, in, out any) (bool, any) {
		s, i, o := state.(register), in.(input), out.(output)
		switch i.kind {
		case kindWrite:
			return true, register{set: true, v: i.value}
		case kindCas:
			// An ok cas took effect; a cas that did not is recorded as fail.
			matches := s.set && s.v == i.expected
			if matches {
				return true, register{set: true, v: i.value}
			}
			return i.info, s
		default:
			if o.result == nil {
				return !s.set, s
			}
			return s.set && s.v == *o.result, s
		}
	},
}
