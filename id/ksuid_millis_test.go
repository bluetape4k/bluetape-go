package id

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestKSUIDMillisGeneratorUsesKotlinCompatibleTimeAndEntropy(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 987_000_000, time.UTC)
	payload := []byte("abcdefghijkl")
	generator, err := NewKSUIDMillisGenerator(
		WithKSUIDMillisTime(func() time.Time { return fixed }),
		WithKSUIDMillisEntropy(bytes.NewReader(payload)),
	)
	if err != nil {
		t.Fatalf("NewKSUIDMillisGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	if value != "AAAAY56PSOdYiNGZlZ2ZolmarxG" {
		t.Fatalf("expected Kotlin-compatible KSUID millis vector, got %q", value)
	}
	if len(value) != 27 {
		t.Fatalf("expected canonical KSUID millis length, got %q", value)
	}

	canonical, err := ParseKSUIDMillis(value)
	if err != nil {
		t.Fatalf("ParseKSUIDMillis failed: %v", err)
	}
	if canonical != value {
		t.Fatalf("expected canonical round trip, got %q", canonical)
	}
	gotTime, err := KSUIDMillisTime(value)
	if err != nil {
		t.Fatalf("KSUIDMillisTime failed: %v", err)
	}
	if !gotTime.Equal(fixed) {
		t.Fatalf("expected timestamp %v, got %v", fixed, gotTime)
	}
}

func TestKSUIDMillisGeneratorTruncatesEncodedBitstreamLikeKotlin(t *testing.T) {
	fixed := time.UnixMilli(ksuidMillisEpoch + 1).UTC()
	payload := bytes.Repeat([]byte{0xff}, ksuidMillisPayloadLen)
	generator, err := NewKSUIDMillisGenerator(
		WithKSUIDMillisTime(func() time.Time { return fixed }),
		WithKSUIDMillisEntropy(bytes.NewReader(payload)),
	)
	if err != nil {
		t.Fatalf("NewKSUIDMillisGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}

	var raw [ksuidMillisTotalBytes]byte
	raw[ksuidMillisTimestampLen-1] = 1
	copy(raw[ksuidMillisTimestampLen:], payload)
	full := encodeKSUIDMillisBase62(raw[:])
	if len(full) <= ksuidMillisEncodedLen {
		t.Fatalf("test vector must exercise truncation, full=%q", full)
	}
	if value != full[:ksuidMillisEncodedLen] {
		t.Fatalf("expected Kotlin substring-compatible prefix %q, got %q", full[:ksuidMillisEncodedLen], value)
	}
	if len(value) != ksuidMillisEncodedLen {
		t.Fatalf("expected truncated length %d, got %d", ksuidMillisEncodedLen, len(value))
	}
}

func TestKSUIDMillisParsesKnownZeroVector(t *testing.T) {
	value := "AAAAAAAAAAAAAAAAAAAAAAAAAAA"
	gotTime, err := KSUIDMillisTime(value)
	if err != nil {
		t.Fatalf("KSUIDMillisTime failed: %v", err)
	}
	want := time.UnixMilli(ksuidMillisEpoch).UTC()
	if !gotTime.Equal(want) {
		t.Fatalf("expected epoch timestamp %v, got %v", want, gotTime)
	}
	canonical, err := ParseKSUIDMillis(value)
	if err != nil {
		t.Fatalf("ParseKSUIDMillis failed: %v", err)
	}
	if canonical != value {
		t.Fatalf("expected canonical zero vector, got %q", canonical)
	}
}

func TestKSUIDMillisBase62RoundTripPreservesExpectedByteLength(t *testing.T) {
	for _, size := range []int{1, 2, 3, 4, 5, 7, 8, 9, 12, 20, 21, 32} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			input := make([]byte, size)
			for i := range input {
				input[i] = byte(i + 1)
			}
			encoded := encodeKSUIDMillisBase62(input)
			decoded, err := decodeKSUIDMillisBase62(encoded, size)
			if err != nil {
				t.Fatalf("decodeKSUIDMillisBase62(%q) failed: %v", encoded, err)
			}
			if !bytes.Equal(decoded, input) {
				t.Fatalf("round trip = %x, want %x", decoded, input)
			}
		})
	}
}

func TestKSUIDMillisBase62DecodeTrimsOrPadsExpectedLength(t *testing.T) {
	input := []byte("abcdefghij")
	encoded := encodeKSUIDMillisBase62(input)

	trimmed, err := decodeKSUIDMillisBase62(encoded, 5)
	if err != nil {
		t.Fatalf("trim decode failed: %v", err)
	}
	if len(trimmed) != 5 || !bytes.Equal(trimmed, input[:5]) {
		t.Fatalf("trimmed = %x, want %x", trimmed, input[:5])
	}

	padded, err := decodeKSUIDMillisBase62(encoded, 12)
	if err != nil {
		t.Fatalf("pad decode failed: %v", err)
	}
	if len(padded) != 12 || !bytes.Equal(padded[:len(input)], input) {
		t.Fatalf("padded = %x, want prefix %x", padded, input)
	}
	if !bytes.Equal(padded[len(input):], []byte{0, 0}) {
		t.Fatalf("expected zero padding, got %x", padded[len(input):])
	}
}

func TestKSUIDMillisDoesNotClaimLexicalTimestampSorting(t *testing.T) {
	payload := []byte("abcdefghijkl")
	firstTime := time.UnixMilli(ksuidMillisEpoch + 12).UTC()
	secondTime := firstTime.Add(time.Millisecond)

	first, err := deterministicKSUIDMillis(firstTime, payload)
	if err != nil {
		t.Fatalf("first KSUID millis failed: %v", err)
	}
	second, err := deterministicKSUIDMillis(secondTime, payload)
	if err != nil {
		t.Fatalf("second KSUID millis failed: %v", err)
	}
	if first < second {
		t.Fatalf("test vector should prove Kotlin-compatible millis encoding is not generally lexically sortable, first=%s second=%s", first, second)
	}
}

func TestKSUIDMillisRejectsInvalidInput(t *testing.T) {
	tests := []string{
		"",
		"short",
		"AAAAAAAAAAAAAAAAAAAAAAAAAA!",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseKSUIDMillis(value); !errors.Is(err, ErrInvalidID) {
				t.Fatalf("expected invalid KSUID millis parse error, got %v", err)
			}
			if _, err := KSUIDMillisTime(value); !errors.Is(err, ErrInvalidID) {
				t.Fatalf("expected invalid KSUID millis time error, got %v", err)
			}
		})
	}
}

func TestKSUIDMillisGeneratorRejectsInvalidOptions(t *testing.T) {
	if _, err := NewKSUIDMillisGenerator(nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil option error, got %v", err)
	}
	if _, err := NewKSUIDMillisGenerator(WithKSUIDMillisEntropy(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil entropy error, got %v", err)
	}
	if _, err := NewKSUIDMillisGenerator(WithKSUIDMillisTime(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil clock error, got %v", err)
	}
}

func TestKSUIDMillisGeneratorWrapsEntropyFailure(t *testing.T) {
	expected := errors.New("entropy down")
	generator, err := NewKSUIDMillisGenerator(WithKSUIDMillisEntropy(errorReader{err: expected}))
	if err != nil {
		t.Fatalf("NewKSUIDMillisGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no KSUID millis on entropy failure, got %q", value)
	}
	if !errors.Is(err, ErrEntropy) || !errors.Is(err, expected) {
		t.Fatalf("expected wrapped entropy error, got %v", err)
	}
}

func TestKSUIDMillisGeneratorWrapsShortEntropy(t *testing.T) {
	generator, err := NewKSUIDMillisGenerator(WithKSUIDMillisEntropy(strings.NewReader("short")))
	if err != nil {
		t.Fatalf("NewKSUIDMillisGenerator failed: %v", err)
	}

	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no KSUID millis on short entropy, got %q", value)
	}
	if !errors.Is(err, ErrEntropy) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected short entropy error, got %v", err)
	}
}

func TestKSUIDMillisCrossFamilyParsingIsExplicitlyNonDisambiguating(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 987_000_000, time.UTC)
	millis, err := deterministicKSUIDMillis(fixed, []byte("abcdefghijkl"))
	if err != nil {
		t.Fatalf("KSUID millis failed: %v", err)
	}
	seconds, err := deterministicKSUID(fixed, []byte("abcdefghijklmnop"))
	if err != nil {
		t.Fatalf("KSUID seconds failed: %v", err)
	}

	if _, err := ParseKSUID(millis); err != nil {
		t.Fatalf("standard KSUID parser currently accepts millis-shaped strings: %v", err)
	}
	if _, err := ParseKSUIDMillis(seconds); err != nil {
		t.Fatalf("millis parser currently accepts Segment-shaped strings: %v", err)
	}
	secondsTime, err := KSUIDTime(millis)
	if err != nil {
		t.Fatalf("KSUIDTime on millis-shaped string failed: %v", err)
	}
	millisTime, err := KSUIDMillisTime(seconds)
	if err != nil {
		t.Fatalf("KSUIDMillisTime on Segment-shaped string failed: %v", err)
	}
	if secondsTime.Equal(fixed.Truncate(time.Second)) {
		t.Fatalf("standard parser unexpectedly recovered millis timestamp %v", secondsTime)
	}
	if millisTime.Equal(fixed) {
		t.Fatalf("millis parser unexpectedly recovered Segment timestamp %v", millisTime)
	}
}

func TestKSUIDMillisGeneratorStress(t *testing.T) {
	generator, err := NewKSUIDMillisGenerator()
	if err != nil {
		t.Fatalf("NewKSUIDMillisGenerator failed: %v", err)
	}
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       16,
		RoundsPerTask: 512,
		Timeout:       10 * time.Second,
	})

	var (
		mu   sync.Mutex
		seen = make(map[string]struct{}, 512)
	)
	report, err := tester.Run(context.Background(), func(context.Context) error {
		value, err := generator.NextString()
		if err != nil {
			return err
		}
		if _, err := ParseKSUIDMillis(value); err != nil {
			return fmt.Errorf("ParseKSUIDMillis(%q): %w", value, err)
		}
		mu.Lock()
		defer mu.Unlock()
		if _, ok := seen[value]; ok {
			return fmt.Errorf("duplicate KSUID millis %q", value)
		}
		seen[value] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("KSUID millis stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != 512 || len(seen) != 512 {
		t.Fatalf("expected 512 unique completions, report=%+v seen=%d", report, len(seen))
	}
}

func deterministicKSUIDMillis(now time.Time, payload []byte) (string, error) {
	generator, err := NewKSUIDMillisGenerator(
		WithKSUIDMillisTime(func() time.Time { return now }),
		WithKSUIDMillisEntropy(bytes.NewReader(payload)),
	)
	if err != nil {
		return "", err
	}
	return generator.NextString()
}
