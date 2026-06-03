package resilience_test

import (
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestExponentialBackoffCapsDelay(t *testing.T) {
	backoff := resilience.ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2,
		MaxDelay:     250 * time.Millisecond,
	}

	if got := backoff.Delay(1); got != 100*time.Millisecond {
		t.Fatalf("attempt 1 delay = %s, want 100ms", got)
	}
	if got := backoff.Delay(2); got != 200*time.Millisecond {
		t.Fatalf("attempt 2 delay = %s, want 200ms", got)
	}
	if got := backoff.Delay(3); got != 250*time.Millisecond {
		t.Fatalf("attempt 3 delay = %s, want capped 250ms", got)
	}
}

func TestExponentialBackoffAppliesDeterministicJitter(t *testing.T) {
	backoff := resilience.ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2,
		Jitter:       0.5,
		Random: func() float64 {
			return 1
		},
	}

	if got := backoff.Delay(1); got != 150*time.Millisecond {
		t.Fatalf("jittered delay = %s, want 150ms", got)
	}
}

func TestExponentialBackoffClampsRandomJitterInput(t *testing.T) {
	backoff := resilience.ExponentialBackoff{
		InitialDelay: 100 * time.Millisecond,
		Jitter:       0.5,
		Random: func() float64 {
			return 2
		},
	}

	if got := backoff.Delay(1); got != 150*time.Millisecond {
		t.Fatalf("jittered delay = %s, want clamped 150ms", got)
	}
}

func TestExponentialBackoffSaturatesOverflow(t *testing.T) {
	backoff := resilience.ExponentialBackoff{
		InitialDelay: time.Hour,
		Multiplier:   100,
	}

	if got := backoff.Delay(1000); got <= 0 {
		t.Fatalf("overflowed delay = %s, want positive saturated duration", got)
	}
}
