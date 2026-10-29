package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// scriptedModel returns canned responses in order and records requests.
type scriptedModel struct {
	responses []ModelResponse
	requests  []ModelRequest
}

func (m *scriptedModel) Create(_ context.Context, req ModelRequest) (ModelResponse, error) {
	m.requests = append(m.requests, req)
	if len(m.responses) == 0 {
		return ModelResponse{Content: []json.RawMessage{textBlock("idle")}, StopReason: "end_turn"}, nil
	}
	r := m.responses[0]
	m.responses = m.responses[1:]
	return r, nil
}

var toolSeq int

func toolUseBlock(name string, input any) json.RawMessage {
	toolSeq++
	return mustJSON(map[string]any{"type": "tool_use", "id": fmt.Sprintf("toolu_%d", toolSeq), "name": name, "input": input})
}

func assistantTurn(blocks ...json.RawMessage) ModelResponse {
	return ModelResponse{Content: blocks, StopReason: "tool_use", Usage: Usage{InputTokens: 100, OutputTokens: 20}}
}

var isolateSpec = map[string]any{
	"seed":   1,
	"faults": []any{map[string]any{"atMs": 500, "action": "isolate", "args": map[string]any{"node": "n1"}}, map[string]any{"atMs": 900, "action": "heal", "args": map[string]any{}}},
}

var quietSpec = map[string]any{"seed": 1}
