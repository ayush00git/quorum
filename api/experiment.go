package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ayush00git/quorum/pkg/agent"
	"github.com/ayush00git/quorum/pkg/httpx"
)

var experimentEngine agent.Engine = agent.CoreEngine{}

type experimentRequest struct {
	Spec json.RawMessage `json:"spec"`
}

// Experiment serves POST /api/experiment: it runs one spec and returns the
// full deterministic result.
func Experiment(w http.ResponseWriter, r *http.Request) {
	httpx.Handle(w, r, http.MethodPost, func(r *http.Request) (any, error) {
		var req experimentRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			return nil, err
		}
		if len(req.Spec) == 0 {
			return nil, httpx.BadRequest("spec is required")
		}
		spec, err := experimentEngine.ParseSpec(req.Spec)
		if err != nil {
			return nil, httpx.BadRequest("%v", err)
		}
		if !agent.KnownTarget(experimentEngine, spec.Target) {
			return nil, httpx.BadRequest("unknown target %q", spec.Target)
		}
		res, err := experimentEngine.Run(spec)
		if err != nil {
			return nil, fmt.Errorf("run experiment: %w", err)
		}
		return res, nil
	})
}
