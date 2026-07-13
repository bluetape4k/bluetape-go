package sqlratelimit

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestOptionsRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "blank namespace", opts: Options{Namespace: "  ", RatePerSecond: 1, Burst: 1}},
		{name: "long namespace", opts: Options{Namespace: strings.Repeat("n", 129), RatePerSecond: 1, Burst: 1}},
		{name: "zero rate", opts: Options{Burst: 1}},
		{name: "nan rate", opts: Options{RatePerSecond: math.NaN(), Burst: 1}},
		{name: "infinite rate", opts: Options{RatePerSecond: math.Inf(1), Burst: 1}},
		{name: "tiny rate", opts: Options{RatePerSecond: 0.0000001, Burst: 1}},
		{name: "large rate", opts: Options{RatePerSecond: float64(math.MaxInt64), Burst: 1}},
		{name: "zero burst", opts: Options{RatePerSecond: 1}},
		{name: "large burst", opts: Options{RatePerSecond: 1, Burst: math.MaxInt64}},
		{name: "negative ttl", opts: Options{RatePerSecond: 1, Burst: 1, IdleTTL: -time.Second}},
		{name: "short ttl", opts: Options{RatePerSecond: 1, Burst: 10, IdleTTL: time.Second}},
		{name: "negative key limit", opts: Options{RatePerSecond: 1, Burst: 1, MaxKeyBytes: -1}},
		{name: "large key limit", opts: Options{RatePerSecond: 1, Burst: 1, MaxKeyBytes: 1025}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.opts.normalize(); err == nil {
				t.Fatalf("normalize(%+v) succeeded", tt.opts)
			}
		})
	}
}

func TestOptionsApplyDefaultsAndMicrosecondCeiling(t *testing.T) {
	normalized, err := (Options{RatePerSecond: 10, Burst: 5}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	if string(normalized.namespace) != "default" || normalized.maxKeyBytes != 512 {
		t.Fatalf("defaults = namespace %q key limit %d", normalized.namespace, normalized.maxKeyBytes)
	}
	if normalized.rateMicrosPerSecond != 10_000_000 || normalized.burstMicros != 5_000_000 {
		t.Fatalf("token units = rate %d burst %d", normalized.rateMicrosPerSecond, normalized.burstMicros)
	}
	if normalized.idleTTLMicros != int64(time.Minute/time.Microsecond) {
		t.Fatalf("idle ttl micros = %d", normalized.idleTTLMicros)
	}

	explicit, err := (Options{RatePerSecond: 1_000_000_000, Burst: 1, IdleTTL: 1001 * time.Nanosecond}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	if explicit.idleTTLMicros != 2 {
		t.Fatalf("ceil idle ttl micros = %d, want 2", explicit.idleTTLMicros)
	}
}

func TestOptionsTrimNamespaceButPreserveKeyBytes(t *testing.T) {
	normalized, err := (Options{Namespace: " tenant ", RatePerSecond: 1, Burst: 1, MaxKeyBytes: 8}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	if string(normalized.namespace) != "tenant" {
		t.Fatalf("namespace = %q", normalized.namespace)
	}
	for _, key := range []string{" key ", "a\x00b", string([]byte{0xff, 0xfe})} {
		got, err := normalized.normalizeKey(key)
		if err != nil {
			t.Fatalf("normalizeKey(%q): %v", key, err)
		}
		if got != key {
			t.Fatalf("normalizeKey(%q) = %q", key, got)
		}
	}
	for _, key := range []string{"  ", "123456789"} {
		if _, err := normalized.normalizeKey(key); err == nil {
			t.Fatalf("normalizeKey(%q) succeeded", key)
		}
	}
}

func TestDurationMicrosCeilsPositive(t *testing.T) {
	for _, tt := range []struct {
		value time.Duration
		want  int64
	}{{time.Nanosecond, 1}, {time.Microsecond, 1}, {1001 * time.Nanosecond, 2}} {
		got, err := durationMicrosCeil(tt.value)
		if err != nil || got != tt.want {
			t.Fatalf("durationMicrosCeil(%s) = %d, %v; want %d", tt.value, got, err, tt.want)
		}
	}
}
