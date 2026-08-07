package redissem

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

// Options configures a Semaphore.
type Options struct {
	// Key is the caller-owned logical semaphore identity. It is hashed before
	// being used in Redis keys and diagnostics.
	Key string
	// Permits is the maximum number of live leases.
	Permits int
	// TTL is the lease duration for each permit.
	TTL time.Duration
}

type options struct {
	key     string
	permits int
	ttl     time.Duration
}

func (o Options) normalize(client redis.Cmdable) (options, error) {
	if isNilClient(client) {
		return options{}, fmt.Errorf("%w: redis client", btredis.ErrInvalidKey)
	}
	if strings.TrimSpace(o.Key) == "" {
		return options{}, fmt.Errorf("%w: semaphore key", btredis.ErrInvalidKey)
	}
	if o.Permits <= 0 {
		return options{}, fmt.Errorf("%w: permits", btredis.ErrInvalidKey)
	}
	if err := btredis.ValidateTTL("semaphore", o.TTL); err != nil {
		return options{}, err
	}
	return options{key: o.Key, permits: o.Permits, ttl: o.TTL}, nil
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
