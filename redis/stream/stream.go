package redisstream

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

// Appender Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type Appender interface {
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
}

// Append Redis key, TTL, lease, token, script, stream primitive의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: Append에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func Append(ctx context.Context, client Appender, args redis.XAddArgs) (string, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return "", err
	}
	if err := validateName("stream", args.Stream); err != nil {
		return "", err
	}
	if isNil(args.Values) {
		return "", invalidArgument("values")
	}

	result, err := client.XAdd(ctx, &args).Result()
	if err != nil {
		return "", operationError(ctx, "append", args.Stream, err)
	}
	return result, nil
}

// Reader Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type Reader interface {
	XRead(context.Context, *redis.XReadArgs) *redis.XStreamSliceCmd
}

// Read Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: Read에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func Read(ctx context.Context, client Reader, args redis.XReadArgs) ([]redis.XStream, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return nil, err
	}
	if err := validateStreams(args.Streams); err != nil {
		return nil, err
	}

	args.Streams = copyStrings(args.Streams)
	result, err := client.XRead(ctx, &args).Result()
	if err != nil {
		return nil, operationError(ctx, "read", streamCorrelationKey(args.Streams[:len(args.Streams)/2]), err)
	}
	return result, nil
}

// GroupCreator Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type GroupCreator interface {
	XGroupCreateMkStream(context.Context, string, string, string) *redis.StatusCmd
}

// CreateGroup Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - group: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - start: CreateGroup에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func CreateGroup(ctx context.Context, client GroupCreator, stream, group, start string) error {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return err
	}
	if err := validateName("stream", stream); err != nil {
		return err
	}
	if err := validateName("group", group); err != nil {
		return err
	}
	if err := validateName("start", start); err != nil {
		return err
	}

	if err := client.XGroupCreateMkStream(ctx, stream, group, start).Err(); err != nil {
		return operationError(ctx, "group-create", stream, err)
	}
	return nil
}

// GroupReader Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type GroupReader interface {
	XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd
}

// ReadGroup Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: ReadGroup에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func ReadGroup(ctx context.Context, client GroupReader, args redis.XReadGroupArgs) ([]redis.XStream, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return nil, err
	}
	if err := validateName("group", args.Group); err != nil {
		return nil, err
	}
	if err := validateName("consumer", args.Consumer); err != nil {
		return nil, err
	}
	if err := validateStreams(args.Streams); err != nil {
		return nil, err
	}

	args.Streams = copyStrings(args.Streams)
	result, err := client.XReadGroup(ctx, &args).Result()
	if err != nil {
		return nil, operationError(ctx, "group-read", streamCorrelationKey(args.Streams[:len(args.Streams)/2]), err)
	}
	return result, nil
}

// Acknowledger Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type Acknowledger interface {
	XAck(context.Context, string, string, ...string) *redis.IntCmd
}

// Acknowledge Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - group: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - ids: Acknowledge에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func Acknowledge(ctx context.Context, client Acknowledger, stream, group string, ids ...string) (int64, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return 0, err
	}
	if err := validateName("stream", stream); err != nil {
		return 0, err
	}
	if err := validateName("group", group); err != nil {
		return 0, err
	}
	if err := validateIDs(ids); err != nil {
		return 0, err
	}

	result, err := client.XAck(ctx, stream, group, copyStrings(ids)...).Result()
	if err != nil {
		return 0, operationError(ctx, "ack", stream, err)
	}
	return result, nil
}

// PendingInspector Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type PendingInspector interface {
	XPendingExt(context.Context, *redis.XPendingExtArgs) *redis.XPendingExtCmd
}

// Pending Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: Pending에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func Pending(ctx context.Context, client PendingInspector, args redis.XPendingExtArgs) ([]redis.XPendingExt, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return nil, err
	}
	if err := validateName("stream", args.Stream); err != nil {
		return nil, err
	}
	if err := validateName("group", args.Group); err != nil {
		return nil, err
	}

	result, err := client.XPendingExt(ctx, &args).Result()
	if err != nil {
		return nil, operationError(ctx, "pending", args.Stream, err)
	}
	return result, nil
}

// AutoClaimer Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type AutoClaimer interface {
	XAutoClaim(context.Context, *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
}

// AutoClaim Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: AutoClaim에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func AutoClaim(ctx context.Context, client AutoClaimer, args redis.XAutoClaimArgs) ([]redis.XMessage, string, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return nil, "", err
	}
	if err := validateName("stream", args.Stream); err != nil {
		return nil, "", err
	}
	if err := validateName("group", args.Group); err != nil {
		return nil, "", err
	}
	if err := validateName("consumer", args.Consumer); err != nil {
		return nil, "", err
	}
	if err := validateName("start", args.Start); err != nil {
		return nil, "", err
	}
	if args.MinIdle < 0 {
		return nil, "", invalidArgument("minimum idle")
	}

	messages, start, err := client.XAutoClaim(ctx, &args).Result()
	if err != nil {
		return nil, "", operationError(ctx, "autoclaim", args.Stream, err)
	}
	return messages, start, nil
}

// Trimmer Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type Trimmer interface {
	XTrimMaxLen(context.Context, string, int64) *redis.IntCmd
	XTrimMinID(context.Context, string, string) *redis.IntCmd
}

// TrimMaxLen Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - maxLen: TrimMaxLen에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func TrimMaxLen(ctx context.Context, client Trimmer, stream string, maxLen int64) (int64, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return 0, err
	}
	if err := validateName("stream", stream); err != nil {
		return 0, err
	}
	if maxLen <= 0 {
		return 0, invalidArgument("maximum length")
	}

	result, err := client.XTrimMaxLen(ctx, stream, maxLen).Result()
	if err != nil {
		return 0, operationError(ctx, "trim-maxlen", stream, err)
	}
	return result, nil
}

// TrimMinID Redis key, TTL, lease, token, script, stream primitive 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - minID: TrimMinID에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func TrimMinID(ctx context.Context, client Trimmer, stream, minID string) (int64, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return 0, err
	}
	if err := validateName("stream", stream); err != nil {
		return 0, err
	}
	if err := validateName("minimum id", minID); err != nil {
		return 0, err
	}

	result, err := client.XTrimMinID(ctx, stream, minID).Result()
	if err != nil {
		return 0, operationError(ctx, "trim-minid", stream, err)
	}
	return result, nil
}

// Deleter Redis key, TTL, lease, token, script, stream primitive에서 사용하는 인터페이스이다.
type Deleter interface {
	XDel(context.Context, string, ...string) *redis.IntCmd
}

// Delete Redis key, TTL, lease, token, script, stream primitive의 상태를 변경한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - ids: Delete에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func Delete(ctx context.Context, client Deleter, stream string, ids ...string) (int64, error) {
	ctx, err := prepareContext(ctx, client)
	if err != nil {
		return 0, err
	}
	if err := validateName("stream", stream); err != nil {
		return 0, err
	}
	if err := validateIDs(ids); err != nil {
		return 0, err
	}

	result, err := client.XDel(ctx, stream, copyStrings(ids)...).Result()
	if err != nil {
		return 0, operationError(ctx, "delete", stream, err)
	}
	return result, nil
}
