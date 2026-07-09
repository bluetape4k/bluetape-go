package redisstreams

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
	"github.com/redis/go-redis/v9"
)

const defaultStream = "audit:sqloutbox"

// Client is the narrow go-redis surface used by Publisher.
type Client interface {
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
}

// Options configures a Redis Streams sqloutbox publisher.
type Options struct {
	// Client is the caller-owned Redis client.
	Client Client
	// Stream is the Redis stream key. The default is "audit:sqloutbox".
	Stream string
}

// Publisher appends sqloutbox records to one Redis stream.
type Publisher struct {
	client Client
	stream string
}

var _ sqloutbox.Publisher = (*Publisher)(nil)

// New creates a Redis Streams sqloutbox publisher.
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

// Stream returns the Redis stream key used by the publisher.
func (p *Publisher) Stream() string {
	if p == nil {
		return ""
	}
	return p.stream
}

// Publish appends one Redis stream entry for record.
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
	if _, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		Values: values,
	}).Result(); err != nil {
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
