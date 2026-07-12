package leader_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
)

func TestOperationErrorIsSanitizedAndUnwraps(t *testing.T) {
	cause := errors.New("secret endpoint credential")
	err := leader.NewOperationError("mongo", "campaign", cause)
	if !errors.Is(err, cause) {
		t.Fatal("cause must be preserved")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("leaked cause: %v", err)
	}

	var operationErr *leader.OperationError
	if !errors.As(err, &operationErr) {
		t.Fatalf("error type = %T, want *leader.OperationError", err)
	}
	if operationErr.Backend() != "mongo" || operationErr.Operation() != "campaign" {
		t.Fatalf("metadata = %q/%q", operationErr.Backend(), operationErr.Operation())
	}
}

func TestOperationErrorNilAndZeroValuesAreSafe(t *testing.T) {
	var nilErr *leader.OperationError
	var zero leader.OperationError
	for _, err := range []*leader.OperationError{nilErr, &zero} {
		if err.Error() != "leader operation failed" || err.Unwrap() != nil ||
			err.Backend() != "unknown" || err.Operation() != "unknown" {
			t.Fatalf("unsafe fallback: %#v", err)
		}
	}
}

func TestNewOperationErrorRejectsUnsafeMetadata(t *testing.T) {
	cause := errors.New("cause")
	invalid := []struct {
		backend   string
		operation string
	}{
		{"", "campaign"},
		{" ", "campaign"},
		{" mongo", "campaign"},
		{"mongo", "campaign "},
		{"mongo\n", "campaign"},
		{"mongo", "campaign\x00"},
		{strings.Repeat("m", 33), "campaign"},
		{"mongo", strings.Repeat("c", 33)},
	}
	for _, tt := range invalid {
		if _, ok := leader.NewOperationError(tt.backend, tt.operation, cause).(*leader.OperationError); ok {
			t.Fatalf("NewOperationError(%q, %q) returned OperationError", tt.backend, tt.operation)
		}
	}
	if _, ok := leader.NewOperationError("mongo", "campaign", nil).(*leader.OperationError); ok {
		t.Fatal("nil cause returned OperationError")
	}
}

func TestLeaderSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		leader.ErrAlreadyLeader,
		leader.ErrNotLeader,
		leader.ErrCampaignInProgress,
		leader.ErrCleanupPending,
		leader.ErrInvalidContext,
		leader.ErrCommitUnknown,
	}
	for i, left := range sentinels {
		for j, right := range sentinels {
			if i == j && !errors.Is(left, right) {
				t.Fatalf("sentinel %d must match itself", i)
			}
			if i != j && errors.Is(left, right) {
				t.Fatalf("sentinels %d and %d overlap", i, j)
			}
		}
	}
}
