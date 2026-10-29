package experiment

import "github.com/ayush00git/quorum/pkg/raft"

func publicCodenames() []string {
	var out []string
	for _, c := range raft.Codenames() {
		if t, _ := raft.LookupTarget(c); t.Enabled {
			out = append(out, c)
		}
	}
	return out
}

func isPublic(codename string) bool {
	t, ok := raft.LookupTarget(codename)
	return ok && t.Enabled
}

// Targets lists the public targets in display order. It never includes the
// planted bug.
func Targets() []TargetInfo {
	var out []TargetInfo
	for _, c := range publicCodenames() {
		t, _ := raft.LookupTarget(c)
		out = append(out, TargetInfo{Codename: c, Difficulty: t.Difficulty})
	}
	return out
}

// RevealBug returns the planted bug's human name for a codename ("none" for
// the control). Only call it once a run has ended.
func RevealBug(codename string) (string, bool) {
	if !isPublic(codename) {
		return "", false
	}
	t, _ := raft.LookupTarget(codename)
	return t.Bug, true
}
