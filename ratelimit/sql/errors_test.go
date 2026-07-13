package sqlratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestConfigurationMismatchSupportsNestedInspection(t *testing.T) {
	if !errors.Is(fmt.Errorf("nested: %w", ErrConfigurationMismatch), ErrConfigurationMismatch) {
		t.Fatal("nested ErrConfigurationMismatch did not match")
	}
}

func TestOpErrorIsRedactedAndImplementsRootContract(t *testing.T) {
	const namespace = "secret-namespace"
	const key = "secret-key"
	cause := fmt.Errorf("dsn=postgres://user:password@host/db key=%s", key)
	err := newOperationError("allow", namespace, key, cause)

	var concrete *OpError
	var root ratelimit.OperationError
	if !errors.As(err, &concrete) || !errors.As(fmt.Errorf("nested: %w", err), &root) {
		t.Fatalf("operation error inspection failed: %v", err)
	}
	if !errors.Is(err, cause) || root.Family() != "rate limiter" || root.Operation() != "allow" {
		t.Fatalf("operation error contract = %q %q %v", root.Family(), root.Operation(), err)
	}
	if root.KeyID() == "" || root.KeyID() == key {
		t.Fatalf("unsafe KeyID = %q", root.KeyID())
	}
	for _, marker := range []string{namespace, key, "postgres://", "password", "host"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("error leaked %q: %v", marker, err)
		}
	}
}

func TestOpErrorZeroValueIsSafe(t *testing.T) {
	var nilErr *OpError
	for _, err := range []*OpError{nilErr, {}} {
		if err.Error() == "" || err.Family() == "" || err.Operation() == "" || err.KeyID() == "" {
			t.Fatalf("unsafe zero OpError: %q %q %q %q", err.Error(), err.Family(), err.Operation(), err.KeyID())
		}
		if err.Unwrap() != nil {
			t.Fatal("zero OpError unwrap was non-nil")
		}
	}
}

func TestClassifyAllowErrorSeparatesKnownRollbackFromUnknown(t *testing.T) {
	serverErr := &pgconn.PgError{Code: "42P01", Message: "relation missing"}
	known := classifyOperationError("allow", "namespace", "key", serverErr, nil)
	if errors.Is(known, ratelimit.ErrCommitUnknown) {
		t.Fatalf("known rollback marked unknown: %v", known)
	}
	unknown := classifyOperationError("allow", "namespace", "key", errors.New("transport lost"), context.Canceled)
	if !errors.Is(unknown, ratelimit.ErrCommitUnknown) || !errors.Is(unknown, context.Canceled) {
		t.Fatalf("unknown classification = %v", unknown)
	}
}
