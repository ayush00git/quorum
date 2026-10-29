package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ayush00git/quorum/pkg/httpx"
)

// Per-step limits for the model call.
const (
	stepMaxTokens = 8192
	modelTimeout  = 50 * time.Second
	stopRefusal   = "refusal"
)

// Session runs agent steps. The same code serves /api/agent and cmd/record.
type Session struct {
	Model  Model
	Engine Engine
	Config Config
}

// Step validates the request, makes exactly one Claude call, executes any
// tool calls in-process and returns the step. Validation failures are
// *httpx.Error; model failures are *UpstreamError.
func (s Session) Step(ctx context.Context, req Request) (Response, error) {
	budget := s.Config.StepBudget(req.MaxSteps)
	if !KnownTarget(s.Engine, req.Target) {
		return Response{}, httpx.BadRequest("unknown target %q", req.Target)
	}
	if err := validateMessages(req.Messages, budget); err != nil {
		return Response{}, err
	}

	var kickoff *Message
	messages := req.Messages
	if len(messages) == 0 {
		k := Kickoff(req.Target, budget, req.Seed)
		kickoff = &k
		messages = []Message{k}
	}
	prior := 0
	for _, m := range messages {
		if m.Role == RoleAssistant {
			prior++
		}
	}
	if prior >= budget {
		return Response{}, httpx.BadRequest("the step budget of %d steps is already used", budget)
	}
	step := prior + 1

	reg := newRegistry(s.Engine, req.Target)
	reg.scan(messages)

	callCtx, cancel := context.WithTimeout(ctx, modelTimeout)
	defer cancel()
	out, err := s.Model.Create(callCtx, ModelRequest{
		System:    SystemPrompt,
		Tools:     Tools(),
		Messages:  messages,
		MaxTokens: stepMaxTokens,
	})
	if err != nil {
		return Response{}, err
	}
	if len(out.Content) == 0 && out.StopReason != stopRefusal {
		return Response{}, &UpstreamError{Message: "Claude returned an empty response (stop reason " + out.StopReason + ")"}
	}

	x := newExecutor(s.Engine, req.Target, reg, step, budget)
	uses := toolUses(out.Content)
	results := make([]json.RawMessage, 0, len(uses))
	for _, tu := range uses {
		results = append(results, x.result(tu))
	}

	resp := Response{
		Kickoff:     kickoff,
		Assistant:   out.Content,
		ToolResults: results,
		Experiments: x.experiments,
		Usage:       out.Usage,
		Step:        step,
	}
	if resp.Assistant == nil {
		resp.Assistant = []json.RawMessage{}
	}
	if resp.Experiments == nil {
		resp.Experiments = []ExperimentSummary{}
	}
	switch {
	case x.finding != nil:
		resp.Done, resp.Finding = true, x.finding
	case x.conclusion != "":
		resp.Done, resp.Conclusion = true, x.conclusion
	case out.StopReason == stopRefusal:
		resp.Done, resp.Conclusion = true, "Claude declined to continue this run."
	case step >= budget:
		resp.Done, resp.Conclusion = true, fmt.Sprintf("The step budget of %d steps ran out without a checker-confirmed violation.", budget)
	case len(uses) == 0:
		// Claude ended its turn without a tool call. Nudge it so the
		// conversation keeps alternating user/assistant turns.
		resp.ToolResults = []json.RawMessage{textBlock(
			"No tool was called. Continue with the tools and end the run with report_finding or conclude.\n\n" + x.budgetLine())}
	}
	if resp.Done {
		resp.RevealedBug, _ = s.Engine.RevealBug(req.Target)
	}
	return resp, nil
}

func validateMessages(msgs []Message, budget int) error {
	if max := 2*budget + 2; len(msgs) > max {
		return httpx.BadRequest("messages has %d entries; at most %d are allowed for a %d-step budget", len(msgs), max, budget)
	}
	for i, m := range msgs {
		want := RoleUser
		if i%2 == 1 {
			want = RoleAssistant
		}
		if m.Role != RoleUser && m.Role != RoleAssistant {
			return httpx.BadRequest("messages[%d].role must be user or assistant", i)
		}
		if m.Role != want {
			return httpx.BadRequest("messages[%d].role must be %s: roles alternate, starting with user", i, want)
		}
		if len(m.Content) == 0 {
			return httpx.BadRequest("messages[%d].content must not be empty", i)
		}
		for j, raw := range m.Content {
			if _, ok := readHeader(raw); !ok {
				return httpx.BadRequest("messages[%d].content[%d] must be an object with a type", i, j)
			}
		}
	}
	if n := len(msgs); n > 0 && msgs[n-1].Role != RoleUser {
		return httpx.BadRequest("the last message must be a user message (tool results)")
	}
	return nil
}

// HTTPError maps an error from Step to the public HTTP error.
func HTTPError(err error) error {
	var ue *UpstreamError
	if !errors.As(err, &ue) {
		return err
	}
	switch ue.Status {
	case http.StatusTooManyRequests:
		return &httpx.Error{Status: http.StatusTooManyRequests, Code: httpx.CodeRateLimited, Message: "Claude is rate limited; retry shortly"}
	case 529:
		return &httpx.Error{Status: http.StatusServiceUnavailable, Code: httpx.CodeOverloaded, Message: "Claude is overloaded; retry shortly"}
	default:
		return &httpx.Error{Status: http.StatusBadGateway, Code: httpx.CodeUpstream, Message: ue.Error()}
	}
}
