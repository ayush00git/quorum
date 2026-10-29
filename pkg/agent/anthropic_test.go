package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

const fakeMessage = `{
  "id": "msg_1", "type": "message", "role": "assistant", "model": "test-model",
  "content": [
    {"type": "thinking", "thinking": "", "signature": "sig-abc"},
    {"type": "text", "text": "Hypothesis: isolate the leader."},
    {"type": "tool_use", "id": "toolu_1", "name": "cluster_info", "input": {}}
  ],
  "stop_reason": "tool_use", "stop_sequence": null,
  "usage": {"input_tokens": 11, "output_tokens": 22, "cache_read_input_tokens": 33, "cache_creation_input_tokens": 44}
}`

func TestAnthropicModelRequestShape(t *testing.T) {
	var body map[string]any
	var apiKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, fakeMessage)
	}))
	defer srv.Close()

	thinking := json.RawMessage(`{"type":"thinking","thinking":"","signature":"keep-me"}`)
	msgs := []Message{
		Kickoff("target-c", 30, nil),
		{Role: RoleAssistant, Content: []json.RawMessage{thinking, toolUseBlock(ToolClusterInfo, map[string]any{})}},
		{Role: RoleUser, Content: []json.RawMessage{toolResultBlock("toolu_9", "info", false)}},
	}
	m := NewAnthropicModel("sk-test", "test-model", option.WithBaseURL(srv.URL))
	resp, err := m.Create(context.Background(), ModelRequest{System: SystemPrompt, Tools: Tools(), Messages: msgs, MaxTokens: 1234})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if apiKey != "Bearer sk-test" || body["model"] != "test-model" || body["max_tokens"] != float64(1234) {
		t.Fatalf("bad request header/body: key=%q model=%v max_tokens=%v", apiKey, body["model"], body["max_tokens"])
	}
	system := body["system"].([]any)[0].(map[string]any)
	if system["text"] != SystemPrompt || system["cache_control"] == nil {
		t.Fatalf("system prompt not cached: %v", system)
	}
	tools := body["tools"].([]any)
	if len(tools) != 7 || tools[6].(map[string]any)["cache_control"] == nil || tools[0].(map[string]any)["cache_control"] != nil {
		t.Fatalf("cache control must be on the last tool only")
	}
	if tools[1].(map[string]any)["name"] != ToolRunExperiment || tools[1].(map[string]any)["input_schema"] == nil {
		t.Fatalf("tool definition malformed: %v", tools[1])
	}
	sent, _ := json.Marshal(body["messages"])
	want, _ := json.Marshal(msgs)
	var a, b any
	_ = json.Unmarshal(sent, &a)
	_ = json.Unmarshal(want, &b)
	if !jsonEqual(a, b) {
		t.Fatalf("messages not forwarded verbatim:\n got %s\nwant %s", sent, want)
	}

	if len(resp.Content) != 3 || !strings.Contains(string(resp.Content[0]), "sig-abc") {
		t.Fatalf("content blocks not preserved: %s", resp.Content)
	}
	if resp.StopReason != "tool_use" || resp.Usage != (Usage{InputTokens: 11, OutputTokens: 22, CacheReadTokens: 33, CacheCreationTokens: 44}) {
		t.Fatalf("bad stop reason/usage: %+v", resp)
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestAnthropicModelErrors(t *testing.T) {
	for _, status := range []int{429, 529, 400} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"some_error","message":"nope"}}`)
		}))
		m := NewAnthropicModel("sk-secret-key", "test-model", option.WithBaseURL(srv.URL))
		_, err := m.Create(context.Background(), ModelRequest{System: "s", Tools: Tools(), Messages: []Message{Kickoff("target-a", 10, nil)}, MaxTokens: 10})
		srv.Close()
		var ue *UpstreamError
		if !errors.As(err, &ue) || ue.Status != status {
			t.Fatalf("status %d: got %v", status, err)
		}
		if strings.Contains(ue.Error(), "sk-secret-key") {
			t.Fatal("error leaks the API key")
		}
	}
}
