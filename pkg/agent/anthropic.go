package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// anthropicModel is the production Model backed by the Anthropic Go SDK.
type anthropicModel struct {
	client anthropic.Client
	model  string
}

// NewAnthropicModel builds a Model for the given API key and model id.
// The SDK does not retry: the browser (and cmd/record) back off on 429
// and 529 themselves, which keeps each request well under 60 seconds.
func NewAnthropicModel(apiKey, model string, opts ...option.RequestOption) Model {
	base := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(0),
		option.WithRequestTimeout(modelTimeout),
	}
	return &anthropicModel{client: anthropic.NewClient(append(base, opts...)...), model: model}
}

func (m *anthropicModel) Create(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	tools := make([]anthropic.ToolUnionParam, len(req.Tools))
	for i, t := range req.Tools {
		u := anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{Properties: t.Properties, Required: t.Required}, t.Name)
		u.OfTool.Description = anthropic.String(t.Description)
		if i == len(req.Tools)-1 {
			u.OfTool.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		tools[i] = u
	}
	// Messages are forwarded as the exact JSON the browser holds, so every
	// content block (thinking blocks included) round-trips byte for byte.
	messages, err := json.Marshal(req.Messages)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("encode messages: %w", err)
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(m.model),
		MaxTokens: req.MaxTokens,
		System: []anthropic.TextBlockParam{{
			Text:         req.System,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Tools: tools,
		// Medium effort keeps each step's latency well inside the 60 s limit.
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortMedium},
		// Automatic breakpoint on the newest message caches the growing history.
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
	msg, err := m.client.Messages.New(ctx, params, option.WithJSONSet("messages", json.RawMessage(messages)))
	if err != nil {
		return ModelResponse{}, upstreamError(err)
	}
	content := make([]json.RawMessage, 0, len(msg.Content))
	for _, block := range msg.Content {
		raw := block.RawJSON()
		if raw == "" {
			b, err := json.Marshal(block)
			if err != nil {
				return ModelResponse{}, fmt.Errorf("encode content block: %w", err)
			}
			raw = string(b)
		}
		content = append(content, json.RawMessage(raw))
	}
	return ModelResponse{
		Content:    content,
		StopReason: string(msg.StopReason),
		Usage: Usage{
			InputTokens:         msg.Usage.InputTokens,
			OutputTokens:        msg.Usage.OutputTokens,
			CacheReadTokens:     msg.Usage.CacheReadInputTokens,
			CacheCreationTokens: msg.Usage.CacheCreationInputTokens,
		},
	}, nil
}

func upstreamError(err error) *UpstreamError {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		msg := fmt.Sprintf("request failed with status %d", apiErr.StatusCode)
		if t := apiErr.Type(); t != "" {
			msg += " (" + string(t) + ")"
		}
		return &UpstreamError{Status: apiErr.StatusCode, Message: msg}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &UpstreamError{Message: "request timed out"}
	}
	if errors.Is(err, context.Canceled) {
		return &UpstreamError{Message: "request canceled"}
	}
	return &UpstreamError{Message: "request failed: " + err.Error()}
}
