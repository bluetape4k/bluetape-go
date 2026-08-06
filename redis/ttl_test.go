package btredis

import (
	"errors"
	"testing"
	"time"
)

func TestTTLMillis(t *testing.T) {
	got, err := TTLMillis("lease ttl", time.Millisecond)
	if err != nil {
		t.Fatalf("TTLMillis() error = %v", err)
	}
	if got != 1 {
		t.Fatalf("TTLMillis() = %d, want 1", got)
	}
}

func TestValidateTTLRejectsInvalidDurations(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Second, time.Nanosecond} {
		if err := ValidateTTL("lease ttl", ttl); !errors.Is(err, ErrInvalidTTL) {
			t.Fatalf("ValidateTTL(%s) error = %v, want ErrInvalidTTL", ttl, err)
		}
	}
}

func TestValidateTTLIncludesSafeName(t *testing.T) {
	err := ValidateTTL("lease ttl", 0)
	if err == nil {
		t.Fatal("ValidateTTL() error = nil, want ErrInvalidTTL")
	}
	if !contains(err.Error(), "lease ttl") {
		t.Fatalf("ValidateTTL() error = %q, want safe ttl name", err)
	}
	err = ValidateTTL("raw:key", 0)
	if contains(err.Error(), "raw:key") {
		t.Fatal("ValidateTTL() leaked unsafe ttl name")
	}
}
