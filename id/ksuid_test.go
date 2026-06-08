package id

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	segmentioksuid "github.com/segmentio/ksuid"
)

func TestKSUIDGeneratesAndParses(t *testing.T) {
	value, err := NewKSUID()
	if err != nil {
		t.Fatalf("NewKSUID failed: %v", err)
	}
	if len(value) != 27 {
		t.Fatalf("expected canonical KSUID length, got %q", value)
	}
	canonical, err := ParseKSUID(value)
	if err != nil {
		t.Fatalf("ParseKSUID failed: %v", err)
	}
	if canonical != value {
		t.Fatalf("expected canonical round trip, got %q", canonical)
	}
	gotTime, err := KSUIDTime(value)
	if err != nil {
		t.Fatalf("KSUIDTime failed: %v", err)
	}
	if gotTime.IsZero() || gotTime.Location() != time.UTC {
		t.Fatalf("expected non-zero UTC time, got %v location=%v", gotTime, gotTime.Location())
	}
}

func TestKSUIDGeneratorUsesInjectedTimeAndEntropy(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 987_000_000, time.UTC)
	payload := []byte("abcdefghijklmnop")
	generator, err := NewKSUIDGenerator(
		WithKSUIDTime(func() time.Time { return fixed }),
		WithKSUIDEntropy(bytes.NewReader(payload)),
	)
	if err != nil {
		t.Fatalf("NewKSUIDGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	if len(value) != 27 {
		t.Fatalf("expected canonical KSUID length, got %q", value)
	}
	if value != "3Epfe5fwjidB4aKO8WJm6x2QaIK" {
		t.Fatalf("expected deterministic canonical KSUID, got %q", value)
	}

	parsed, err := segmentioksuid.Parse(value)
	if err != nil {
		t.Fatalf("segmentio Parse failed: %v", err)
	}
	if !parsed.Time().UTC().Equal(fixed.Truncate(time.Second)) {
		t.Fatalf("expected timestamp %v, got %v", fixed.Truncate(time.Second), parsed.Time().UTC())
	}
	if !bytes.Equal(parsed.Payload(), payload) {
		t.Fatalf("expected payload %x, got %x", payload, parsed.Payload())
	}
	if parsed.String() != value {
		t.Fatalf("expected canonical Segment KSUID string, got %q", parsed.String())
	}
}

func TestKSUIDSortsByTimestamp(t *testing.T) {
	payload := []byte("abcdefghijklmnop")
	firstTime := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	secondTime := firstTime.Add(time.Second)

	first, err := deterministicKSUID(firstTime, payload)
	if err != nil {
		t.Fatalf("first KSUID failed: %v", err)
	}
	second, err := deterministicKSUID(secondTime, payload)
	if err != nil {
		t.Fatalf("second KSUID failed: %v", err)
	}
	if first >= second {
		t.Fatalf("expected timestamp lexical order, first=%s second=%s", first, second)
	}
}

func TestKSUIDRejectsInvalidInput(t *testing.T) {
	tests := []string{
		"",
		"short",
		"0000000000000000000000000000",
		"00000000000000000000000000!",
		"aWgEPTl1tmebfsQzFP4bxwgy80W",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseKSUID(value); !errors.Is(err, ErrInvalidID) {
				t.Fatalf("expected invalid KSUID parse error, got %v", err)
			}
			if _, err := KSUIDTime(value); !errors.Is(err, ErrInvalidID) {
				t.Fatalf("expected invalid KSUID time error, got %v", err)
			}
		})
	}
}

func TestKSUIDGeneratorRejectsInvalidOptions(t *testing.T) {
	if _, err := NewKSUIDGenerator(nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil option error, got %v", err)
	}
	if _, err := NewKSUIDGenerator(WithKSUIDEntropy(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil entropy error, got %v", err)
	}
	if _, err := NewKSUIDGenerator(WithKSUIDTime(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil clock error, got %v", err)
	}
}

func TestKSUIDGeneratorRejectsOutOfRangeTime(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
	}{
		{name: "before epoch", now: time.Unix(ksuidEpochStamp-1, 0).UTC()},
		{name: "after max", now: time.Unix(ksuidEpochStamp+ksuidMaxTimestampOffset+1, 0).UTC()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewKSUIDGenerator(
				WithKSUIDTime(func() time.Time { return tt.now }),
				WithKSUIDEntropy(bytes.NewReader([]byte("abcdefghijklmnop"))),
			)
			if err != nil {
				t.Fatalf("NewKSUIDGenerator failed: %v", err)
			}

			value, err := generator.NextString()
			if value != "" {
				t.Fatalf("expected no KSUID for out-of-range time, got %q", value)
			}
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("expected invalid time option error, got %v", err)
			}
		})
	}
}

func TestKSUIDGeneratorWrapsEntropyFailure(t *testing.T) {
	expected := errors.New("entropy down")
	generator, err := NewKSUIDGenerator(WithKSUIDEntropy(errorReader{err: expected}))
	if err != nil {
		t.Fatalf("NewKSUIDGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no KSUID on entropy failure, got %q", value)
	}
	if !errors.Is(err, ErrEntropy) || !errors.Is(err, expected) {
		t.Fatalf("expected wrapped entropy error, got %v", err)
	}
}

func TestKSUIDGeneratorWrapsShortEntropy(t *testing.T) {
	generator, err := NewKSUIDGenerator(WithKSUIDEntropy(strings.NewReader("short")))
	if err != nil {
		t.Fatalf("NewKSUIDGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no KSUID on short entropy, got %q", value)
	}
	if !errors.Is(err, ErrEntropy) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected short entropy error, got %v", err)
	}
}

func deterministicKSUID(now time.Time, payload []byte) (string, error) {
	generator, err := NewKSUIDGenerator(
		WithKSUIDTime(func() time.Time { return now }),
		WithKSUIDEntropy(bytes.NewReader(payload)),
	)
	if err != nil {
		return "", err
	}
	return generator.NextString()
}
