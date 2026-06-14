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
