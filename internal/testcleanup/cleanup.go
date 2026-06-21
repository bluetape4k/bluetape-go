// Package testcleanup provides bounded cleanup helpers for integration tests.
package testcleanup

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// DefaultTerminateTimeout bounds container termination during test cleanup.
const DefaultTerminateTimeout = 30 * time.Second

// Terminator is the subset of Testcontainers containers used during cleanup.
type Terminator interface {
	Terminate(context.Context, ...testcontainers.TerminateOption) error
}

// Terminate stops a container with a bounded context that ignores parent
// cancellation while preserving parent context values.
func Terminate(parent context.Context, timeout time.Duration, terminator Terminator) error {
	if terminator == nil {
		return fmt.Errorf("terminator must not be nil")
	}
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = DefaultTerminateTimeout
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
	defer cancel()
	return terminator.Terminate(ctx)
}

// Register wires bounded Testcontainers cleanup to tb.Cleanup.
func Register(parent context.Context, tb testing.TB, name string, terminator Terminator) {
	tb.Helper()
	tb.Cleanup(func() {
		tb.Helper()
		if err := Terminate(parent, DefaultTerminateTimeout, terminator); err != nil {
			tb.Fatalf("terminate %s container: %v", name, err)
		}
	})
}
