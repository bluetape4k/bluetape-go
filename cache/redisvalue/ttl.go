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

func knownWriteLocalTTL(
	localTTL time.Duration,
	remoteTTL time.Duration,
	started time.Time,
	now func() time.Time,
) (time.Duration, bool) {
	if remoteTTL == 0 {
		return localTTL, true
	}
	wireTTL := normalizeWireTTL(remoteTTL)
	remaining := wireTTL - now().Sub(started)
	if remaining <= 0 {
		return 0, false
	}
	if localTTL < remaining {
		return localTTL, true
	}
	return remaining, true
}
