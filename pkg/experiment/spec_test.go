package experiment

import (
	"strings"
	"testing"
)

func TestParseSpecDefaults(t *testing.T) {
	s, err := ParseSpec([]byte(`{"target":"target-a","seed":9}`))
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultSpec("target-a")
	want.Seed = 9
	if SpecID(s) != SpecID(want) {
		t.Fatalf("defaults not applied: %+v", s)
	}
	if s.Faults == nil {
		t.Fatal("faults must normalize to an empty list, not null")
	}
}

func TestParseSpecSortsFaultsStably(t *testing.T) {
	s, err := ParseSpec([]byte(`{"target":"target-a","faults":[
		{"atMs":900,"action":"heal","args":{}},
		{"atMs":100,"action":"isolate","args":{"node":"n1"}},
		{"atMs":100,"action":"crash","args":{"node":"n2"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{s.Faults[0].Action, s.Faults[1].Action, s.Faults[2].Action}
	if strings.Join(got, ",") != "isolate,crash,heal" {
		t.Fatalf("order %v", got)
	}
}

func TestParseSpecErrors(t *testing.T) {
	cases := []struct{ json, want string }{
		{`{"seed":1}`, "target: required"},
		{`{"target":"target-z"}`, `target: unknown target "target-z"`},
		{`{"target":"target-a","bogus":1}`, `unknown field "bogus"`},
		{`{"target":"target-a"} {}`, "trailing data"},
		{`{"target":"target-a","durationMs":20000}`, "durationMs: must be between 100 and 10000"},
		{`{"target":"target-a","nodes":3}`, "nodes: the cluster always has 5 nodes"},
		{`{"target":"target-a","clients":11}`, "clients: must be between 1 and 10"},
		{`{"target":"target-a","keys":9}`, "keys: must be between 1 and 8"},
		{`{"target":"target-a","readRatio":1.5}`, "readRatio: must be between 0 and 1"},
		{`{"target":"target-a","readFrom":"followers"}`, `readFrom: must be "leader" or "any"`},
		{`{"target":"target-a","opTimeoutMs":10}`, "opTimeoutMs: must be between 50 and 5000"},
		{`{"target":"target-a","network":{"minLatencyMs":50,"maxLatencyMs":10}}`, "network.maxLatencyMs"},
		{`{"target":"target-a","network":{"dropRate":0.95}}`, "network.dropRate"},
		{`{"target":"target-a","faults":[{"atMs":5000,"action":"heal","args":{}}]}`, "faults[0].atMs: must be between 0 and durationMs-1"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"explode","args":{}}]}`, `faults[0].action: unknown action "explode"`},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"heal","args":{}},{"atMs":2,"action":"crash","args":{"node":"n9"}}]}`, `faults[1].args.node: unknown node "n9"`},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"crash","args":{}}]}`, "faults[0].args.node: required for crash"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"heal","args":{"node":"n1"}}]}`, "faults[0].args.node: not allowed for heal"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"isolate","args":{"node":"n1","bogus":2}}]}`, `unknown field "bogus"`},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"pause","args":{"node":"n1"}}]}`, "faults[0].args.durationMs: must be between 1 and 5000"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"partition","args":{"groups":[["n1"]]}}]}`, "need at least 2 groups"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"partition","args":{"groups":[["n1"],["n1","n2"]]}}]}`, `node "n1" appears in more than one group`},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"partition","args":{"groups":[["n1"],[]]}}]}`, "args.groups[1]: empty group"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"setNetwork","args":{}}]}`, "setNetwork needs at least one"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"setNetwork","args":{"minLatencyMs":40}}]}`, "args.maxLatencyMs: must be between minLatencyMs (40)"},
		{`{"target":"target-a","faults":[{"atMs":1,"action":"skewClock","args":{"node":"n1","driftPpm":900000}}]}`, "args.driftPpm"},
	}
	for _, tc := range cases {
		_, err := ParseSpec([]byte(tc.json))
		if err == nil {
			t.Errorf("%s: accepted, want error containing %q", tc.json, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q, want it to contain %q", tc.json, err, tc.want)
		}
	}
}

func TestTooManyFaults(t *testing.T) {
	s := DefaultSpec("target-a")
	for i := 0; i <= MaxFaults; i++ {
		s.Faults = append(s.Faults, Fault{AtMs: int64(i), Action: ActionHeal})
	}
	if _, err := Normalize(s); err == nil || !strings.Contains(err.Error(), "at most 30 faults") {
		t.Fatalf("err %v", err)
	}
}

func TestTargetsHideBugs(t *testing.T) {
	ts := Targets()
	if len(ts) < 5 || ts[0].Codename != "target-a" {
		t.Fatalf("targets %+v", ts)
	}
	for _, tg := range ts {
		bug, ok := RevealBug(tg.Codename)
		if !ok || bug == "" {
			t.Fatalf("no bug name for %s", tg.Codename)
		}
	}
	if bug, _ := RevealBug("target-a"); bug != "none" {
		t.Fatalf("control reveals %q, want none", bug)
	}
	if _, ok := RevealBug("target-zz"); ok {
		t.Fatal("unknown codename revealed")
	}
}
