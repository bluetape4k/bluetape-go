package redisstream

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

// Appender is the Redis command surface used by Append.
type Appender interface {
	XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
}

// Append adds one caller-owned entry to a Redis stream.
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

// Reader is the Redis command surface used by Read.
type Reader interface {
	XRead(context.Context, *redis.XReadArgs) *redis.XStreamSliceCmd
}

// Read reads entries from caller-selected streams and IDs.
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

// GroupCreator is the Redis command surface used by CreateGroup.
type GroupCreator interface {
	XGroupCreateMkStream(context.Context, string, string, string) *redis.StatusCmd
}

// CreateGroup creates a consumer group and its stream when absent.
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

// GroupReader is the Redis command surface used by ReadGroup.
type GroupReader interface {
	XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd
}

// ReadGroup reads entries for one caller-selected consumer group and consumer.
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

// Acknowledger is the Redis command surface used by Acknowledge.
type Acknowledger interface {
	XAck(context.Context, string, string, ...string) *redis.IntCmd
}

// Acknowledge removes caller-selected IDs from one consumer group's pending list.
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

// PendingInspector is the Redis command surface used by Pending.
type PendingInspector interface {
	XPendingExt(context.Context, *redis.XPendingExtArgs) *redis.XPendingExtCmd
}

// Pending returns caller-selected pending entries for one consumer group.
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

// AutoClaimer is the Redis command surface used by AutoClaim.
type AutoClaimer interface {
	XAutoClaim(context.Context, *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
}

// AutoClaim claims pending entries and returns the Redis-provided next cursor.
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

// Trimmer is the Redis command surface used by TrimMaxLen and TrimMinID.
type Trimmer interface {
	XTrimMaxLen(context.Context, string, int64) *redis.IntCmd
	XTrimMinID(context.Context, string, string) *redis.IntCmd
}

// TrimMaxLen explicitly trims a stream to a caller-selected maximum length.
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

// TrimMinID explicitly trims a stream before a caller-selected minimum ID.
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

// Deleter is the Redis command surface used by Delete.
type Deleter interface {
	XDel(context.Context, string, ...string) *redis.IntCmd
}

// Delete explicitly removes caller-selected IDs from a stream.
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
