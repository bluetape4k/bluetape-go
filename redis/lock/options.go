package redislock

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

// Options configures a FencedLock.
type Options struct {
	// Key is the caller-owned logical lock identity. It is hashed before being
	// used in Redis keys and diagnostics.
	Key string
	// TTL is the owner lease duration.
	TTL time.Duration
}

type options struct {
	key string
	ttl time.Duration
}

func (o Options) normalize(client redis.Cmdable) (options, error) {
	if isNilClient(client) {
		return options{}, fmt.Errorf("%w: redis client", btredis.ErrInvalidKey)
	}
	if strings.TrimSpace(o.Key) == "" {
		return options{}, fmt.Errorf("%w: lock key", btredis.ErrInvalidKey)
	}
	if err := btredis.ValidateTTL("lock", o.TTL); err != nil {
		return options{}, err
	}
	return options{key: o.Key, ttl: o.TTL}, nil
}

func isNilClient(client redis.Cmdable) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
