package redislock

import (
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var acquireScript = redis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 1 then
    return {0, 0}
end
local fencing = redis.call("INCR", KEYS[2])
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
return {1, fencing}
`)

func parseAcquireResult(cmd *redis.Cmd) (bool, uint64, error) {
	values, err := int64Slice(cmd)
	if err != nil {
		return false, 0, err
	}
	if len(values) != 2 {
		return false, 0, fmt.Errorf("unexpected fenced lock script result length %d", len(values))
	}
	switch values[0] {
	case 0:
		if values[1] != 0 {
			return false, 0, fmt.Errorf("unexpected busy fenced lock token %d", values[1])
		}
		return false, 0, nil
	case 1:
		if values[1] <= 0 {
			return false, 0, fmt.Errorf("unexpected fencing token %d", values[1])
		}
		return true, uint64(values[1]), nil
	default:
		return false, 0, fmt.Errorf("unexpected fenced lock script status %d", values[0])
	}
}

func int64Slice(cmd *redis.Cmd) ([]int64, error) {
	if err := cmd.Err(); err != nil {
		return nil, err
	}
	switch value := cmd.Val().(type) {
	case []int64:
		return value, nil
	case []interface{}:
		values := make([]int64, len(value))
		for index, item := range value {
			parsed, err := int64Value(item)
			if err != nil {
				return nil, fmt.Errorf("unexpected fenced lock script result at index %d: %w", index, err)
			}
			values[index] = parsed
		}
		return values, nil
	default:
		return nil, fmt.Errorf("redis: unexpected type=%T for Slice", cmd.Val())
	}
}

func int64Value(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	case string:
		return strconv.ParseInt(typed, 10, 64)
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}
