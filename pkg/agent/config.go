package agent

import (
	"os"
	"strconv"
	"strings"
)

// Step budget bounds and defaults.
const (
	MinSteps        = 10
	MaxStepsCap     = 40
	DefaultMaxSteps = 30
	DefaultModel    = "claude-sonnet-5-5"
)

// Config is the server-owned agent configuration, read from the environment.
type Config struct {
	APIKey     string
	Model      string
	MaxSteps   int
	AccessCode string
}

// ConfigFromEnv reads ANTHROPIC_API_KEY, ANTHROPIC_MODEL, AGENT_MAX_STEPS
// and ACCESS_CODE. AGENT_MAX_STEPS is clamped to [MinSteps, MaxStepsCap].
func ConfigFromEnv() Config {
	cfg := Config{
		APIKey:     strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")),
		Model:      strings.TrimSpace(os.Getenv("ANTHROPIC_MODEL")),
		MaxSteps:   DefaultMaxSteps,
		AccessCode: os.Getenv("ACCESS_CODE"),
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("AGENT_MAX_STEPS"))); err == nil {
		cfg.MaxSteps = clamp(n, MinSteps, MaxStepsCap)
	}
	return cfg
}

// StepBudget resolves a requested budget against the server maximum.
// Zero means "use the server maximum".
func (c Config) StepBudget(requested int) int {
	if requested == 0 {
		return c.MaxSteps
	}
	return clamp(requested, MinSteps, c.MaxSteps)
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
