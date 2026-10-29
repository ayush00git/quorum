package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type toolUse struct {
	ID    string
	Name  string
	Input json.RawMessage
}

type blockHeader struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

func readHeader(raw json.RawMessage) (blockHeader, bool) {
	var h blockHeader
	if err := json.Unmarshal(raw, &h); err != nil || h.Type == "" {
		return blockHeader{}, false
	}
	return h, true
}

// toolUses extracts tool_use blocks in order.
func toolUses(content []json.RawMessage) []toolUse {
	var out []toolUse
	for _, raw := range content {
		h, ok := readHeader(raw)
		if !ok || h.Type != "tool_use" {
			continue
		}
		out = append(out, toolUse{ID: h.ID, Name: h.Name, Input: h.Input})
	}
	return out
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("agent: marshal %T: %v", v, err))
	}
	return b
}

func toolResultBlock(toolUseID, text string, isError bool) json.RawMessage {
	block := map[string]any{"type": "tool_result", "tool_use_id": toolUseID, "content": text}
	if isError {
		block["is_error"] = true
	}
	return mustJSON(block)
}

func textBlock(text string) json.RawMessage {
	return mustJSON(map[string]string{"type": "text", "text": text})
}

// decodeInput strictly decodes a tool input object into dst.
func decodeInput(input json.RawMessage, dst any) error {
	if len(bytes.TrimSpace(input)) == 0 || bytes.Equal(bytes.TrimSpace(input), []byte("null")) {
		input = json.RawMessage("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("input is empty")
		}
		return fmt.Errorf("invalid input: %w", err)
	}
	return nil
}
