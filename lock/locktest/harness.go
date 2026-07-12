package locktest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Config identifies a stable lock owner and lease duration.
type Config struct {
	Key   string
	Owner string
	TTL   time.Duration
}

// ReleaseFunc releases only the ownership acquired by its paired AcquireFunc.
type ReleaseFunc func(context.Context) (bool, error)

// AcquireFunc attempts one immediate lock acquisition.
type AcquireFunc func(context.Context) (ReleaseFunc, error)

// Factory creates an acquire function attached to the provider namespace.
type Factory func(testing.TB, Config) (AcquireFunc, error)

// Operation identifies an acquire or release mutation.
type Operation string

const (
	// OperationAcquire identifies one acquire mutation.
	OperationAcquire Operation = "acquire"
	// OperationRelease identifies one release mutation.
	OperationRelease Operation = "release"
)

// Phase identifies a deterministic command boundary.
type Phase string

const (
	// PhaseBeforeLinearize pauses before mutation dispatch.
	PhaseBeforeLinearize Phase = "before-linearize"
	// PhaseAfterLinearize pauses after the mutation commits.
	PhaseAfterLinearize Phase = "after-linearize"
)

// Gate pauses one operation at a deterministic boundary.
type Gate interface {
	AwaitStarted(context.Context) error
	Resume()
}

// Control supplies mandatory backend observation and fault injection.
type Control interface {
	GateNext(context.Context, Config, Operation, Phase) (Gate, error)
	FailNext(context.Context, Config, Operation, error) error
	Owner(context.Context, Config) (string, error)
	OperationCount(Config, Operation) int64
}

// ErrorClassifier identifies the provider's typed operation errors.
type ErrorClassifier func(error) bool

// Harness supplies a provider factory, control, and neutral error classifier.
type Harness struct {
	New             Factory
	Control         Control
	IsProviderError ErrorClassifier
}

var errInvalidInput = errors.New("locktest: invalid input")

func validateConfig(config Config) error {
	if strings.TrimSpace(config.Key) == "" || strings.TrimSpace(config.Owner) == "" || config.TTL <= 0 {
		return errInvalidInput
	}
	return nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errInvalidInput
	}
	return ctx.Err()
}

func validOperation(operation Operation) bool {
	return operation == OperationAcquire || operation == OperationRelease
}

func validPhase(phase Phase) bool {
	return phase == PhaseBeforeLinearize || phase == PhaseAfterLinearize
}

func validateHarness(h Harness) error {
	if h.New == nil || h.Control == nil || h.IsProviderError == nil {
		return errors.New("locktest: incomplete harness")
	}
	for _, err := range []error{nil, context.Canceled, errInvalidInput, errors.New("raw-cause")} {
		matched, panicValue := classifySafely(h.IsProviderError, err)
		if panicValue != nil || matched {
			return errors.New("locktest: invalid provider error classifier")
		}
	}
	return nil
}

func classifySafely(classifier ErrorClassifier, err error) (matched bool, panicValue any) {
	defer func() { panicValue = recover() }()
	return classifier(err), nil
}

type fixtureError struct {
	operation Operation
	cause     error
}

func (e *fixtureError) Error() string { return fmt.Sprintf("locktest %s failed", e.operation) }

func (e *fixtureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func validatePositiveClassifier(t *testing.T, h Harness) error {
	t.Helper()
	config := Config{Key: fmt.Sprintf("locktest-classifier-%d", runnerID.Add(1)), Owner: "classifier-owner", TTL: time.Second}
	acquire, err := h.New(t, config)
	if err != nil || acquire == nil {
		return errors.New("locktest: classifier probe factory failed")
	}
	cause := errors.New("classifier-probe-cause")
	if err := h.Control.FailNext(context.Background(), config, OperationAcquire, cause); err != nil {
		return errors.New("locktest: classifier probe injection failed")
	}
	release, err := acquire(context.Background())
	if release != nil {
		defer func() { _, _ = release(context.Background()) }()
	}
	if err == nil {
		return errors.New("locktest: classifier probe returned nil error")
	}
	matched, panicValue := classifySafely(h.IsProviderError, err)
	if panicValue != nil || !matched {
		return errors.New("locktest: classifier rejected provider error")
	}
	matched, panicValue = classifySafely(h.IsProviderError, fmt.Errorf("nested: %w", err))
	if panicValue != nil || !matched {
		return errors.New("locktest: classifier rejected nested provider error")
	}
	return nil
}
