package handler

import (
	"net/http"

	"github.com/ayush00git/quorum/pkg/agent"
	"github.com/ayush00git/quorum/pkg/httpx"
)

var (
	agentEngine agent.Engine = agent.CoreEngine{}
	agentModel               = func(cfg agent.Config) agent.Model {
		return agent.NewAnthropicModel(cfg.APIKey, cfg.Model)
	}
)

// Agent serves POST /api/agent: exactly one Claude call, with any tool
// calls executed in-process. The browser holds the conversation.
func Agent(w http.ResponseWriter, r *http.Request) {
	httpx.Handle(w, r, http.MethodPost, func(r *http.Request) (any, error) {
		var req agent.Request
		if err := httpx.DecodeJSON(r, &req); err != nil {
			return nil, err
		}
		cfg := agent.ConfigFromEnv()
		if err := httpx.CheckAccessCode(cfg.AccessCode, req.AccessCode); err != nil {
			return nil, err
		}
		if cfg.APIKey == "" {
			return nil, &httpx.Error{Status: http.StatusInternalServerError, Code: httpx.CodeNotConfigured, Message: "ANTHROPIC_API_KEY is not set on the server"}
		}
		s := agent.Session{Model: agentModel(cfg), Engine: agentEngine, Config: cfg}
		resp, err := s.Step(r.Context(), req)
		if err != nil {
			return nil, agent.HTTPError(err)
		}
		return resp, nil
	})
}
