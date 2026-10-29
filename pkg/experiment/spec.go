package experiment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ayush00git/quorum/pkg/raft"
)

// Spec limits beyond those in types.go.
const (
	MinDurationMs = 100
	MaxClients    = 10
	MaxKeys       = 8
	MaxLatencyMs  = 1000
	MaxRate       = 0.9
	MinOpTimeout  = 50
	MaxOpTimeout  = 5000
	MaxPauseMs    = 5000
	MaxDriftPpm   = 500_000
)

// DefaultSpec returns the defaults used by Normalize for a target.
func DefaultSpec(target string) Spec {
	return Spec{
		Target:      target,
		DurationMs:  3000,
		Nodes:       NodeCount,
		Clients:     4,
		Keys:        2,
		ReadRatio:   0.5,
		ReadFrom:    "leader",
		OpTimeoutMs: 500,
		Network:     Network{MinLatencyMs: 1, MaxLatencyMs: 10},
		Faults:      []Fault{},
	}
}

// ParseSpec strictly decodes JSON (unknown fields are rejected), fills
// defaults for zero-valued optional fields and validates. Errors are
// user-facing.
func ParseSpec(data []byte) (Spec, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s Spec
	if err := dec.Decode(&s); err != nil {
		return Spec{}, fmt.Errorf("invalid spec JSON: %s", strings.TrimPrefix(err.Error(), "json: "))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Spec{}, errors.New("invalid spec JSON: trailing data after the spec object")
	}
	return Normalize(s)
}

// Normalize fills defaults for zero-valued optional fields, sorts faults by
// time (stable) and validates. It is idempotent.
func Normalize(s Spec) (Spec, error) {
	if s.Target == "" {
		return Spec{}, errors.New("target: required")
	}
	if _, ok := raft.LookupTarget(s.Target); !ok || !isPublic(s.Target) {
		return Spec{}, fmt.Errorf("target: unknown target %q (want one of %s)", s.Target, strings.Join(publicCodenames(), ", "))
	}
	d := DefaultSpec(s.Target)
	fillInt64(&s.DurationMs, d.DurationMs)
	fillInt(&s.Nodes, d.Nodes)
	fillInt(&s.Clients, d.Clients)
	fillInt(&s.Keys, d.Keys)
	if s.ReadRatio == 0 {
		s.ReadRatio = d.ReadRatio
	}
	if s.ReadFrom == "" {
		s.ReadFrom = d.ReadFrom
	}
	fillInt64(&s.OpTimeoutMs, d.OpTimeoutMs)
	if s.Network.MinLatencyMs == 0 && s.Network.MaxLatencyMs == 0 {
		s.Network.MinLatencyMs, s.Network.MaxLatencyMs = d.Network.MinLatencyMs, d.Network.MaxLatencyMs
	}
	if s.Faults == nil {
		s.Faults = []Fault{}
	} else {
		s.Faults = append([]Fault(nil), s.Faults...)
		sort.SliceStable(s.Faults, func(i, j int) bool { return s.Faults[i].AtMs < s.Faults[j].AtMs })
	}
	if err := validate(s); err != nil {
		return Spec{}, err
	}
	return s, nil
}

func fillInt(v *int, d int) {
	if *v == 0 {
		*v = d
	}
}

func fillInt64(v *int64, d int64) {
	if *v == 0 {
		*v = d
	}
}

func validate(s Spec) error {
	switch {
	case s.DurationMs < MinDurationMs || s.DurationMs > MaxDurationMs:
		return fmt.Errorf("durationMs: must be between %d and %d, got %d", MinDurationMs, MaxDurationMs, s.DurationMs)
	case s.Nodes != NodeCount:
		return fmt.Errorf("nodes: the cluster always has %d nodes, got %d", NodeCount, s.Nodes)
	case s.Clients < 1 || s.Clients > MaxClients:
		return fmt.Errorf("clients: must be between 1 and %d, got %d", MaxClients, s.Clients)
	case s.Keys < 1 || s.Keys > MaxKeys:
		return fmt.Errorf("keys: must be between 1 and %d, got %d", MaxKeys, s.Keys)
	case s.ReadRatio < 0 || s.ReadRatio > 1:
		return fmt.Errorf("readRatio: must be between 0 and 1, got %g", s.ReadRatio)
	case s.ReadFrom != "leader" && s.ReadFrom != "any":
		return fmt.Errorf("readFrom: must be \"leader\" or \"any\", got %q", s.ReadFrom)
	case s.OpTimeoutMs < MinOpTimeout || s.OpTimeoutMs > MaxOpTimeout:
		return fmt.Errorf("opTimeoutMs: must be between %d and %d, got %d", MinOpTimeout, MaxOpTimeout, s.OpTimeoutMs)
	}
	n := s.Network
	if err := validateNetwork("network", n.MinLatencyMs, n.MaxLatencyMs, n.DropRate, n.DuplicateRate); err != nil {
		return err
	}
	if len(s.Faults) > MaxFaults {
		return fmt.Errorf("faults: at most %d faults, got %d", MaxFaults, len(s.Faults))
	}
	for i, f := range s.Faults {
		if err := validateFault(s, f, &n); err != nil {
			return fmt.Errorf("faults[%d].%w", i, err)
		}
	}
	return nil
}

func validateNetwork(field string, minLat, maxLat int64, drop, dup float64) error {
	switch {
	case minLat < 0 || minLat > MaxLatencyMs:
		return fmt.Errorf("%s.minLatencyMs: must be between 0 and %d, got %d", field, MaxLatencyMs, minLat)
	case maxLat < minLat || maxLat > MaxLatencyMs:
		return fmt.Errorf("%s.maxLatencyMs: must be between minLatencyMs (%d) and %d, got %d", field, minLat, MaxLatencyMs, maxLat)
	case drop < 0 || drop > MaxRate:
		return fmt.Errorf("%s.dropRate: must be between 0 and %g, got %g", field, MaxRate, drop)
	case dup < 0 || dup > MaxRate:
		return fmt.Errorf("%s.duplicateRate: must be between 0 and %g, got %g", field, MaxRate, dup)
	}
	return nil
}
