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

	googleuuid "github.com/google/uuid"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
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

func TestUUIDV7DefaultTimeUsesCurrentUnixEpochMilliseconds(t *testing.T) {
	before := time.Now().UnixMilli()
	value, err := NewUUIDV7()
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("NewUUIDV7 failed: %v", err)
	}

	got := uuidV7UnixMilli(mustParseGoogleUUID(t, value))
	if got < before || got > after {
		t.Fatalf("UUID v7 millis = %d, want between %d and %d", got, before, after)
	}
}

func TestUUIDV7GeneratorUsesDeterministicClock(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 456789000, time.UTC)
	generator, err := NewUUIDV7Generator(
		WithUUIDTime(func() time.Time { return fixed }),
		WithUUIDReader(bytes.NewReader([]byte("abcdefghijklmnop"))),
	)
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	parsed := mustParseGoogleUUID(t, value)
	if parsed.Version() != 7 {
		t.Fatalf("expected UUID v7 marker, got %d", parsed.Version())
	}
	if parsed.Variant() != googleuuid.RFC4122 {
		t.Fatalf("expected RFC4122 variant, got %v", parsed.Variant())
	}
	if got := uuidV7UnixMilli(parsed); got != fixed.UnixMilli() {
		t.Fatalf("UUID v7 millis = %d, want %d", got, fixed.UnixMilli())
	}
}

func TestUUIDV7SameTickOrdering(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	generator, err := NewUUIDV7Generator(
		WithUUIDTime(func() time.Time { return fixed }),
		WithUUIDReader(bytes.NewReader(bytes.Repeat([]byte{0x11}, 32))),
	)
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
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
		t.Fatalf("expected same-tick UUID v7 values to increase, first=%s second=%s", first, second)
	}
	if uuidV7RandA(mustParseGoogleUUID(t, second)) != uuidV7RandA(mustParseGoogleUUID(t, first))+1 {
		t.Fatalf("expected rand_a logical tick to increment, first=%s second=%s", first, second)
	}
}

func TestUUIDV7RollbackUsesLogicalClock(t *testing.T) {
	times := []time.Time{
		time.UnixMilli(1_800_000_000_000).UTC(),
		time.UnixMilli(1_799_999_999_999).UTC(),
	}
	var index int
	generator, err := NewUUIDV7Generator(
		WithUUIDTime(func() time.Time {
			value := times[min(index, len(times)-1)]
			index++
			return value
		}),
		WithUUIDReader(bytes.NewReader(bytes.Repeat([]byte{0x22}, 32))),
	)
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
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
		t.Fatalf("expected rollback UUID v7 values to increase, first=%s second=%s", first, second)
	}
	if uuidV7LogicalTick(mustParseGoogleUUID(t, second)) != uuidV7LogicalTick(mustParseGoogleUUID(t, first))+1 {
		t.Fatalf("expected rollback to advance the logical UUID v7 tick")
	}
}

func TestUUIDV7CombinesClockAndEntropyReader(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	entropy := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22}
	generator, err := NewUUIDV7Generator(
		WithUUIDTime(func() time.Time { return fixed }),
		WithUUIDReader(bytes.NewReader(entropy)),
	)
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	parsed := mustParseGoogleUUID(t, value)
	if got := uuidV7UnixMilli(parsed); got != fixed.UnixMilli() {
		t.Fatalf("UUID v7 millis = %d, want %d", got, fixed.UnixMilli())
	}
	if parsed[9] != entropy[9] || parsed[15] != entropy[15] {
		t.Fatalf("expected entropy reader to supply rand_b bytes, got %x want source %x", parsed, entropy)
	}
}

func TestUUIDGeneratorRejectsInvalidClockOption(t *testing.T) {
	if _, err := NewUUIDV7Generator(WithUUIDTime(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil clock option error, got %v", err)
	}
}

func TestUUIDV7RejectsOutOfRangeClock(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
	}{
		{name: "before epoch", now: time.UnixMilli(-1).UTC()},
		{name: "outside timestamp range", now: time.Date(10890, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewUUIDV7Generator(WithUUIDTime(func() time.Time { return tt.now }))
			if err != nil {
				t.Fatalf("NewUUIDV7Generator failed: %v", err)
			}
			value, err := generator.NextString()
			if value != "" {
				t.Fatalf("expected no UUID for invalid time, got %q", value)
			}
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("expected invalid options error, got %v", err)
			}
			if errors.Is(err, ErrEntropy) {
				t.Fatalf("expected clock option error without entropy category, got %v", err)
			}
		})
	}
}

func TestUUIDV7RejectsLogicalTickOverflow(t *testing.T) {
	lastValid := time.UnixMilli(uuidV7MaxUnixMilli).UTC()
	generator, err := NewUUIDV7Generator(
		WithUUIDTime(func() time.Time { return lastValid }),
		WithUUIDReader(bytes.NewReader(bytes.Repeat([]byte{0x33}, 4096*16))),
	)
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
	}

	for i := 0; i < 4096; i++ {
		if _, err := generator.NextString(); err != nil {
			t.Fatalf("NextString(%d) failed before logical overflow: %v", i, err)
		}
	}
	value, err := generator.NextString()
	if value != "" {
		t.Fatalf("expected no UUID after logical overflow, got %q", value)
	}
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected logical overflow invalid options error, got %v", err)
	}
	if errors.Is(err, ErrEntropy) {
		t.Fatalf("expected logical overflow without entropy category, got %v", err)
	}
}

func TestUUIDV4IgnoresClockOption(t *testing.T) {
	generator, err := NewUUIDV4Generator(
		WithUUIDTime(func() time.Time { return time.UnixMilli(0) }),
		WithUUIDReader(strings.NewReader("abcdefghijklmnop")),
	)
	if err != nil {
		t.Fatalf("NewUUIDV4Generator failed: %v", err)
	}

	value, err := generator.NextString()
	if err != nil {
		t.Fatalf("NextString failed: %v", err)
	}
	if value[14] != '4' {
		t.Fatalf("expected UUID v4 marker, got %q", value)
	}
}

func TestUUIDV7FixedClockStress(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	generator, err := NewUUIDV7Generator(WithUUIDTime(func() time.Time { return fixed }))
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 128,
		Timeout:       5 * time.Second,
	})

	var (
		mu   sync.Mutex
		seen = make(map[string]struct{}, 256)
	)
	report, err := tester.Run(context.Background(), func(context.Context) error {
		value, err := generator.NextString()
		if err != nil {
			return err
		}
		parsed, err := googleuuid.Parse(value)
		if err != nil {
			return fmt.Errorf("parse UUID v7 %q: %w", value, err)
		}
		if got := uuidV7UnixMilli(parsed); got < fixed.UnixMilli() {
			return fmt.Errorf("UUID v7 millis moved backwards: got %d want >= %d", got, fixed.UnixMilli())
		}
		mu.Lock()
		defer mu.Unlock()
		if _, ok := seen[value]; ok {
			return fmt.Errorf("duplicate UUID v7 %q", value)
		}
		seen[value] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("fixed clock stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != 128 || len(seen) != 128 {
		t.Fatalf("expected 128 unique completions, report=%+v seen=%d", report, len(seen))
	}
}

func TestUUIDV7RollbackClockStress(t *testing.T) {
	base := time.UnixMilli(1_800_000_000_000).UTC()
	times := []time.Time{
		base.Add(2 * time.Millisecond),
		base,
		base.Add(time.Millisecond),
		base.Add(-time.Millisecond),
	}
	var (
		clockMu sync.Mutex
		index   int
	)
	generator, err := NewUUIDV7Generator(WithUUIDTime(func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		value := times[index%len(times)]
		index++
		return value
	}))
	if err != nil {
		t.Fatalf("NewUUIDV7Generator failed: %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 256,
		Timeout:       5 * time.Second,
	})

	var (
		mu    sync.Mutex
		ticks = make(map[int64]struct{}, 256)
	)
	report, err := tester.Run(context.Background(), func(context.Context) error {
		value, err := generator.NextString()
		if err != nil {
			return err
		}
		parsed, err := googleuuid.Parse(value)
		if err != nil {
			return fmt.Errorf("parse UUID v7 %q: %w", value, err)
		}
		tick := uuidV7LogicalTick(parsed)
		mu.Lock()
		defer mu.Unlock()
		if _, ok := ticks[tick]; ok {
			return fmt.Errorf("duplicate UUID v7 logical tick %d from %q", tick, value)
		}
		ticks[tick] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("rollback clock stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != 256 || len(ticks) != 256 {
		t.Fatalf("expected 256 unique logical ticks, report=%+v ticks=%d", report, len(ticks))
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

func mustParseGoogleUUID(tb testing.TB, value string) googleuuid.UUID {
	tb.Helper()
	parsed, err := googleuuid.Parse(value)
	if err != nil {
		tb.Fatalf("Parse(%q) failed: %v", value, err)
	}
	return parsed
}

func uuidV7UnixMilli(uuid googleuuid.UUID) int64 {
	return int64(uuid[0])<<40 |
		int64(uuid[1])<<32 |
		int64(uuid[2])<<24 |
		int64(uuid[3])<<16 |
		int64(uuid[4])<<8 |
		int64(uuid[5])
}

func uuidV7RandA(uuid googleuuid.UUID) int64 {
	return int64(uuid[6]&0x0f)<<8 | int64(uuid[7])
}

func uuidV7LogicalTick(uuid googleuuid.UUID) int64 {
	return uuidV7UnixMilli(uuid)<<12 | uuidV7RandA(uuid)
}
