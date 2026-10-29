// Package handler holds the HTTP endpoints, one file per endpoint. Each
// file is self-contained and delegates to pkg/agent and pkg/experiment.
package handler

import (
	"net/http"

	"github.com/ayush00git/quorum/pkg/agent"
	"github.com/ayush00git/quorum/pkg/experiment"
	"github.com/ayush00git/quorum/pkg/httpx"
)

var metaEngine agent.Engine = agent.CoreEngine{}

type metaLimits struct {
	MaxDurationMs int `json:"maxDurationMs"`
	MaxFaults     int `json:"maxFaults"`
	Nodes         int `json:"nodes"`
	MinSteps      int `json:"minSteps"`
	MaxStepsCap   int `json:"maxStepsCap"`
}

type metaResponse struct {
	Model              string                  `json:"model"`
	MaxSteps           int                     `json:"maxSteps"`
	AccessCodeRequired bool                    `json:"accessCodeRequired"`
	Targets            []experiment.TargetInfo `json:"targets"`
	Limits             metaLimits              `json:"limits"`
}

// Meta serves GET /api/meta: model, step budget, targets and limits.
func Meta(w http.ResponseWriter, r *http.Request) {
	httpx.Handle(w, r, http.MethodGet, func(*http.Request) (any, error) {
		cfg := agent.ConfigFromEnv()
		targets := metaEngine.Targets()
		if targets == nil {
			targets = []experiment.TargetInfo{}
		}
		return metaResponse{
			Model:              cfg.Model,
			MaxSteps:           cfg.MaxSteps,
			AccessCodeRequired: cfg.AccessCode != "",
			Targets:            targets,
			Limits: metaLimits{
				MaxDurationMs: experiment.MaxDurationMs,
				MaxFaults:     experiment.MaxFaults,
				Nodes:         experiment.NodeCount,
				MinSteps:      agent.MinSteps,
				MaxStepsCap:   agent.MaxStepsCap,
			},
		}, nil
	})
}
