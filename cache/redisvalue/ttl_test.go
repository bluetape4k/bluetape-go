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

func TestTieredCacheTTLAdjustsKnownWriteLifetime(t *testing.T) {
	started := time.Unix(100, 0)
	tests := []struct {
		name      string
		localTTL  time.Duration
		remoteTTL time.Duration
		now       time.Time
		want      time.Duration
		wantOK    bool
	}{
		{name: "zero remote uses local", localTTL: 30 * time.Second, remoteTTL: 0, now: started.Add(time.Hour), want: 30 * time.Second, wantOK: true},
		{name: "finite remote caps local", localTTL: time.Minute, remoteTTL: 10 * time.Second, now: started, want: 10 * time.Second, wantOK: true},
		{name: "sub-millisecond minimum", localTTL: time.Minute, remoteTTL: time.Nanosecond, now: started, want: time.Millisecond, wantOK: true},
		{name: "fractional truncation", localTTL: time.Minute, remoteTTL: time.Second + 999*time.Microsecond, now: started, want: time.Second, wantOK: true},
		{name: "elapsed subtraction", localTTL: time.Minute, remoteTTL: 10 * time.Second, now: started.Add(3 * time.Second), want: 7 * time.Second, wantOK: true},
		{name: "local cap", localTTL: 2 * time.Second, remoteTTL: 10 * time.Second, now: started.Add(time.Second), want: 2 * time.Second, wantOK: true},
		{name: "non-positive remainder", localTTL: time.Minute, remoteTTL: 10 * time.Second, now: started.Add(10 * time.Second), wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := knownWriteLocalTTL(tt.localTTL, tt.remoteTTL, started, func() time.Time { return tt.now })
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("knownWriteLocalTTL() = %s/%v, want %s/%v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
