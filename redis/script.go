package btredis

import (
	"context"
	"fmt"
	"reflect"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var (
	compareDeleteScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)
	compareExtendScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)
)

// CompareAndDelete deletes the lease key only when the owner token still matches.
func CompareAndDelete(ctx context.Context, client redis.Scripter, lease Lease, family string) (bool, error) {
	if err := validateScriptInputs(ctx, client, lease); err != nil {
		return false, err
	}
	labels := OpLabels{Family: family, Operation: "compare-delete"}
	if err := labels.validate(); err != nil {
		return false, err
	}
	cmd := compareDeleteScript.Run(ctx, client, []string{lease.Key()}, lease.Token().RedisValue())
	return scriptBool(cmd, labels, lease.RedactedKeyID())
}

// CompareAndExtend extends the lease TTL only when the owner token still matches.
func CompareAndExtend(ctx context.Context, client redis.Scripter, lease Lease, ttl time.Duration, family string) (bool, error) {
	if err := validateScriptInputs(ctx, client, lease); err != nil {
		return false, err
	}
	millis, err := TTLMillis("lease ttl", ttl)
	if err != nil {
		return false, err
	}
	labels := OpLabels{Family: family, Operation: "compare-extend"}
	if err := labels.validate(); err != nil {
		return false, err
	}
	cmd := compareExtendScript.Run(ctx, client, []string{lease.Key()}, lease.Token().RedisValue(), millis)
	return scriptBool(cmd, labels, lease.RedactedKeyID())
}

func validateScriptInputs(ctx context.Context, client redis.Scripter, lease Lease) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", ErrInvalidKey)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if isNilScripter(client) {
		return fmt.Errorf("%w: nil redis client", ErrInvalidKey)
	}
	return lease.Validate()
}

func isNilScripter(client redis.Scripter) bool {
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

func scriptBool(cmd *redis.Cmd, labels OpLabels, keyID string) (bool, error) {
	result, err := cmd.Int64()
	if err != nil {
		return false, NewOpErrorWithRedactedKey(labels, keyID, err)
	}
	switch result {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, NewOpErrorWithRedactedKey(labels, keyID, fmt.Errorf("unexpected script result type %d", result))
	}
}
