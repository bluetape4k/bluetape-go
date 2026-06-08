package id

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestULIDGeneratorUsesDeterministicEntropyAndTime(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 4_000_000, time.UTC)
	generator, err := NewULIDGenerator(
		WithULIDTime(func() time.Time { return fixed }),
		WithULIDEntropy(strings.NewReader("abcdefghij")),
	)
	if err != nil {
		t.Fatalf("NewULIDGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	if len(value) != 26 {
		t.Fatalf("expected canonical ULID length, got %q", value)
	}
	canonical, err := ParseULID(value)
	if err != nil {
		t.Fatalf("ParseULID failed: %v", err)
	}
	if canonical != value {
		t.Fatalf("expected canonical round trip, got %q", canonical)
	}
	gotTime, err := ULIDTime(value)
	if err != nil {
		t.Fatalf("ULIDTime failed: %v", err)
	}
	if !gotTime.Equal(fixed.Truncate(time.Millisecond)) {
		t.Fatalf("expected timestamp %v, got %v", fixed.Truncate(time.Millisecond), gotTime)
	}
}

func TestMonotonicULIDGeneratorOrdersSameMillisecond(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	entropy := strings.NewReader("abcdefghij\x01\x00\x00\x00\x01\x00\x00\x00")
	generator, err := NewMonotonicULIDGenerator(
		WithULIDTime(func() time.Time { return fixed }),
		WithULIDEntropy(entropy),
	)
	if err != nil {
		t.Fatalf("NewMonotonicULIDGenerator failed: %v", err)
	}

	first, err := generator.NextString()
	if err != nil {
		t.Fatalf("first NextString failed: %v", err)
	}
	second, err := generator.NextString()
	if err != nil {
		t.Fatalf("second NextString failed: %v", err)
	}
	if first >= second {
		t.Fatalf("expected monotonic order, got first=%s second=%s", first, second)
	}
}

func TestULIDGeneratorWrapsEntropyFailure(t *testing.T) {
	expected := errors.New("entropy down")
	generator, err := NewULIDGenerator(WithULIDEntropy(errorReader{err: expected}))
	if err != nil {
		t.Fatalf("NewULIDGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no ULID on entropy failure, got %q", value)
	}
	if !errors.Is(err, ErrEntropy) || !errors.Is(err, expected) {
		t.Fatalf("expected entropy error, got %v", err)
	}
}

func TestULIDGeneratorRejectsInvalidOptions(t *testing.T) {
	if _, err := NewULIDGenerator(nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil option error, got %v", err)
	}
	if _, err := NewULIDGenerator(WithULIDEntropy(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil entropy error, got %v", err)
	}
	if _, err := NewULIDGenerator(WithULIDTime(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil clock error, got %v", err)
	}
}

func TestParseULIDRejectsInvalidInput(t *testing.T) {
	if _, err := ParseULID("invalid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected invalid ULID error, got %v", err)
	}
}
