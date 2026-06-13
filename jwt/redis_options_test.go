package jwt

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestOptionsNormalizeRequiresClient(t *testing.T) {
	_, err := (RedisRepositoryOptions{Namespace: "prod"}).normalize()
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("normalize() error = %v, want ErrInvalidOptions", err)
	}
}

func TestOptionsNormalizeRequiresNamespace(t *testing.T) {
	client := newRedisOptionsTestClient(t)
	for _, namespace := range []string{"", " "} {
		t.Run("namespace "+namespace, func(t *testing.T) {
			_, err := (RedisRepositoryOptions{Client: client, Namespace: namespace}).normalize()
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("normalize() error = %v, want ErrInvalidOptions", err)
			}
		})
	}
}

func TestOptionsNormalizeRejectsUnsafeNamespace(t *testing.T) {
	client := newRedisOptionsTestClient(t)
	valid := []string{"prod", "tenant-1", "tenant_1", "tenant.1"}
	for _, namespace := range valid {
		t.Run("valid "+namespace, func(t *testing.T) {
			normalized, err := (RedisRepositoryOptions{Client: client, Namespace: " " + namespace + " "}).normalize()
			if err != nil {
				t.Fatalf("normalize(%q) error = %v", namespace, err)
			}
			if normalized.namespace != namespace {
				t.Fatalf("namespace = %q, want %q", normalized.namespace, namespace)
			}
		})
	}

	invalid := []string{"", " ", "a:b", "tenant/name", "tenant\nname", "tenant name", "tenant\tname", "tenant{name}", "tenant*name", "tenant%name", "한글", strings.Repeat("x", 129)}
	for _, namespace := range invalid {
		t.Run("invalid "+namespace, func(t *testing.T) {
			_, err := (RedisRepositoryOptions{Client: client, Namespace: namespace}).normalize()
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("normalize(%q) error = %v, want ErrInvalidOptions", namespace, err)
			}
		})
	}
}

func TestOptionsNormalizeCapacityBounds(t *testing.T) {
	client := newRedisOptionsTestClient(t)
	normalized, err := (RedisRepositoryOptions{Client: client, Namespace: "prod"}).normalize()
	if err != nil {
		t.Fatalf("normalize(default) error = %v", err)
	}
	if normalized.capacity != defaultRepositorySize {
		t.Fatalf("capacity = %d, want %d", normalized.capacity, defaultRepositorySize)
	}

	for _, capacity := range []int{minRepositorySize, maxRepositorySize} {
		t.Run("valid", func(t *testing.T) {
			normalized, err := (RedisRepositoryOptions{Client: client, Namespace: "prod", Capacity: capacity}).normalize()
			if err != nil {
				t.Fatalf("normalize(capacity=%d) error = %v", capacity, err)
			}
			if normalized.capacity != capacity {
				t.Fatalf("capacity = %d, want %d", normalized.capacity, capacity)
			}
		})
	}
	for _, capacity := range []int{1, maxRepositorySize + 1, -1} {
		t.Run("invalid", func(t *testing.T) {
			_, err := (RedisRepositoryOptions{Client: client, Namespace: "prod", Capacity: capacity}).normalize()
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("normalize(capacity=%d) error = %v, want ErrInvalidOptions", capacity, err)
			}
		})
	}
}

func TestOptionsNormalizePayloadBounds(t *testing.T) {
	client := newRedisOptionsTestClient(t)
	normalized, err := (RedisRepositoryOptions{Client: client, Namespace: "prod"}).normalize()
	if err != nil {
		t.Fatalf("normalize(default) error = %v", err)
	}
	if normalized.maxKeyBytes != defaultRedisMaxKeyBytes {
		t.Fatalf("max key bytes = %d, want %d", normalized.maxKeyBytes, defaultRedisMaxKeyBytes)
	}

	for _, maxKeyBytes := range []int{minRedisMaxKeyBytes, maxRedisMaxKeyBytes} {
		t.Run("valid", func(t *testing.T) {
			normalized, err := (RedisRepositoryOptions{Client: client, Namespace: "prod", MaxKeyBytes: maxKeyBytes}).normalize()
			if err != nil {
				t.Fatalf("normalize(maxKeyBytes=%d) error = %v", maxKeyBytes, err)
			}
			if normalized.maxKeyBytes != maxKeyBytes {
				t.Fatalf("max key bytes = %d, want %d", normalized.maxKeyBytes, maxKeyBytes)
			}
		})
	}
	for _, maxKeyBytes := range []int{1, minRedisMaxKeyBytes - 1, maxRedisMaxKeyBytes + 1, -1} {
		t.Run("invalid", func(t *testing.T) {
			_, err := (RedisRepositoryOptions{Client: client, Namespace: "prod", MaxKeyBytes: maxKeyBytes}).normalize()
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("normalize(maxKeyBytes=%d) error = %v, want ErrInvalidOptions", maxKeyBytes, err)
			}
		})
	}
}

func TestOptionsNormalizeRetentionLeeway(t *testing.T) {
	client := newRedisOptionsTestClient(t)
	normalized, err := (RedisRepositoryOptions{Client: client, Namespace: "prod", RetentionLeeway: time.Minute}).normalize()
	if err != nil {
		t.Fatalf("normalize() error = %v", err)
	}
	if normalized.retentionLeeway != time.Minute {
		t.Fatalf("retention leeway = %v, want %v", normalized.retentionLeeway, time.Minute)
	}
	if _, err := (RedisRepositoryOptions{Client: client, Namespace: "prod", RetentionLeeway: -time.Nanosecond}).normalize(); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("normalize(negative retention leeway) error = %v, want ErrInvalidOptions", err)
	}
}

func TestRepositoryKeyNamesAreVersionedAndNamespaced(t *testing.T) {
	client := newRedisOptionsTestClient(t)
	normalized, err := (RedisRepositoryOptions{Client: client, Namespace: "prod"}).normalize()
	if err != nil {
		t.Fatalf("normalize() error = %v", err)
	}
	if got := normalized.metaKey(); got != "bluetape:jwt:v1:prod:meta" {
		t.Fatalf("metaKey() = %q", got)
	}
	if got := normalized.currentKey(); got != "bluetape:jwt:v1:prod:current" {
		t.Fatalf("currentKey() = %q", got)
	}
	if got := normalized.keysKey(); got != "bluetape:jwt:v1:prod:keys" {
		t.Fatalf("keysKey() = %q", got)
	}
	if got := normalized.orderKey(); got != "bluetape:jwt:v1:prod:order" {
		t.Fatalf("orderKey() = %q", got)
	}
}

func newRedisOptionsTestClient(t *testing.T) redis.Cmdable {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	return client
}
