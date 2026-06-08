package id

import (
	"errors"
	"testing"
	"time"
)

func TestSnowflakeGeneratesAndDecodes(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := epoch.Add(123 * time.Millisecond)
	generator, err := NewSnowflakeGenerator(
		42,
		WithSnowflakeEpoch(epoch),
		WithSnowflakeTime(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("NewSnowflakeGenerator failed: %v", err)
	}

	value, err := generator.NextInt64()
	if err != nil {
		t.Fatalf("NextInt64 failed: %v", err)
	}
	parts, err := DecodeSnowflake(value, WithSnowflakeEpoch(epoch))
	if err != nil {
		t.Fatalf("DecodeSnowflake failed: %v", err)
	}
	if !parts.Time.Equal(now) || parts.MachineID != 42 || parts.Sequence != 0 {
		t.Fatalf("unexpected parts: %+v", parts)
	}
}

func TestSnowflakeRejectsInvalidMachineID(t *testing.T) {
	if _, err := NewSnowflakeGenerator(-1); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected invalid machine ID error, got %v", err)
	}
	if _, err := NewSnowflakeGenerator(snowflakeMaxMachineID + 1); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected invalid machine ID error, got %v", err)
	}
}

func TestSnowflakeRejectsInvalidOptions(t *testing.T) {
	if _, err := NewSnowflakeGenerator(1, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil option error, got %v", err)
	}
	if _, err := NewSnowflakeGenerator(1, WithSnowflakeEpoch(time.Time{})); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected zero epoch error, got %v", err)
	}
	if _, err := NewSnowflakeGenerator(1, WithSnowflakeTime(nil)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected nil clock error, got %v", err)
	}
}

func TestSnowflakeRejectsTimestampOverflow(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := epoch.Add(time.Duration(snowflakeMaxTimestamp+1) * time.Millisecond)
	generator, err := NewSnowflakeGenerator(1, WithSnowflakeEpoch(epoch), WithSnowflakeTime(func() time.Time {
		return now
	}))
	if err != nil {
		t.Fatalf("NewSnowflakeGenerator failed: %v", err)
	}

	value, err := generator.NextInt64()
	if value != 0 {
		t.Fatalf("expected no Snowflake on timestamp overflow, got %d", value)
	}
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("expected timestamp overflow error, got %v", err)
	}
}

func TestSnowflakeReportsClockRollback(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	times := []time.Time{
		epoch.Add(10 * time.Millisecond),
		epoch.Add(9 * time.Millisecond),
	}
	index := 0
	generator, err := NewSnowflakeGenerator(1, WithSnowflakeEpoch(epoch), WithSnowflakeTime(func() time.Time {
		value := times[index]
		index++
		return value
	}))
	if err != nil {
		t.Fatalf("NewSnowflakeGenerator failed: %v", err)
	}
	if _, err := generator.NextInt64(); err != nil {
		t.Fatalf("first NextInt64 failed: %v", err)
	}
	if _, err := generator.NextInt64(); !errors.Is(err, ErrClockRollback) {
		t.Fatalf("expected clock rollback error, got %v", err)
	}
}

func TestSnowflakeReportsSequenceExhaustion(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := epoch.Add(time.Millisecond)
	generator, err := NewSnowflakeGenerator(1, WithSnowflakeEpoch(epoch), WithSnowflakeTime(func() time.Time {
		return now
	}))
	if err != nil {
		t.Fatalf("NewSnowflakeGenerator failed: %v", err)
	}
	for i := int64(0); i <= snowflakeMaxSequence; i++ {
		if _, err := generator.NextInt64(); err != nil {
			t.Fatalf("NextInt64 #%d failed: %v", i, err)
		}
	}
	if _, err := generator.NextInt64(); !errors.Is(err, ErrSequenceExhausted) {
		t.Fatalf("expected sequence exhausted error, got %v", err)
	}
}

func TestParseSnowflakeRejectsInvalidInput(t *testing.T) {
	if _, err := ParseSnowflake("-1"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected invalid negative snowflake, got %v", err)
	}
	if _, err := ParseSnowflake("bad"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected invalid snowflake string, got %v", err)
	}
	if _, err := DecodeSnowflake(-1); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected invalid snowflake decode, got %v", err)
	}
}
