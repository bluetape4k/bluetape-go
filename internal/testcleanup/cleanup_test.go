package testcleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

func TestTerminateUsesBoundedContextEvenWhenParentIsCancelled(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "kept"))
	cancel()

	terminator := &capturingTerminator{}
	if err := Terminate(parent, 50*time.Millisecond, terminator); err != nil {
		t.Fatalf("Terminate failed: %v", err)
	}

	if terminator.ctx == nil {
		t.Fatal("expected terminator context")
	}
	if terminator.err != nil {
		t.Fatalf("cleanup context should ignore parent cancellation, got %v", terminator.err)
	}
	if terminator.value != "kept" {
		t.Fatalf("expected context value to be preserved, got %v", terminator.value)
	}
	if !terminator.hasDeadline {
		t.Fatal("expected cleanup context deadline")
	}
	if time.Until(terminator.deadline) > time.Second {
		t.Fatalf("expected bounded cleanup deadline, got %s", time.Until(terminator.deadline))
	}
}

func TestTerminateReturnsTimeoutWhenContainerDoesNotStop(t *testing.T) {
	terminator := blockingTerminator{}

	err := Terminate(context.Background(), 10*time.Millisecond, terminator)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestTerminateRejectsNilTerminator(t *testing.T) {
	err := Terminate(context.Background(), time.Second, nil)

	if err == nil {
		t.Fatal("expected nil terminator error")
	}
}

func TestRegisterTerminatesWhenSubtestSkips(t *testing.T) {
	terminator := &countingTerminator{}

	t.Run("skipped", func(t *testing.T) {
		Register(context.Background(), t, "redis", terminator)
		t.Skip("exercise cleanup after skip")
	})

	if terminator.calls != 1 {
		t.Fatalf("expected one cleanup call after skip, got %d", terminator.calls)
	}
}

func TestTerminateCanRunRepeatedly(t *testing.T) {
	terminator := &countingTerminator{}

	for range 2 {
		if err := Terminate(context.Background(), time.Second, terminator); err != nil {
			t.Fatalf("Terminate failed: %v", err)
		}
	}

	if terminator.calls != 2 {
		t.Fatalf("expected two cleanup calls, got %d", terminator.calls)
	}
}

type contextKey struct{}

type capturingTerminator struct {
	ctx         context.Context
	value       any
	err         error
	deadline    time.Time
	hasDeadline bool
}

func (t *capturingTerminator) Terminate(ctx context.Context, _ ...testcontainers.TerminateOption) error {
	t.ctx = ctx
	t.value = ctx.Value(contextKey{})
	t.err = ctx.Err()
	t.deadline, t.hasDeadline = ctx.Deadline()
	return nil
}

type blockingTerminator struct{}

func (blockingTerminator) Terminate(ctx context.Context, _ ...testcontainers.TerminateOption) error {
	<-ctx.Done()
	return ctx.Err()
}

type countingTerminator struct {
	calls int
}

func (t *countingTerminator) Terminate(context.Context, ...testcontainers.TerminateOption) error {
	t.calls++
	return nil
}
