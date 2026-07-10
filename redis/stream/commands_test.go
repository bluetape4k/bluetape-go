package redisstream

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	redis "github.com/redis/go-redis/v9"
)

func TestReadAndGroupReadPreserveGoRedisOrdering(t *testing.T) {
	fake := &streamCommandFake{
		readResult:  []redis.XStream{{Stream: " orders: tenant ", Messages: []redis.XMessage{{ID: "1-0"}}}},
		groupResult: []redis.XStream{{Stream: " orders: tenant ", Messages: []redis.XMessage{{ID: "2-0"}}}},
	}
	readArgs := redis.XReadArgs{Streams: []string{" orders: tenant ", "payments", "0", "$"}, Count: 4}
	groupArgs := redis.XReadGroupArgs{
		Group:    " group-a ",
		Consumer: " consumer-a ",
		Streams:  []string{" orders: tenant ", "payments", ">", "0"},
		NoAck:    true,
	}

	streams, err := Read(context.Background(), fake, readArgs)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if !reflect.DeepEqual(streams, fake.readResult) {
		t.Fatalf("Read() streams = %#v, want %#v", streams, fake.readResult)
	}
	if !reflect.DeepEqual(fake.readArgs, readArgs) {
		t.Fatalf("XRead args = %#v, want %#v", fake.readArgs, readArgs)
	}

	streams, err = ReadGroup(context.Background(), fake, groupArgs)
	if err != nil {
		t.Fatalf("ReadGroup() error = %v", err)
	}
	if !reflect.DeepEqual(streams, fake.groupResult) {
		t.Fatalf("ReadGroup() streams = %#v, want %#v", streams, fake.groupResult)
	}
	if !reflect.DeepEqual(fake.groupArgs, groupArgs) {
		t.Fatalf("XReadGroup args = %#v, want %#v", fake.groupArgs, groupArgs)
	}
}

func TestCommandHelpersPreserveArgumentsAndReturnValues(t *testing.T) {
	pending := []redis.XPendingExt{{ID: "1-0", Consumer: " consumer-a ", RetryCount: 2}}
	messages := []redis.XMessage{{ID: "1-0", Values: map[string]any{"kind": "created"}}}
	fake := &streamCommandFake{
		ackResult:       2,
		pendingResult:   pending,
		autoClaimResult: messages,
		autoClaimStart:  "2-0",
		trimResult:      3,
		deleteResult:    1,
	}
	ctx := context.Background()

	if err := CreateGroup(ctx, fake, " orders: tenant ", " group-a ", "0"); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if fake.createStream != " orders: tenant " || fake.createGroup != " group-a " || fake.createStart != "0" {
		t.Fatalf("CreateGroup args = %#v", fake)
	}

	acknowledged, err := Acknowledge(ctx, fake, " orders: tenant ", " group-a ", "1-0", "2-0")
	if err != nil || acknowledged != 2 {
		t.Fatalf("Acknowledge() = %d, %v; want 2, nil", acknowledged, err)
	}
	if !reflect.DeepEqual(fake.ackIDs, []string{"1-0", "2-0"}) {
		t.Fatalf("XAck ids = %#v", fake.ackIDs)
	}

	pendingArgs := redis.XPendingExtArgs{Stream: " orders: tenant ", Group: " group-a ", Start: "-", End: "+", Count: 10, Idle: time.Second}
	gotPending, err := Pending(ctx, fake, pendingArgs)
	if err != nil || !reflect.DeepEqual(gotPending, pending) {
		t.Fatalf("Pending() = %#v, %v; want %#v, nil", gotPending, err, pending)
	}
	if !reflect.DeepEqual(fake.pendingArgs, pendingArgs) {
		t.Fatalf("XPendingExt args = %#v, want %#v", fake.pendingArgs, pendingArgs)
	}

	autoClaimArgs := redis.XAutoClaimArgs{Stream: " orders: tenant ", Group: " group-a ", Consumer: " consumer-b ", Start: "0-0", MinIdle: time.Second}
	gotMessages, next, err := AutoClaim(ctx, fake, autoClaimArgs)
	if err != nil || next != "2-0" || !reflect.DeepEqual(gotMessages, messages) {
		t.Fatalf("AutoClaim() = %#v, %q, %v", gotMessages, next, err)
	}
	if !reflect.DeepEqual(fake.autoClaimArgs, autoClaimArgs) {
		t.Fatalf("XAutoClaim args = %#v, want %#v", fake.autoClaimArgs, autoClaimArgs)
	}

	if removed, err := TrimMaxLen(ctx, fake, " orders: tenant ", 5); err != nil || removed != 3 {
		t.Fatalf("TrimMaxLen() = %d, %v; want 3, nil", removed, err)
	}
	if fake.trimMaxLenStream != " orders: tenant " || fake.trimMaxLen != 5 {
		t.Fatalf("XTrimMaxLen args = %#v", fake)
	}
	if removed, err := TrimMinID(ctx, fake, " orders: tenant ", "2-0"); err != nil || removed != 3 {
		t.Fatalf("TrimMinID() = %d, %v; want 3, nil", removed, err)
	}
	if fake.trimMinIDStream != " orders: tenant " || fake.trimMinID != "2-0" {
		t.Fatalf("XTrimMinID args = %#v", fake)
	}
	if deleted, err := Delete(ctx, fake, " orders: tenant ", "1-0"); err != nil || deleted != 1 {
		t.Fatalf("Delete() = %d, %v; want 1, nil", deleted, err)
	}
	if !reflect.DeepEqual(fake.deleteIDs, []string{"1-0"}) {
		t.Fatalf("XDel ids = %#v", fake.deleteIDs)
	}
}

func TestCommandValidationAvoidsDispatch(t *testing.T) {
	fake := &streamCommandFake{}
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "read odd streams", run: func() error {
			_, err := Read(context.Background(), fake, redis.XReadArgs{Streams: []string{"orders"}})
			return err
		}},
		{name: "group read blank consumer", run: func() error {
			_, err := ReadGroup(context.Background(), fake, redis.XReadGroupArgs{Group: "group", Streams: []string{"orders", ">"}})
			return err
		}},
		{name: "ack no ids", run: func() error { _, err := Acknowledge(context.Background(), fake, "orders", "group"); return err }},
		{name: "auto claim negative idle", run: func() error {
			_, _, err := AutoClaim(context.Background(), fake, redis.XAutoClaimArgs{Stream: "orders", Group: "group", Consumer: "consumer", Start: "0-0", MinIdle: -time.Second})
			return err
		}},
		{name: "trim zero max length", run: func() error { _, err := TrimMaxLen(context.Background(), fake, "orders", 0); return err }},
		{name: "delete blank id", run: func() error { _, err := Delete(context.Background(), fake, "orders", " "); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := fake.calls
			if err := tt.run(); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("error = %v, want ErrInvalidArgument", err)
			}
			if fake.calls != before {
				t.Fatalf("redis calls = %d, want %d", fake.calls, before)
			}
		})
	}
}

func TestReadRedactsDispatchedError(t *testing.T) {
	const rawStream = "orders:secret-customer-42"
	const providerText = "redis provider secret"
	injected := errors.New(providerText)
	fake := &streamCommandFake{err: injected}

	_, err := Read(context.Background(), fake, redis.XReadArgs{Streams: []string{rawStream, "0"}})
	if !errors.Is(err, injected) {
		t.Fatalf("Read() error = %v, want injected cause", err)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("Read() error = %T, want *btredis.OpError", err)
	}
	if containsAny(err.Error(), rawStream, providerText) {
		t.Fatalf("Read() error leaked sensitive text: %q", err)
	}
}

type streamCommandFake struct {
	calls int
	err   error

	readArgs    redis.XReadArgs
	readResult  []redis.XStream
	groupArgs   redis.XReadGroupArgs
	groupResult []redis.XStream

	createStream string
	createGroup  string
	createStart  string

	ackStream string
	ackGroup  string
	ackIDs    []string
	ackResult int64

	pendingArgs   redis.XPendingExtArgs
	pendingResult []redis.XPendingExt

	autoClaimArgs   redis.XAutoClaimArgs
	autoClaimResult []redis.XMessage
	autoClaimStart  string

	trimMaxLenStream string
	trimMaxLen       int64
	trimMinIDStream  string
	trimMinID        string
	trimResult       int64

	deleteStream string
	deleteIDs    []string
	deleteResult int64
}

func (f *streamCommandFake) XRead(ctx context.Context, args *redis.XReadArgs) *redis.XStreamSliceCmd {
	f.calls++
	f.readArgs = *args
	command := redis.NewXStreamSliceCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.readResult)
	return command
}

func (f *streamCommandFake) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	f.calls++
	f.createStream, f.createGroup, f.createStart = stream, group, start
	command := redis.NewStatusCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal("OK")
	return command
}

func (f *streamCommandFake) XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	f.calls++
	f.groupArgs = *args
	command := redis.NewXStreamSliceCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.groupResult)
	return command
}

func (f *streamCommandFake) XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd {
	f.calls++
	f.ackStream, f.ackGroup, f.ackIDs = stream, group, append([]string(nil), ids...)
	command := redis.NewIntCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.ackResult)
	return command
}

func (f *streamCommandFake) XPendingExt(ctx context.Context, args *redis.XPendingExtArgs) *redis.XPendingExtCmd {
	f.calls++
	f.pendingArgs = *args
	command := redis.NewXPendingExtCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.pendingResult)
	return command
}

func (f *streamCommandFake) XAutoClaim(ctx context.Context, args *redis.XAutoClaimArgs) *redis.XAutoClaimCmd {
	f.calls++
	f.autoClaimArgs = *args
	command := redis.NewXAutoClaimCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.autoClaimResult, f.autoClaimStart)
	return command
}

func (f *streamCommandFake) XTrimMaxLen(ctx context.Context, stream string, maxLen int64) *redis.IntCmd {
	f.calls++
	f.trimMaxLenStream, f.trimMaxLen = stream, maxLen
	command := redis.NewIntCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.trimResult)
	return command
}

func (f *streamCommandFake) XTrimMinID(ctx context.Context, stream, minID string) *redis.IntCmd {
	f.calls++
	f.trimMinIDStream, f.trimMinID = stream, minID
	command := redis.NewIntCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.trimResult)
	return command
}

func (f *streamCommandFake) XDel(ctx context.Context, stream string, ids ...string) *redis.IntCmd {
	f.calls++
	f.deleteStream, f.deleteIDs = stream, append([]string(nil), ids...)
	command := redis.NewIntCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.deleteResult)
	return command
}
