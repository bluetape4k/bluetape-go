package id

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUUIDV4GeneratorUsesDeterministicReader(t *testing.T) {
	generator, err := NewUUIDV4Generator(WithUUIDReader(strings.NewReader("abcdefghijklmnop")))
	if err != nil {
		t.Fatalf("NewUUIDV4Generator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	if len(value) != 36 {
		t.Fatalf("expected canonical UUID length, got %q", value)
	}
	if value[14] != '4' {
		t.Fatalf("expected UUID v4 marker, got %q", value)
	}
	if _, err := ParseUUID(value); err != nil {
		t.Fatalf("ParseUUID failed: %v", err)
	}
}

func TestUUIDV7GeneratorUsesDeterministicReader(t *testing.T) {
	generator, err := NewUUIDV7Generator(WithUUIDReader(strings.NewReader("abcdefghijklmnop")))
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	if value[14] != '7' {
		t.Fatalf("expected UUID v7 marker, got %q", value)
	}
}

func TestUUIDGeneratorWrapsEntropyFailure(t *testing.T) {
	expected := errors.New("entropy down")
	generator, err := NewUUIDV4Generator(WithUUIDReader(errorReader{err: expected}))
	if err != nil {
		t.Fatalf("NewUUIDV4Generator failed: %v", err)
	}

	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no UUID on entropy failure, got %q", value)
	}
	if !errors.Is(err, ErrEntropy) || !errors.Is(err, expected) {
		t.Fatalf("expected wrapped entropy error, got %v", err)
	}
}

func TestUUIDGeneratorRejectsInvalidOptions(t *testing.T) {
	if _, err := NewUUIDV4Generator(nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil option error, got %v", err)
	}
	if _, err := NewUUIDV4Generator(WithUUIDReader(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil reader error, got %v", err)
	}
}

func TestParseUUIDRejectsInvalidInput(t *testing.T) {
	if _, err := ParseUUID("not-a-uuid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected invalid UUID error, got %v", err)
	}
}

func TestParseUUIDRejectsNonCanonicalInput(t *testing.T) {
	tests := []string{
		"550e8400e29b41d4a716446655440000",
		"urn:uuid:550e8400-e29b-41d4-a716-446655440000",
		"{550e8400-e29b-41d4-a716-446655440000}",
		"550E8400-E29B-41D4-A716-446655440000",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseUUID(value); !errors.Is(err, ErrInvalidID) {
				t.Fatalf("expected invalid UUID error, got %v", err)
			}
		})
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

var _ io.Reader = errorReader{}
