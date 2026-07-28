package jwt

import (
	"fmt"
	"strings"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisKeyPrefix   = "bluetape:jwt:v1"
	maxRedisNamespaceBytes  = 128
	defaultRedisMaxKeyBytes = 32 << 10
	minRedisMaxKeyBytes     = 1024
	maxRedisMaxKeyBytes     = 1 << 20
)

// RedisRepositoryOptions는 JWT key provider repository에서 설정값과 기본값 적용 방식을 설명한다.
type RedisRepositoryOptions struct {
	// Client는 JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
	Client redis.Cmdable
	// Namespace는 JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
	Namespace string
	// Capacity는 JWT key provider repository에서 동작과 caller-visible 계약을 설명한다.
	Capacity int
	// KeyTTL는 JWT key provider repository에서 설정값과 기본값 적용 방식을 설명한다.
	KeyTTL time.Duration
	// RetentionLeeway는 JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
	RetentionLeeway time.Duration
	// MaxKeyBytes는 JWT key provider repository에서 동작과 caller-visible 계약을 설명한다.
	MaxKeyBytes int
}

type redisRepositoryOptions struct {
	client          redis.Cmdable
	namespace       string
	keys            redisRepositoryKeys
	capacity        int
	keyTTL          time.Duration
	retentionLeeway time.Duration
	maxKeyBytes     int
}

type redisRepositoryKeys struct {
	meta    btredis.Key
	current btredis.Key
	keys    btredis.Key
	order   btredis.Key
}

func (o RedisRepositoryOptions) normalize() (redisRepositoryOptions, error) {
	if o.Client == nil {
		return redisRepositoryOptions{}, OptionError{Option: "client", Err: errorsNew("must not be nil")}
	}
	namespace, err := normalizeRedisNamespace(o.Namespace)
	if err != nil {
		return redisRepositoryOptions{}, err
	}
	keys, err := buildRedisRepositoryKeys(namespace)
	if err != nil {
		return redisRepositoryOptions{}, OptionError{Option: "namespace", Err: errorsNew("invalid redis key configuration")}
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
		keys:            keys,
		capacity:        capacity,
		keyTTL:          o.KeyTTL,
		retentionLeeway: o.RetentionLeeway,
		maxKeyBytes:     maxKeyBytes,
	}, nil
}

func buildRedisRepositoryKeys(namespace string) (redisRepositoryKeys, error) {
	builder, err := btredis.NewKeyBuilder(defaultRedisKeyPrefix)
	if err != nil {
		return redisRepositoryKeys{}, err
	}
	builder, err = builder.Structural(namespace)
	if err != nil {
		return redisRepositoryKeys{}, err
	}
	meta, err := builder.StructuralKey("meta")
	if err != nil {
		return redisRepositoryKeys{}, err
	}
	current, err := builder.StructuralKey("current")
	if err != nil {
		return redisRepositoryKeys{}, err
	}
	keys, err := builder.StructuralKey("keys")
	if err != nil {
		return redisRepositoryKeys{}, err
	}
	order, err := builder.StructuralKey("order")
	if err != nil {
		return redisRepositoryKeys{}, err
	}
	return redisRepositoryKeys{meta: meta, current: current, keys: keys, order: order}, nil
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

func (o redisRepositoryOptions) metaKey() string {
	return o.keys.meta.Value
}

func (o redisRepositoryOptions) currentKey() string {
	return o.keys.current.Value
}

func (o redisRepositoryOptions) keysKey() string {
	return o.keys.keys.Value
}

func (o redisRepositoryOptions) orderKey() string {
	return o.keys.order.Value
}
