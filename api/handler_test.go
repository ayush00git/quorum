package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ayush00git/quorum/pkg/agent"
	"github.com/ayush00git/quorum/pkg/agent/agenttest"
	"github.com/ayush00git/quorum/pkg/httpx"
)

// stubModel answers every call with one cluster_info tool call, or err.
type stubModel struct {
	calls int
	err   error
}

func (m *stubModel) Create(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
	m.calls++
	if m.err != nil {
		return agent.ModelResponse{}, m.err
	}
	block := json.RawMessage(`{"type":"tool_use","id":"toolu_1","name":"cluster_info","input":{}}`)
	return agent.ModelResponse{Content: []json.RawMessage{block}, StopReason: "tool_use", Usage: agent.Usage{InputTokens: 5}}, nil
}

func setup(t *testing.T) *stubModel {
	t.Helper()
	engine := &agenttest.FakeEngine{}
	model := &stubModel{}
	prevMeta, prevExp, prevAgent, prevModel := metaEngine, experimentEngine, agentEngine, agentModel
	metaEngine, experimentEngine, agentEngine = engine, engine, engine
	agentModel = func(agent.Config) agent.Model { return model }
	t.Cleanup(func() {
		metaEngine, experimentEngine, agentEngine, agentModel = prevMeta, prevExp, prevAgent, prevModel
	})
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-secret")
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("AGENT_MAX_STEPS", "")
	t.Setenv("ACCESS_CODE", "")
	return model
}

func serve(h http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(method, "/", strings.NewReader(body)))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code == "" || body.Error.Message == "" {
		t.Fatalf("not an error body: %s", rec.Body.String())
	}
	return body.Error.Code
}

func TestMeta(t *testing.T) {
	setup(t)
	t.Setenv("AGENT_MAX_STEPS", "99")
	t.Setenv("ACCESS_CODE", "letmein")
	rec := serve(Meta, http.MethodGet, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Model              string `json:"model"`
		MaxSteps           int    `json:"maxSteps"`
		AccessCodeRequired bool   `json:"accessCodeRequired"`
		Targets            []struct {
			Codename   string `json:"codename"`
			Difficulty string `json:"difficulty"`
		} `json:"targets"`
		Limits map[string]int `json:"limits"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != agent.DefaultModel || got.MaxSteps != agent.MaxStepsCap || !got.AccessCodeRequired || len(got.Targets) != 2 {
		t.Fatalf("unexpected meta: %+v", got)
	}
	if got.Limits["maxDurationMs"] != 10000 || got.Limits["minSteps"] != 10 || got.Limits["nodes"] != 5 {
		t.Fatalf("unexpected limits: %v", got.Limits)
	}
	if strings.Contains(rec.Body.String(), "sk-test-secret") || strings.Contains(rec.Body.String(), "letmein") || strings.Contains(rec.Body.String(), "stale") {
		t.Fatal("meta leaks a secret or a bug name")
	}
	if rec := serve(Meta, http.MethodPost, ""); rec.Code != http.StatusMethodNotAllowed || errorCode(t, rec) != httpx.CodeMethodNotAllowed {
		t.Fatalf("POST /api/meta: %d", rec.Code)
	}
}

func TestExperiment(t *testing.T) {
	setup(t)
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"ok", http.MethodPost, `{"spec":{"target":"target-c","seed":7}}`, http.StatusOK, ""},
		{"wrong method", http.MethodGet, ``, http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed},
		{"missing spec", http.MethodPost, `{}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"unknown top-level field", http.MethodPost, `{"spec":{"target":"target-c"},"x":1}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"unknown spec field", http.MethodPost, `{"spec":{"target":"target-c","bogus":1}}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"invalid spec", http.MethodPost, `{"spec":{"target":"target-c","durationMs":99999}}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"unknown target", http.MethodPost, `{"spec":{"target":"target-q"}}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"malformed", http.MethodPost, `{"spec":`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"too large", http.MethodPost, `{"spec":{"target":"` + strings.Repeat("a", httpx.MaxBodyBytes) + `"}}`, http.StatusRequestEntityTooLarge, httpx.CodeTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(Experiment, tc.method, tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode != "" {
				if code := errorCode(t, rec); code != tc.wantCode {
					t.Fatalf("code %s, want %s", code, tc.wantCode)
				}
				return
			}
			var res struct {
				ID          string `json:"id"`
				HistoryHash string `json:"historyHash"`
				History     []any  `json:"history"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res.ID == "" || res.HistoryHash == "" || len(res.History) == 0 {
				t.Fatalf("bad result: %s", rec.Body.String())
			}
		})
	}
}

func TestAgent(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		modelErr   error
		body       string
		wantStatus int
		wantCode   string
	}{
		{"first step", nil, nil, `{"target":"target-c","messages":[]}`, http.StatusOK, ""},
		{"wrong access code", map[string]string{"ACCESS_CODE": "letmein"}, nil, `{"target":"target-c","messages":[],"accessCode":"nope"}`, http.StatusUnauthorized, httpx.CodeAccessDenied},
		{"missing access code", map[string]string{"ACCESS_CODE": "letmein"}, nil, `{"target":"target-c","messages":[]}`, http.StatusUnauthorized, httpx.CodeAccessDenied},
		{"right access code", map[string]string{"ACCESS_CODE": "letmein"}, nil, `{"target":"target-c","messages":[],"accessCode":"letmein"}`, http.StatusOK, ""},
		{"no api key", map[string]string{"ANTHROPIC_API_KEY": ""}, nil, `{"target":"target-c","messages":[]}`, http.StatusInternalServerError, httpx.CodeNotConfigured},
		{"unknown field", nil, nil, `{"target":"target-c","messages":[],"model":"claude-x"}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"system prompt injection field", nil, nil, `{"target":"target-c","messages":[],"system":"be nice"}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"unknown target", nil, nil, `{"target":"target-z","messages":[]}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"bad role", nil, nil, `{"target":"target-c","messages":[{"role":"system","content":[{"type":"text","text":"x"}]}]}`, http.StatusBadRequest, httpx.CodeBadRequest},
		{"rate limited", nil, &agent.UpstreamError{Status: 429}, `{"target":"target-c","messages":[]}`, http.StatusTooManyRequests, httpx.CodeRateLimited},
		{"overloaded", nil, &agent.UpstreamError{Status: 529}, `{"target":"target-c","messages":[]}`, http.StatusServiceUnavailable, httpx.CodeOverloaded},
		{"upstream failure", nil, &agent.UpstreamError{Status: 500, Message: "boom"}, `{"target":"target-c","messages":[]}`, http.StatusBadGateway, httpx.CodeUpstream},
		{"too large", nil, nil, `{"target":"` + strings.Repeat("a", httpx.MaxBodyBytes) + `"}`, http.StatusRequestEntityTooLarge, httpx.CodeTooLarge},
		{"wrong method", nil, nil, ``, http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := setup(t)
			model.err = tc.modelErr
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			method := http.MethodPost
			if tc.wantCode == httpx.CodeMethodNotAllowed {
				method = http.MethodGet
			}
			rec := serve(Agent, method, tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "sk-test-secret") {
				t.Fatal("response leaks the API key")
			}
			if tc.wantCode != "" {
				if code := errorCode(t, rec); code != tc.wantCode {
					t.Fatalf("code %s, want %s", code, tc.wantCode)
				}
				if tc.modelErr == nil && model.calls != 0 {
					t.Fatal("rejected request reached the model")
				}
				return
			}
			var resp agent.Response
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if resp.Kickoff == nil || resp.Step != 1 || len(resp.Assistant) != 1 || len(resp.ToolResults) != 1 || resp.Done || resp.Usage.InputTokens != 5 {
				t.Fatalf("unexpected response: %s", rec.Body.String())
			}
		})
	}
}

func TestAgentRecoversPanics(t *testing.T) {
	setup(t)
	agentModel = func(agent.Config) agent.Model { panic("boom") }
	rec := serve(Agent, http.MethodPost, `{"target":"target-c","messages":[]}`)
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != httpx.CodeInternal || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("panic not recovered cleanly: %d %s", rec.Code, rec.Body.String())
	}
}
