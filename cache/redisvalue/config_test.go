package redisvalue

import (
	"errors"
	"strings"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

func TestDefaultConfigReturnsIndependentValidValues(t *testing.T) {
	first := DefaultConfig()
	second := DefaultConfig()

	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
	if first.Value.RemoteTTL != time.Hour || first.Value.MaxValueBytes != 1<<20 || first.Value.ClearBatchSize != 100 {
		t.Fatalf("value defaults = %+v", first.Value)
	}
	if first.Tiered.LocalTTL != 30*time.Minute || first.Tiered.InvalidationWaitTimeout != 30*time.Second || first.Tiered.LocalCleanupTimeout != time.Second {
		t.Fatalf("tiered defaults = %+v", first.Tiered)
	}

	first.Value.RemoteTTL = 2 * time.Hour
	if second.Value.RemoteTTL != time.Hour {
		t.Fatalf("defaults share state: %+v", second.Value)
	}
}

func TestConfigValidateBounds(t *testing.T) {
	valid := DefaultConfig()
	tests := []struct {
		name       string
		mutate     func(*Config)
		wantReason Reason
		wantIs     error
	}{
		{name: "negative remote ttl", mutate: func(c *Config) { c.Value.RemoteTTL = -1 }, wantReason: ReasonConfiguration, wantIs: btredis.ErrInvalidTTL},
		{name: "zero max bytes", mutate: func(c *Config) { c.Value.MaxValueBytes = 0 }, wantReason: ReasonConfiguration},
		{name: "max bytes above limit", mutate: func(c *Config) { c.Value.MaxValueBytes = 64<<20 + 1 }, wantReason: ReasonConfiguration},
		{name: "zero clear batch", mutate: func(c *Config) { c.Value.ClearBatchSize = 0 }, wantReason: ReasonConfiguration},
		{name: "clear batch above limit", mutate: func(c *Config) { c.Value.ClearBatchSize = 1001 }, wantReason: ReasonConfiguration},
		{name: "zero local ttl", mutate: func(c *Config) { c.Tiered.LocalTTL = 0 }, wantReason: ReasonConfiguration, wantIs: btredis.ErrInvalidTTL},
		{name: "zero invalidation timeout", mutate: func(c *Config) { c.Tiered.InvalidationWaitTimeout = 0 }, wantReason: ReasonConfiguration, wantIs: btredis.ErrInvalidTTL},
		{name: "zero cleanup timeout", mutate: func(c *Config) { c.Tiered.LocalCleanupTimeout = 0 }, wantReason: ReasonConfiguration, wantIs: btredis.ErrInvalidTTL},
		{name: "local ttl exceeds remote", mutate: func(c *Config) { c.Tiered.LocalTTL = c.Value.RemoteTTL + time.Nanosecond }, wantReason: ReasonConfiguration, wantIs: btredis.ErrInvalidTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := valid
			tt.mutate(&config)
			err := config.Validate()
			if !hasReason(err, tt.wantReason) {
				t.Fatalf("Validate() = %v, want reason %q", err, tt.wantReason)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Fatalf("Validate() = %v, want errors.Is(%v)", err, tt.wantIs)
			}
		})
	}
}

func TestConfigValidateAcceptsBoundaryValues(t *testing.T) {
	tests := []Config{
		{
			Value:  ValueConfig{RemoteTTL: 0, MaxValueBytes: 1, ClearBatchSize: 1},
			Tiered: TieredConfig{LocalTTL: time.Nanosecond, InvalidationWaitTimeout: time.Nanosecond, LocalCleanupTimeout: time.Nanosecond},
		},
		{
			Value:  ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 64 << 20, ClearBatchSize: 1000},
			Tiered: TieredConfig{LocalTTL: time.Hour, InvalidationWaitTimeout: time.Hour, LocalCleanupTimeout: time.Hour},
		},
	}

	for i, config := range tests {
		if err := config.Validate(); err != nil {
			t.Fatalf("case %d Validate() = %v", i, err)
		}
	}
}

func TestInputValidationPreservesRedisSentinels(t *testing.T) {
	if err := validateNamespace("tenant*"); !errors.Is(err, btredis.ErrInvalidKey) {
		t.Fatalf("namespace = %v", err)
	}
	if err := validateLogicalKey(" "); !errors.Is(err, btredis.ErrInvalidKey) {
		t.Fatalf("logical key = %v", err)
	}
	if err := validateEntryTTL(-time.Nanosecond); !errors.Is(err, btredis.ErrInvalidTTL) {
		t.Fatalf("ttl = %v", err)
	}
}

func TestInputValidationNamespaceContract(t *testing.T) {
	for _, namespace := range []string{"a", "tenant-1", "tenant_1", "tenant.1", strings.Repeat("a", 128)} {
		if err := validateNamespace(namespace); err != nil {
			t.Fatalf("validateNamespace(%q) = %v", namespace, err)
		}
	}
	for _, namespace := range []string{"", " ", "tenant:a", "tenant*", "tenant?", "tenant[1]", `tenant\\1`, strings.Repeat("a", 129), "한글"} {
		if err := validateNamespace(namespace); !errors.Is(err, btredis.ErrInvalidKey) {
			t.Fatalf("validateNamespace(%q) = %v", namespace, err)
		}
	}
}

func TestInputValidationLogicalKeyPreservesAcceptedBytes(t *testing.T) {
	for _, key := range []string{"a", " leading", "trailing ", "a:b", "tenant*", strings.Repeat("k", 1024)} {
		if err := validateLogicalKey(key); err != nil {
			t.Fatalf("validateLogicalKey(%q) = %v", key, err)
		}
	}
	for _, key := range []string{"", " \t\n", strings.Repeat("k", 1025)} {
		if err := validateLogicalKey(key); !errors.Is(err, btredis.ErrInvalidKey) {
			t.Fatalf("validateLogicalKey(%q) = %v", key, err)
		}
	}
}

func TestInputValidationBuildsStablePhysicalKey(t *testing.T) {
	builder, err := newValueKeyBuilder("catalog")
	if err != nil {
		t.Fatal(err)
	}
	key, err := builder.LogicalKey("a:b ")
	if err != nil {
		t.Fatal(err)
	}
	if key.Value != "bluetape:cache:value:catalog:a:b " {
		t.Fatalf("physical key = %q", key.Value)
	}
}

func hasReason(err error, reason Reason) bool {
	var cacheErr *CacheError
	return errors.As(err, &cacheErr) && cacheErr.Reason() == reason
}
