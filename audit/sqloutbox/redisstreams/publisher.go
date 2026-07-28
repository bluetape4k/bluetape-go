package redisstreams

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	redisstream "github.com/bluetape4k/bluetape-go/redis/stream"
	"github.com/redis/go-redis/v9"
)

const defaultStream = "audit:sqloutbox"

// Client Publisher가 사용하는 좁은 Redis Streams append surface다.
type Client = redisstream.Appender

// Options Redis Stream outbox publish, idempotency, stream key에서 사용하는 구조체다.
type Options struct {
	// Client 호출자가 소유한 Redis client다.
	Client Client
	// Stream Redis stream key다. 기본값은 "audit:sqloutbox"다.
	Stream string
}

// Publisher Redis Stream outbox publish, idempotency, stream key에서 사용하는 구조체다.
type Publisher struct {
	client Client
	stream string
}

var _ sqloutbox.Publisher = (*Publisher)(nil)

// New Redis Stream outbox publish, idempotency, stream key에 사용할 값을 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func New(options Options) (*Publisher, error) {
	if isNilClient(options.Client) {
		return nil, fmt.Errorf("%w: redis client must not be nil", sqloutbox.ErrInvalidArgument)
	}
	stream := options.Stream
	if stream == "" {
		stream = defaultStream
	}
	if strings.TrimSpace(stream) == "" {
		return nil, fmt.Errorf("%w: redis stream must not be blank", sqloutbox.ErrInvalidArgument)
	}
	return &Publisher{client: options.Client, stream: stream}, nil
}

// Stream Redis Stream outbox publish, idempotency, stream key에서 필요한 값을 조회한다.
func (p *Publisher) Stream() string {
	if p == nil {
		return ""
	}
	return p.stream
}

// Publish Redis Stream outbox publish, idempotency, stream key의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - record: Publish에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소, Redis/backend 실패, lease/token 불일치, package sentinel error와 typed error를 그대로 드러낸다.
func (p *Publisher) Publish(ctx context.Context, record sqloutbox.Record) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || isNilClient(p.client) || p.stream == "" {
		return fmt.Errorf("%w: redis streams publisher is not initialized", sqloutbox.ErrInvalidArgument)
	}

	values, err := messageValues(record)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := redisstream.Append(ctx, p.client, redis.XAddArgs{
		Stream: p.stream,
		Values: values,
	}); err != nil {
		return fmt.Errorf("redis streams publish: %w", err)
	}
	return nil
}

func messageValues(record sqloutbox.Record) (map[string]any, error) {
	entryJSON, err := json.Marshal(record.Entry)
	if err != nil {
		return nil, fmt.Errorf("encode audit entry: %w", err)
	}
	return map[string]any{
		"record_id":       strconv.FormatInt(int64(record.ID), 10),
		"status":          string(record.Status),
		"aggregate_type":  record.Aggregate.Type,
		"aggregate_id":    record.Aggregate.ID,
		"revision":        strconv.FormatUint(uint64(record.Revision), 10),
		"event_id":        string(record.EventID),
		"idempotency_key": record.IdempotencyKey,
		"event_type":      string(record.EventType),
		"occurred_at":     record.OccurredAt.UTC().Format(timeFormatRFC3339Nano),
		"recorded_at":     record.RecordedAt.UTC().Format(timeFormatRFC3339Nano),
		"schema_version":  strconv.Itoa(record.SchemaVersion),
		"attempts":        strconv.Itoa(record.Attempts),
		"entry_json":      string(entryJSON),
	}, nil
}

const timeFormatRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func isNilClient(client Client) bool {
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
