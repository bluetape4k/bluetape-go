package redisvalue

import (
	"errors"
	"strings"
	"testing"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

func TestCacheErrorIsInspectableAndRedacted(t *testing.T) {
	cause := errors.New("provider 127.0.0.1:6379 failed for raw:key with payload secret-bytes")
	err := newCacheError("get", ReasonProviderFailure, btredis.RedactedKeyID("raw:key"), cause)

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is() = false for cause")
	}
	var cacheErr *CacheError
	if !errors.As(err, &cacheErr) {
		t.Fatalf("errors.As() = false: %v", err)
	}
	if cacheErr.Operation() != "get" || cacheErr.Reason() != ReasonProviderFailure {
		t.Fatalf("accessors = %q/%q", cacheErr.Operation(), cacheErr.Reason())
	}
	message := err.Error()
	for _, secret := range []string{"raw:key", "secret-bytes", "127.0.0.1", "provider 127.0.0.1:6379"} {
		if strings.Contains(message, secret) {
			t.Fatalf("Error() leaked %q: %q", secret, message)
		}
	}
	if !strings.Contains(message, btredis.RedactedKeyID("raw:key")) {
		t.Fatalf("Error() omitted redacted id: %q", message)
	}
}

func TestCacheErrorClearProgress(t *testing.T) {
	want := ClearProgress{ScannedKeys: 7, UnlinkedBatches: 2}
	err := newPartialClearError("clear", want, errors.New("secret provider message"))

	var cacheErr *CacheError
	if !errors.As(err, &cacheErr) {
		t.Fatalf("errors.As() = false: %v", err)
	}
	got, ok := cacheErr.ClearProgress()
	if !ok || got != want {
		t.Fatalf("ClearProgress() = %+v/%v, want %+v/true", got, ok, want)
	}
	if strings.Contains(err.Error(), "secret provider message") {
		t.Fatalf("Error() leaked cause: %q", err.Error())
	}
}

func TestCacheErrorNilReceiverIsSafe(t *testing.T) {
	var err *CacheError
	if err.Error() != "<nil>" || err.Unwrap() != nil || err.Operation() != "" || err.Reason() != "" {
		t.Fatalf("nil accessors are not safe")
	}
	if progress, ok := err.ClearProgress(); ok || progress != (ClearProgress{}) {
		t.Fatalf("nil ClearProgress() = %+v/%v", progress, ok)
	}
}

func TestCacheErrorWithoutKeyUsesLowCardinalityMessage(t *testing.T) {
	err := newCacheError("validate-config", ReasonConfiguration, "", errors.New("sensitive cause"))
	if got := err.Error(); got != "redisvalue validate-config failed: configuration" {
		t.Fatalf("Error() = %q", got)
	}
	if _, ok := err.ClearProgress(); ok {
		t.Fatalf("non-clear error exposed progress")
	}
}
