package redisvalue

import (
	"fmt"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

func validateEntryTTL(ttl time.Duration) error {
	if ttl < 0 {
		return fmt.Errorf("%w: negative cache ttl", btredis.ErrInvalidTTL)
	}
	return nil
}

func normalizeWireTTL(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return 0
	}
	if ttl < time.Millisecond {
		return time.Millisecond
	}
	if ttl%time.Second == 0 {
		return ttl.Truncate(time.Second)
	}
	return ttl.Truncate(time.Millisecond)
}
