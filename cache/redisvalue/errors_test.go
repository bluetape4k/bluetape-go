package redisvalue

import (
	"errors"
	"fmt"
	"log/slog"
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

func TestCacheErrorFormattingAndStructuredLoggingStayRedacted(t *testing.T) {
	secrets := []string{"raw:key", "secret-bytes", "127.0.0.1", "provider-password"}
	keyID := btredis.RedactedKeyID("raw:key")
	tests := map[string]error{
		"provider": newCacheError(
			"get",
			ReasonProviderFailure,
			keyID,
			errors.New("provider 127.0.0.1 failed for raw:key with provider-password"),
		),
		"serializer": newCacheError(
			"get",
			ReasonInvalidPayload,
			keyID,
			errors.New("secret-bytes from raw:key"),
		),
		"partial-clear": newPartialClearError(
			"clear",
			ClearProgress{ScannedKeys: 2, UnlinkedBatches: 1},
			errors.New("provider-password at 127.0.0.1"),
		),
		"joined-cleanup": newCacheError(
			"set",
			ReasonLocalBlocked,
			keyID,
			errors.Join(
				errors.New("provider 127.0.0.1 failed for raw:key"),
				errors.New("cleanup leaked secret-bytes and provider-password"),
			),
		),
	}

	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			for _, formatted := range []string{
				fmt.Sprintf("%v", err),
				fmt.Sprintf("%+v", err),
				fmt.Sprintf("%#v", err),
			} {
				assertNoSecrets(t, formatted, secrets)
			}
			valuer, ok := err.(slog.LogValuer)
			if !ok {
				t.Fatal("CacheError does not implement slog.LogValuer")
			}
			assertNoSecrets(t, valuer.LogValue().String(), secrets)
		})
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

func TestCacheErrorClearProgressSurvivesOuterBlockedCleanup(t *testing.T) {
	want := ClearProgress{ScannedKeys: 7, UnlinkedBatches: 2}
	remoteErr := newPartialClearError("clear", want, errors.New("provider failure"))
	err := newCacheError(
		"clear",
		ReasonLocalBlocked,
		"",
		errors.Join(remoteErr, errors.New("local cleanup failure")),
	)

	got, ok := err.ClearProgress()
	if !ok || got != want {
		t.Fatalf("ClearProgress() = %+v/%v, want %+v/true", got, ok, want)
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

func assertNoSecrets(t *testing.T, text string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("formatted error leaked %q: %q", secret, text)
		}
	}
}
