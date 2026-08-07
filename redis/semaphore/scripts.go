package redissem

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	acquireScript = redis.NewScript(`
local now = redis.call("TIME")
local nowMillis = now[1] * 1000 + math.floor(now[2] / 1000)
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", nowMillis)
if redis.call("ZCARD", KEYS[1]) >= tonumber(ARGV[1]) then
    return 0
end
redis.call("ZADD", KEYS[1], nowMillis + tonumber(ARGV[2]), ARGV[3])
return 1
`)
	releaseScript = redis.NewScript(`
return redis.call("ZREM", KEYS[1], ARGV[1])
`)
)

func parseAcquireResult(cmd *redis.Cmd) (bool, error) {
	result, err := cmd.Int64()
	if err != nil {
		return false, err
	}
	switch result {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("unexpected semaphore acquire script result %d", result)
	}
}
