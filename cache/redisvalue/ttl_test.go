package redisvalue

import (
	"errors"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

func TestTTLValidationAcceptsZeroAndPositiveSubMillisecond(t *testing.T) {
	for _, ttl := range []time.Duration{0, time.Nanosecond, time.Millisecond, time.Second} {
		if err := validateEntryTTL(ttl); err != nil {
			t.Fatalf("validateEntryTTL(%s) = %v", ttl, err)
		}
	}
	if err := validateEntryTTL(-time.Nanosecond); !errors.Is(err, btredis.ErrInvalidTTL) {
		t.Fatalf("validateEntryTTL(-1ns) = %v", err)
	}
}

func TestTTLNormalizeWirePrecision(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "none", in: 0, want: 0},
		{name: "minimum", in: time.Nanosecond, want: time.Millisecond},
		{name: "sub-millisecond", in: 999 * time.Microsecond, want: time.Millisecond},
		{name: "millisecond exact", in: time.Millisecond, want: time.Millisecond},
		{name: "millisecond truncation", in: time.Second + 999*time.Microsecond, want: time.Second},
		{name: "whole seconds", in: 3 * time.Second, want: 3 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeWireTTL(tt.in); got != tt.want {
				t.Fatalf("normalizeWireTTL(%s) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}
