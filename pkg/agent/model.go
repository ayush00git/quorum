package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool is a server-owned tool definition.
type Tool struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
}

// ModelRequest is one Claude call. Everything except Messages is server-owned.
type ModelRequest struct {
	System    string
	Tools     []Tool
	Messages  []Message
	MaxTokens int64
}

// ModelResponse is the part of a Claude response the loop needs.
type ModelResponse struct {
	Content    []json.RawMessage
	StopReason string
	Usage      Usage
}

// Model abstracts the Anthropic Messages API so the loop can be tested
// and run headlessly with the same code.
type Model interface {
	Create(ctx context.Context, req ModelRequest) (ModelResponse, error)
}

// UpstreamError is a failed call to the model provider. Status is the
// provider's HTTP status, or 0 for transport failures and timeouts.
type UpstreamError struct {
	Status  int
	Message string
}

func (e *UpstreamError) Error() string {
	if e.Status == 0 {
		return "anthropic: " + e.Message
	}
	return fmt.Sprintf("anthropic: status %d: %s", e.Status, e.Message)
}

// Retryable reports whether the caller should back off and retry
// (rate limited or overloaded).
func (e *UpstreamError) Retryable() bool { return e.Status == 429 || e.Status == 529 }
