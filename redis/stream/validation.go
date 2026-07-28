package redisstream

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

// ErrInvalidArgument는 변수 공개 값이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 호출자는 이 식별자를 Redis 오류, stream 상태, lease/token, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
var ErrInvalidArgument = errors.New("redis stream: invalid argument")

func prepareContext(ctx context.Context, client any) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if isNil(client) {
		return nil, invalidArgument("client")
	}
	return ctx, nil
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func invalidArgument(name string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, name)
}

func validateName(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalidArgument(name)
	}
	return nil
}

func validateIDs(ids []string) error {
	if len(ids) == 0 {
		return invalidArgument("message ids")
	}
	for _, id := range ids {
		if err := validateName("message id", id); err != nil {
			return err
		}
	}
	return nil
}

func validateStreams(streams []string) error {
	if len(streams) == 0 || len(streams)%2 != 0 {
		return invalidArgument("streams")
	}

	for _, stream := range streams[:len(streams)/2] {
		if err := validateName("stream", stream); err != nil {
			return err
		}
	}
	return nil
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}

func operationError(ctx context.Context, operation, rawKey string, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		err = errors.Join(err, contextErr)
	}
	return btredis.NewOpError(btredis.OpLabels{
		Family:    "redis stream",
		Operation: operation,
	}, rawKey, err)
}

func streamCorrelationKey(streams []string) string {
	var builder strings.Builder
	for _, stream := range streams {
		builder.WriteString(strconv.Itoa(len(stream)))
		builder.WriteByte(':')
		builder.WriteString(stream)
	}
	return builder.String()
}
