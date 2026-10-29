package raft

type session struct {
	seq    int
	result *int
	casOK  bool
}

// stateMachine is the replicated key-value map plus client sessions. It is
// rebuilt from the log after a restart.
type stateMachine struct {
	kv       map[string]int
	sessions map[int]session
}

func newStateMachine() *stateMachine {
	return &stateMachine{kv: map[string]int{}, sessions: map[int]session{}}
}

func (s *stateMachine) get(key string) *int {
	v, ok := s.kv[key]
	if !ok {
		return nil
	}
	return &v
}

// apply executes a command once per (client, seq); a retried command that
// was already applied returns the cached result without a second effect.
func (s *stateMachine) apply(c Command) session {
	if c.Kind == CmdNoop {
		return session{}
	}
	if prev, ok := s.sessions[c.Client]; ok && c.Seq <= prev.seq {
		return prev
	}
	out := session{seq: c.Seq}
	switch c.Kind {
	case CmdRead:
		out.result = s.get(c.Key)
	case CmdWrite:
		s.kv[c.Key] = c.Value
	case CmdCas:
		if cur, ok := s.kv[c.Key]; ok && cur == c.Expected {
			s.kv[c.Key] = c.Value
			out.casOK = true
		}
	}
	s.sessions[c.Client] = out
	return out
}

// cached returns the result of an already applied request.
func (s *stateMachine) cached(client, seq int) (session, bool) {
	prev, ok := s.sessions[client]
	if !ok || prev.seq != seq {
		return session{}, false
	}
	return prev, true
}
