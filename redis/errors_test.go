package btredis

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestNewOpErrorRedactsKeyAndWrapsCause(t *testing.T) {
	err := NewOpError(OpLabels{Family: "redis lock", Operation: "release"}, "raw:key", context.DeadlineExceeded)
	var opErr *OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("NewOpError() = %T, want OpError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("NewOpError() does not wrap context.DeadlineExceeded")
	}
	if contains(err.Error(), "raw:key") {
		t.Fatal("NewOpError() leaked raw key")
	}
	if opErr.KeyID() != RedactedKeyID("raw:key") {
		t.Fatalf("OpError.KeyID() = %q, want redacted raw key", opErr.KeyID())
	}
}

func TestNewOpErrorWithRedactedKeyValidatesID(t *testing.T) {
	id := RedactedKeyID("raw:key")
	err := NewOpErrorWithRedactedKey(OpLabels{Family: "redis lock", Operation: "release"}, id, context.Canceled)
	var opErr *OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("NewOpErrorWithRedactedKey() = %T, want OpError", err)
	}
	if opErr.KeyID() != id {
		t.Fatalf("OpError.KeyID() = %q, want %q", opErr.KeyID(), id)
	}

	for _, value := range []string{"raw:key", "redis-key:abc", "redis-key:ABCDEFabcdefabcdefabcdef"} {
		err := NewOpErrorWithRedactedKey(OpLabels{Family: "redis lock", Operation: "release"}, value, context.Canceled)
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("NewOpErrorWithRedactedKey invalid id error = %v, want ErrInvalidKey", err)
		}
		if contains(err.Error(), value) {
			t.Fatal("invalid redacted key error leaked rejected value")
		}
	}
}

func TestOpErrorZeroValueIsSanitized(t *testing.T) {
	err := (&OpError{}).Error()
	if contains(err, "raw:key") {
		t.Fatal("zero OpError leaked raw key")
	}
	if err == "" {
		t.Fatal("zero OpError returned empty message")
	}
}

func TestOpErrorSanitizesCauseText(t *testing.T) {
	token, err := NewOwnerToken()
	if err != nil {
		t.Fatalf("NewOwnerToken() error = %v", err)
	}
	cause := fmt.Errorf("provider failed for raw:key token=%s", token.RedisValue())
	err = NewOpError(OpLabels{Family: "redis lock", Operation: "release"}, "raw:key", cause)

	if contains(err.Error(), "raw:key") {
		t.Fatal("OpError leaked raw key")
	}
	if contains(err.Error(), token.RedisValue()) {
		t.Fatal("OpError leaked owner token")
	}
	if !errors.Is(err, cause) {
		t.Fatal("OpError does not preserve cause")
	}
}

func TestOpLabelsValidation(t *testing.T) {
	for _, labels := range []OpLabels{
		{},
		{Family: "redis lock", Operation: ""},
		{Family: "redis:lock", Operation: "release"},
		{Family: "redis lock", Operation: "{release}"},
		{Family: "redis lock", Operation: "release\nnext"},
		{Family: "redis lock", Operation: "release\rnext"},
		{Family: "redis lock", Operation: "release\tdebug"},
		{Family: "redis lock", Operation: "release\x1b"},
		{Family: "redis lock", Operation: "  "},
		{Family: "redis lock", Operation: string(make([]byte, 65))},
	} {
		err := NewOpError(labels, "raw:key", context.Canceled)
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("NewOpError(%+v) error = %v, want ErrInvalidKey", labels, err)
		}
	}
}
