package experiment

import "errors"

// This file is a placeholder so dependent packages compile while the core is
// built in parallel. The core work package deletes it and provides the real
// implementations with exactly these signatures.

var errNotImplemented = errors.New("experiment: not implemented")

// ParseSpec strictly decodes JSON (DisallowUnknownFields), fills defaults for
// zero-valued optional fields and validates. Errors are user-facing.
func ParseSpec(data []byte) (Spec, error) { return Spec{}, errNotImplemented }

// Normalize fills defaults and validates an already-decoded spec.
func Normalize(s Spec) (Spec, error) { return Spec{}, errNotImplemented }

// SpecID is a short hash of the canonical JSON of a normalized spec.
func SpecID(s Spec) string { return "" }

// Run executes a normalized spec. Pure and deterministic.
func Run(s Spec) (Result, error) { return Result{}, errNotImplemented }

// Summarize renders a compact, token-efficient text summary for the agent.
func Summarize(r Result) string { return "" }

// Shrink minimizes a violating spec (ddmin over faults, then duration,
// clients, keys), capped at about 80 runs.
func Shrink(s Spec) (ShrinkResult, error) { return ShrinkResult{}, errNotImplemented }

// Targets lists the public targets in display order.
func Targets() []TargetInfo { return nil }

// RevealBug returns the planted bug's human name for a codename ("none" for
// the control). Only called once a run has ended.
func RevealBug(codename string) (string, bool) { return "", false }

// DefaultSpec returns the defaults used by Normalize for a target.
func DefaultSpec(target string) Spec { return Spec{Target: target} }
