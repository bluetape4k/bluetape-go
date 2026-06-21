package jwt

import (
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisKeyPrefix   = "bluetape:jwt:v1"
	maxRedisNamespaceBytes  = 128
	defaultRedisMaxKeyBytes = 32 << 10
	minRedisMaxKeyBytes     = 1024
	maxRedisMaxKeyBytes     = 1 << 20
)

// RedisRepositoryOptions configures a Redis-backed distributed KeyChain repository.
type RedisRepositoryOptions struct {
	// Client is caller-owned. The repository never closes it.
	Client redis.Cmdable
	// Namespace scopes Redis signing authority keys.
	Namespace string
	// Capacity limits retained KeyChains.
	Capacity int
	// KeyTTL sets Redis expiration for stored KeyChains. Zero means no Redis TTL.
	KeyTTL time.Duration
	// RetentionLeeway is the maximum parse leeway Redis TTL must preserve.
	RetentionLeeway time.Duration
	// MaxKeyBytes limits each serialized KeyChain payload.
	MaxKeyBytes int
}

type redisRepositoryOptions struct {
	client          redis.Cmdable
	namespace       string
	capacity        int
	keyTTL          time.Duration
	retentionLeeway time.Duration
	maxKeyBytes     int
}

func (o RedisRepositoryOptions) normalize() (redisRepositoryOptions, error) {
	if o.Client == nil {
		return redisRepositoryOptions{}, OptionError{Option: "client", Err: errorsNew("must not be nil")}
	}
	namespace, err := normalizeRedisNamespace(o.Namespace)
	if err != nil {
		return redisRepositoryOptions{}, err
	}
	capacity := o.Capacity
	if capacity == 0 {
		capacity = defaultRepositorySize
	}
	if capacity < minRepositorySize || capacity > maxRepositorySize {
		return redisRepositoryOptions{}, OptionError{Option: "capacity", Err: errorsNew("outside repository capacity bounds")}
	}
	if o.KeyTTL < 0 {
		return redisRepositoryOptions{}, OptionError{Option: "key_ttl", Err: errorsNew("must not be negative")}
	}
	if o.RetentionLeeway < 0 {
		return redisRepositoryOptions{}, OptionError{Option: "retention_leeway", Err: errorsNew("must not be negative")}
	}
	maxKeyBytes := o.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultRedisMaxKeyBytes
	}
	if maxKeyBytes < minRedisMaxKeyBytes || maxKeyBytes > maxRedisMaxKeyBytes {
		return redisRepositoryOptions{}, OptionError{Option: "max_key_bytes", Err: errorsNew("outside redis key payload bounds")}
	}
	return redisRepositoryOptions{
		client:          o.Client,
		namespace:       namespace,
		capacity:        capacity,
		keyTTL:          o.KeyTTL,
		retentionLeeway: o.RetentionLeeway,
		maxKeyBytes:     maxKeyBytes,
	}, nil
}

func normalizeRedisNamespace(namespace string) (string, error) {
	trimmed := strings.TrimSpace(namespace)
	if trimmed == "" {
		return "", OptionError{Option: "namespace", Err: errorsNew("must not be empty")}
	}
	if len([]byte(trimmed)) > maxRedisNamespaceBytes {
		return "", OptionError{Option: "namespace", Err: fmt.Errorf("must be at most %d bytes", maxRedisNamespaceBytes)}
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return "", OptionError{Option: "namespace", Err: errorsNew("must contain only ASCII letters, digits, dot, underscore, or hyphen")}
		}
	}
	return trimmed, nil
}

func (o redisRepositoryOptions) keyPrefix() string {
	return defaultRedisKeyPrefix + ":" + o.namespace
}

func (o redisRepositoryOptions) metaKey() string {
	return o.keyPrefix() + ":meta"
}

func (o redisRepositoryOptions) currentKey() string {
	return o.keyPrefix() + ":current"
}

func (o redisRepositoryOptions) keysKey() string {
	return o.keyPrefix() + ":keys"
}

func (o redisRepositoryOptions) orderKey() string {
	return o.keyPrefix() + ":order"
}
