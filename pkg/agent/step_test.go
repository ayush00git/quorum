package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/ayush00git/quorum/pkg/agent/agenttest"
	"github.com/ayush00git/quorum/pkg/checker"
	"github.com/ayush00git/quorum/pkg/httpx"
)

func testSession(m Model) Session {
	return Session{Model: m, Engine: &agenttest.FakeEngine{}, Config: Config{Model: "test-model", MaxSteps: 30}}
}

// drive plays the browser: it appends each step and calls again until done.
func drive(t *testing.T, s Session, req Request, limit int) []Response {
	t.Helper()
	var out []Response
	for range limit {
		resp, err := s.Step(context.Background(), req)
		if err != nil {
			t.Fatalf("step %d: %v", len(out)+1, err)
		}
		out = append(out, resp)
		if resp.Kickoff != nil {
			req.Messages = append(req.Messages, *resp.Kickoff)
		}
		req.Messages = append(req.Messages, Message{Role: RoleAssistant, Content: resp.Assistant})
		if resp.Done {
			return out
		}
		req.Messages = append(req.Messages, Message{Role: RoleUser, Content: resp.ToolResults})
	}
	t.Fatalf("run not done after %d steps", limit)
	return nil
}

func TestStepFullRunReportsFinding(t *testing.T) {
	e := &agenttest.FakeEngine{}
	badID := specID(t, e, isolateSpec)
	spec, _ := parseToolSpec(e, mustJSON(isolateSpec), "target-c")
	sr, _ := e.Shrink(spec)
	model := &scriptedModel{responses: []ModelResponse{
		assistantTurn(textBlock("Let me look."), toolUseBlock(ToolClusterInfo, map[string]any{})),
		assistantTurn(textBlock("Hypothesis: isolating the leader."), toolUseBlock(ToolRunExperiment, map[string]any{"spec": isolateSpec})),
		assistantTurn(toolUseBlock(ToolShrink, map[string]any{"experimentId": badID})),
		assistantTurn(toolUseBlock(ToolReportFinding, map[string]any{
			"experimentId": sr.Result.ID, "title": "Stale read", "anomaly": "r(x) saw an old value", "causalChain": []string{"isolate n1 at 500ms"},
		})),
	}}
	s := testSession(model)
	steps := drive(t, s, Request{Target: "target-c", MaxSteps: 12}, 10)

	if len(steps) != 4 {
		t.Fatalf("got %d steps, want 4", len(steps))
	}
	first := steps[0]
	if first.Kickoff == nil || first.Step != 1 || len(first.ToolResults) != 1 {
		t.Fatalf("bad first step: %+v", first)
	}
	if !strings.Contains(string(first.Kickoff.Content[0]), "Step budget: 12") {
		t.Fatalf("kickoff missing budget: %s", first.Kickoff.Content[0])
	}
	if steps[1].Kickoff != nil {
		t.Fatal("kickoff must only appear on the first step")
	}
	if got := steps[1].Experiments; len(got) != 1 || got[0].ID != badID || got[0].Verdict != checker.VerdictViolation {
		t.Fatalf("step 2 experiments: %+v", got)
	}
	if got := steps[2].Experiments; len(got) != 1 || got[0].ShrunkFrom != badID {
		t.Fatalf("step 3 experiments: %+v", got)
	}
	last := steps[3]
	if !last.Done || last.Finding == nil || last.Finding.ExperimentID != sr.Result.ID || last.RevealedBug != "stale leader reads" {
		t.Fatalf("bad final step: %+v", last)
	}
	for i, st := range steps[:3] {
		if st.Done || st.RevealedBug != "" || st.Finding != nil {
			t.Fatalf("step %d leaked end-of-run fields", i+1)
		}
	}

	req := model.requests[0]
	if req.System != SystemPrompt || len(req.Tools) != 7 || req.MaxTokens != stepMaxTokens {
		t.Fatalf("model request not server-owned: %+v", req)
	}
	if n := len(model.requests[3].Messages); n != 7 {
		t.Fatalf("fourth call saw %d messages, want 7", n)
	}
}

func TestStepRejectedFindingKeepsRunning(t *testing.T) {
	e := &agenttest.FakeEngine{}
	okID := specID(t, e, quietSpec)
	model := &scriptedModel{responses: []ModelResponse{
		assistantTurn(toolUseBlock(ToolRunExperiment, map[string]any{"spec": quietSpec})),
		assistantTurn(toolUseBlock(ToolReportFinding, map[string]any{"experimentId": okID, "title": "t", "anomaly": "a", "causalChain": []string{"c"}})),
		assistantTurn(toolUseBlock(ToolConclude, map[string]any{"summary": "Nothing found."})),
	}}
	steps := drive(t, testSession(model), Request{Target: "target-c"}, 10)
	if steps[1].Done || !strings.Contains(string(steps[1].ToolResults[0]), "rejected") {
		t.Fatalf("report on ok experiment was not rejected: %s", steps[1].ToolResults[0])
	}
	last := steps[len(steps)-1]
	if !last.Done || last.Conclusion != "Nothing found." || last.RevealedBug != "stale leader reads" || last.Finding != nil {
		t.Fatalf("bad conclusion step: %+v", last)
	}
}

func TestStepBudgetExhaustion(t *testing.T) {
	var responses []ModelResponse
	for range 12 {
		responses = append(responses, assistantTurn(toolUseBlock(ToolClusterInfo, map[string]any{})))
	}
	model := &scriptedModel{responses: responses}
	steps := drive(t, testSession(model), Request{Target: "target-a", MaxSteps: 10}, 20)
	last := steps[len(steps)-1]
	if len(steps) != 10 || !last.Done || !strings.Contains(last.Conclusion, "budget of 10 steps") || last.RevealedBug == "" {
		t.Fatalf("budget not enforced: %d steps, last %+v", len(steps), last)
	}
	var block struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(steps[8].ToolResults[0], &block); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(block.Content, `budget: {"stepsUsed":9,"stepsLeft":1}`) {
		t.Fatalf("budget line wrong: %s", block.Content)
	}
}

func TestStepNudgesWhenNoToolCalled(t *testing.T) {
	model := &scriptedModel{responses: []ModelResponse{{Content: []json.RawMessage{textBlock("thinking out loud")}, StopReason: "end_turn"}}}
	resp, err := testSession(model).Step(context.Background(), Request{Target: "target-a"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Done || len(resp.ToolResults) != 1 {
		t.Fatalf("unexpected response %+v", resp)
	}
	h, _ := readHeader(resp.ToolResults[0])
	if h.Type != "text" {
		t.Fatalf("nudge should be a text block, got %s", h.Type)
	}
}

func TestStepRefusalEndsRun(t *testing.T) {
	model := &scriptedModel{responses: []ModelResponse{{StopReason: "refusal"}}}
	resp, err := testSession(model).Step(context.Background(), Request{Target: "target-a"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Done || resp.Conclusion == "" || resp.Assistant == nil {
		t.Fatalf("refusal not handled: %+v", resp)
	}
}

func TestStepValidation(t *testing.T) {
	user := Message{Role: RoleUser, Content: []json.RawMessage{textBlock("hi")}}
	asst := Message{Role: RoleAssistant, Content: []json.RawMessage{textBlock("ok")}}
	many := []Message{}
	for range 12 {
		many = append(many, user, asst)
	}
	full := []Message{}
	for range 10 {
		full = append(full, user, asst)
	}
	full = append(full, user)
	tests := []struct {
		name string
		req  Request
		want string
	}{
		{"unknown target", Request{Target: "target-z"}, "unknown target"},
		{"system role", Request{Target: "target-a", Messages: []Message{{Role: "system", Content: user.Content}}}, "must be user or assistant"},
		{"starts with assistant", Request{Target: "target-a", Messages: []Message{asst}}, "roles alternate"},
		{"ends with assistant", Request{Target: "target-a", Messages: []Message{user, asst}}, "last message must be a user"},
		{"empty content", Request{Target: "target-a", Messages: []Message{{Role: RoleUser}}}, "must not be empty"},
		{"non-object block", Request{Target: "target-a", Messages: []Message{{Role: RoleUser, Content: []json.RawMessage{json.RawMessage(`"hi"`)}}}}, "must be an object"},
		{"too many messages", Request{Target: "target-a", MaxSteps: 10, Messages: many}, "at most 22"},
		{"budget used", Request{Target: "target-a", MaxSteps: 10, Messages: full}, "already used"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := &scriptedModel{}
			_, err := testSession(model).Step(context.Background(), tc.req)
			var he *httpx.Error
			if !errors.As(err, &he) || he.Status != http.StatusBadRequest || !strings.Contains(he.Message, tc.want) {
				t.Fatalf("got %v, want 400 containing %q", err, tc.want)
			}
			if len(model.requests) != 0 {
				t.Fatal("invalid requests must not reach the model")
			}
		})
	}
}

func TestHTTPError(t *testing.T) {
	tests := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{&UpstreamError{Status: 429}, http.StatusTooManyRequests, httpx.CodeRateLimited},
		{&UpstreamError{Status: 529}, http.StatusServiceUnavailable, httpx.CodeOverloaded},
		{&UpstreamError{Status: 400, Message: "bad"}, http.StatusBadGateway, httpx.CodeUpstream},
		{&UpstreamError{Message: "request timed out"}, http.StatusBadGateway, httpx.CodeUpstream},
	}
	for _, tc := range tests {
		var he *httpx.Error
		if !errors.As(HTTPError(tc.err), &he) || he.Status != tc.wantStatus || he.Code != tc.wantCode {
			t.Errorf("%v: got %+v", tc.err, he)
		}
	}
	plain := errors.New("x")
	if HTTPError(plain) != plain {
		t.Error("non-upstream errors pass through")
	}
}

func TestConfigFromEnv(t *testing.T) {
	tests := []struct{ env, want int }{{0, 30}, {5, MinSteps}, {25, 25}, {99, MaxStepsCap}}
	for _, tc := range tests {
		if tc.env == 0 {
			t.Setenv("AGENT_MAX_STEPS", "")
		} else {
			t.Setenv("AGENT_MAX_STEPS", strconv.Itoa(tc.env))
		}
		t.Setenv("ANTHROPIC_MODEL", "")
		cfg := ConfigFromEnv()
		if cfg.MaxSteps != tc.want || cfg.Model != DefaultModel {
			t.Errorf("AGENT_MAX_STEPS=%d: got %d %s", tc.env, cfg.MaxSteps, cfg.Model)
		}
	}
	cfg := Config{MaxSteps: 25}
	if cfg.StepBudget(0) != 25 || cfg.StepBudget(3) != MinSteps || cfg.StepBudget(99) != 25 || cfg.StepBudget(15) != 15 {
		t.Error("StepBudget clamping is wrong")
	}
}
