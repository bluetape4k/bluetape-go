package graphtest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/bluetape4k/bluetape-go/graph"
)

var (
	errInvalidConfig   = errors.New("graphtest: invalid config")
	errInvalidProvider = errors.New("graphtest: invalid provider metadata")
	errInvalidHarness  = errors.New("graphtest: invalid harness")
	reasonCodePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	metadataPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	imagePattern       = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*:[A-Za-z0-9_][A-Za-z0-9._-]{0,127}@sha256:[0-9a-f]{64}$`)
)

func validateHarness(h Harness, cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if err := validateProvider(h.Provider); err != nil {
		return err
	}
	if h.New == nil {
		return errInvalidHarness
	}
	if len(h.Capabilities) != 1 {
		return errInvalidHarness
	}
	support, ok := h.Capabilities[CapabilityTraversal]
	if !ok {
		return errInvalidHarness
	}
	if support.Enabled {
		if support.ReasonCode != "" {
			return errInvalidHarness
		}
	} else if !reasonCodePattern.MatchString(support.ReasonCode) {
		return errInvalidHarness
	}
	return nil
}

func validateConfig(c Config) error {
	if c.StartupTimeout <= 0 || c.StartupTimeout > MaxStartupTimeout ||
		c.CaseTimeout <= 0 || c.CaseTimeout > MaxCaseTimeout ||
		c.CleanupTimeout <= 0 || c.CleanupTimeout > MaxCleanupTimeout ||
		c.CloseTimeout <= 0 || c.CloseTimeout > MaxCloseTimeout ||
		c.MaxVertices < 1 || c.MaxVertices > MaxResultLimit ||
		c.MaxEdges < 1 || c.MaxEdges > MaxResultLimit ||
		c.MaxTraversalResults < 1 || c.MaxTraversalResults > MaxResultLimit {
		return errInvalidConfig
	}
	return nil
}

func validateProvider(p ProviderMetadata) error {
	if !metadataPattern.MatchString(p.Name) || !metadataPattern.MatchString(p.Version) {
		return errInvalidProvider
	}
	if strings.TrimSpace(p.ImageReference) == "" || len(p.ImageReference) > 256 ||
		strings.ContainsAny(p.ImageReference, "\r\n") || strings.IndexFunc(p.ImageReference, unicode.IsControl) >= 0 {
		return errInvalidProvider
	}
	if strings.Contains(p.ImageReference, "://") || strings.Contains(p.ImageReference, "?") || !imagePattern.MatchString(p.ImageReference) {
		return errInvalidProvider
	}
	return nil
}

func validateAdapter(a Adapter, caps Capabilities) error {
	if a.VerifyConnectivity == nil || a.CreateFixture == nil || a.ReadVertices == nil ||
		a.ReadEdges == nil || a.InvalidOperation == nil || a.BlockUntilCanceled == nil ||
		a.CleanupFixture == nil || a.Close == nil || a.IsProviderError == nil {
		return errInvalidHarness
	}
	if caps[CapabilityTraversal].Enabled != (a.Traverse != nil) {
		return errInvalidHarness
	}
	for _, probe := range []error{nil, context.Canceled, context.DeadlineExceeded, graph.ErrInvalidVertex, errors.New("raw cause")} {
		matched, panicked := classify(a.IsProviderError, probe)
		if panicked || matched {
			return errInvalidHarness
		}
	}
	return nil
}

func classify(fn func(error) bool, err error) (matched bool, panicked bool) {
	defer func() { panicked = recover() != nil }()
	return fn(err), false
}

func category(phase string, err error) error {
	if err == nil {
		return nil
	}
	return &phaseError{phase: phase, cause: err}
}

func imageDigest(reference string) string {
	_, digest, _ := strings.Cut(reference, "@")
	return digest
}

type phaseError struct {
	phase string
	cause error
}

func (e *phaseError) Error() string {
	return fmt.Sprintf("graphtest: %s failed", e.phase)
}

func (e *phaseError) Unwrap() error {
	return e.cause
}
