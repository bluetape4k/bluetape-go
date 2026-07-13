package ratelimit

import (
	"errors"
	"fmt"
	"testing"
)

type operationErrorStub struct{}

func (operationErrorStub) Error() string     { return "rate limiter consume failed" }
func (operationErrorStub) Family() string    { return "rate limiter" }
func (operationErrorStub) Operation() string { return "consume" }
func (operationErrorStub) KeyID() string     { return "key:0123" }

func TestOperationErrorContractSupportsNestedInspection(t *testing.T) {
	var operationErr OperationError = operationErrorStub{}
	wrapped := fmt.Errorf("nested: %w", operationErr)
	var target OperationError
	if !errors.As(wrapped, &target) {
		t.Fatalf("OperationError inspection failed: %v", wrapped)
	}
	if target.Family() != "rate limiter" || target.Operation() != "consume" || target.KeyID() != "key:0123" {
		t.Fatalf("OperationError labels = %q, %q, %q", target.Family(), target.Operation(), target.KeyID())
	}
}

func TestErrCommitUnknownHasProviderNeutralIdentity(t *testing.T) {
	if ErrCommitUnknown == nil || ErrCommitUnknown.Error() != "ratelimit: commit outcome unknown" {
		t.Fatalf("ErrCommitUnknown = %v", ErrCommitUnknown)
	}
	if !errors.Is(fmt.Errorf("nested: %w", ErrCommitUnknown), ErrCommitUnknown) {
		t.Fatal("nested ErrCommitUnknown did not match")
	}
}
