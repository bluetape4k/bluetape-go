package redisstream

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

// Appender interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Appender interface {
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
}

// Append Append 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: Append 동작에 필요한 args 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// Reader interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Reader interface {
	XRead(context.Context, *redis.XReadArgs) *redis.XStreamSliceCmd
}

// Read Read 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: Read 동작에 필요한 args 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// GroupCreator interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type GroupCreator interface {
	XGroupCreateMkStream(context.Context, string, string, string) *redis.StatusCmd
}

// CreateGroup CreateGroup 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - group: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - start: CreateGroup 동작에 필요한 start 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// GroupReader interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type GroupReader interface {
	XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd
}

// ReadGroup ReadGroup 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: ReadGroup 동작에 필요한 args 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// Acknowledger interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Acknowledger interface {
	XAck(context.Context, string, string, ...string) *redis.IntCmd
}

// Acknowledge Acknowledge 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - group: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - ids: Acknowledge 동작에 필요한 ids 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// PendingInspector interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type PendingInspector interface {
	XPendingExt(context.Context, *redis.XPendingExtArgs) *redis.XPendingExtCmd
}

// Pending Pending 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: Pending 동작에 필요한 args 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// AutoClaimer interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AutoClaimer interface {
	XAutoClaim(context.Context, *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
}

// AutoClaim AutoClaim 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - args: AutoClaim 동작에 필요한 args 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// Trimmer interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Trimmer interface {
	XTrimMaxLen(context.Context, string, int64) *redis.IntCmd
	XTrimMinID(context.Context, string, string) *redis.IntCmd
}

// TrimMaxLen TrimMaxLen 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - maxLen: TrimMaxLen 동작에 필요한 maxLen 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// TrimMinID TrimMinID 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - minID: TrimMinID 동작에 필요한 minID 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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

// Deleter interface 공개 타입이며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Deleter interface {
	XDel(context.Context, string, ...string) *redis.IntCmd
}

// Delete Delete 공개 API의 동작을 수행하며 Redis key, TTL, lease, token, script, stream primitive 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - client: Redis backend client 또는 fixture다. 연결과 종료 소유권은 생성자 계약을 따른다.
//   - stream: Redis Stream id, entry, 또는 consumer group 관련 값이다.
//   - ids: Delete 동작에 필요한 ids 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lease/token 불일치, 또는 package sentinel/typed error 계약을 보존한다.
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
