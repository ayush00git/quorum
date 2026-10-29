// Command record runs a full agent session headlessly, with the same loop
// as /api/agent, and writes public/runs/<target>-<seed>.json.
//
//	go run ./cmd/record -target target-c -seed 42
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ayush00git/quorum/pkg/agent"
)

const (
	maxAttempts = 6
	baseBackoff = 2 * time.Second
)

func main() {
	target := flag.String("target", "", "target codename, e.g. target-c (required)")
	seed := flag.Int64("seed", 42, "base seed suggested to the agent; also names the run")
	steps := flag.Int("steps", 0, "step budget (default: AGENT_MAX_STEPS)")
	out := flag.String("out", "public/runs", "output directory")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := record(ctx, *target, *seed, *steps, *out); err != nil {
		fmt.Fprintln(os.Stderr, "record:", err)
		os.Exit(1)
	}
}

func record(ctx context.Context, target string, seed int64, steps int, out string) error {
	cfg := agent.ConfigFromEnv()
	engine := agent.CoreEngine{}
	if target == "" {
		return errors.New("-target is required (e.g. -target target-c)")
	}
	if !agent.KnownTarget(engine, target) {
		return fmt.Errorf("unknown target %q", target)
	}
	if cfg.APIKey == "" {
		return errors.New("ANTHROPIC_API_KEY is not set: export it, or put it in .env and run make record TARGET=... SEED=...")
	}

	s := agent.Session{Model: agent.NewAnthropicModel(cfg.APIKey, cfg.Model), Engine: engine, Config: cfg}
	req := agent.Request{Target: target, MaxSteps: steps, Seed: &seed, Messages: []agent.Message{}}
	rf := runFile{
		Version:   1,
		Target:    target,
		Model:     cfg.Model,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:     []recordedStep{},
	}
	fmt.Fprintf(os.Stderr, "recording %s (seed %d, model %s, budget %d steps)\n", target, seed, cfg.Model, cfg.StepBudget(steps))

	for {
		resp, err := stepWithRetry(ctx, s, req)
		if err != nil {
			return err
		}
		if resp.Kickoff != nil {
			req.Messages = append(req.Messages, *resp.Kickoff)
		}
		req.Messages = append(req.Messages, agent.Message{Role: agent.RoleAssistant, Content: resp.Assistant})
		rf.Steps = append(rf.Steps, recordedStep{Assistant: resp.Assistant, ToolResults: resp.ToolResults, Experiments: resp.Experiments})
		rf.Usage.Add(resp.Usage)
		logStep(resp)
		if resp.Done {
			rf.Finding = resp.Finding
			if resp.Conclusion != "" {
				rf.Conclusion = &resp.Conclusion
			}
			rf.RevealedBug = resp.RevealedBug
			break
		}
		req.Messages = append(req.Messages, agent.Message{Role: agent.RoleUser, Content: resp.ToolResults})
	}

	if err := collectExperiments(engine, &rf); err != nil {
		return err
	}
	path, err := saveRun(out, fmt.Sprintf("%s-%d", target, seed), rf)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "done in %d steps (finding: %t, bug: %s); wrote %s\n", len(rf.Steps), rf.Finding != nil, rf.RevealedBug, path)
	return nil
}

// stepWithRetry backs off on rate limits, overload and transport failures,
// as the browser does.
func stepWithRetry(ctx context.Context, s agent.Session, req agent.Request) (agent.Response, error) {
	backoff := baseBackoff
	for attempt := 1; ; attempt++ {
		resp, err := s.Step(ctx, req)
		if err == nil {
			return resp, nil
		}
		var ue *agent.UpstreamError
		retryable := errors.As(err, &ue) && (ue.Retryable() || ue.Status == 0 || ue.Status >= 500)
		if !retryable || attempt == maxAttempts || ctx.Err() != nil {
			return agent.Response{}, err
		}
		fmt.Fprintf(os.Stderr, "  %v; retrying in %s\n", err, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return agent.Response{}, ctx.Err()
		}
		backoff *= 2
	}
}

func logStep(resp agent.Response) {
	var parts []string
	for _, ex := range resp.Experiments {
		parts = append(parts, fmt.Sprintf("%s %s", ex.ID, ex.Verdict))
	}
	line := fmt.Sprintf("step %d: %d tool results", resp.Step, len(resp.ToolResults))
	if len(parts) > 0 {
		line += "; experiments: " + strings.Join(parts, ", ")
	}
	fmt.Fprintln(os.Stderr, line)
}
