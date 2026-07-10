package redisstream

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	redis "github.com/redis/go-redis/v9"
)

func TestAppendRejectsInvalidInputWithoutDispatch(t *testing.T) {
	client := &appendFake{}
	typedNilValues := map[string]any(nil)
	typedNilClient := (*appendFake)(nil)

	tests := []struct {
		name   string
		client Appender
		args   redis.XAddArgs
	}{
		{name: "blank stream", client: client, args: redis.XAddArgs{Stream: "  ", Values: map[string]any{"kind": "test"}}},
		{name: "nil values", client: client, args: redis.XAddArgs{Stream: "orders"}},
		{name: "typed nil values", client: client, args: redis.XAddArgs{Stream: "orders", Values: typedNilValues}},
		{name: "nil client", args: redis.XAddArgs{Stream: "orders", Values: map[string]any{"kind": "test"}}},
		{name: "typed nil client", client: typedNilClient, args: redis.XAddArgs{Stream: "orders", Values: map[string]any{"kind": "test"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := client.calls
			_, err := Append(context.Background(), tt.client, tt.args)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("Append() error = %v, want ErrInvalidArgument", err)
			}
			if got := client.calls; got != before {
				t.Fatalf("XAdd calls = %d, want %d", got, before)
			}
		})
	}
}

func TestAppendPreservesArgumentsAndReturnsID(t *testing.T) {
	values := map[string]any{"kind": "created", "id": "42"}
	args := redis.XAddArgs{
		Stream: " orders: tenant-a ",
		ID:     "42-0",
		MaxLen: 128,
		Approx: true,
		Values: values,
	}
	client := &appendFake{id: "42-0"}

	id, err := Append(context.Background(), client, args)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if id != "42-0" {
		t.Fatalf("Append() id = %q, want 42-0", id)
	}
	if client.calls != 1 {
		t.Fatalf("XAdd calls = %d, want 1", client.calls)
	}
	if !reflect.DeepEqual(client.args, args) {
		t.Fatalf("XAdd args = %#v, want %#v", client.args, args)
	}
	if !reflect.DeepEqual(args.Values, values) {
		t.Fatalf("Append mutated caller values = %#v, want %#v", args.Values, values)
	}
}

func TestAppendPreservesCanceledContextWithoutDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &appendFake{}

	_, err := Append(ctx, client, redis.XAddArgs{Stream: "orders", Values: map[string]any{"kind": "test"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Append() error = %v, want context.Canceled", err)
	}
	if client.calls != 0 {
		t.Fatalf("XAdd calls = %d, want 0", client.calls)
	}
}

func TestAppendRedactsDispatchedError(t *testing.T) {
	const rawStream = "orders:secret-customer-42"
	const providerText = "redis provider secret"
	injected := errors.New(providerText)
	client := &appendFake{err: injected}

	_, err := Append(context.Background(), client, redis.XAddArgs{Stream: rawStream, Values: map[string]any{"kind": "test"}})
	if !errors.Is(err, injected) {
		t.Fatalf("Append() error = %v, want injected cause", err)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("Append() error = %T, want *btredis.OpError", err)
	}
	if got := err.Error(); containsAny(got, rawStream, providerText) {
		t.Fatalf("Append() error leaked sensitive text: %q", got)
	}
}

func TestOperationErrorRetainsDispatchedCauseAndExpiredContext(t *testing.T) {
	const rawStream = "orders:secret-customer-42"
	injected := errors.New("redis provider secret")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := operationError(ctx, "read", rawStream, injected)
	if !errors.Is(err, injected) {
		t.Fatalf("operationError() error = %v, want injected cause", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("operationError() error = %v, want context.Canceled", err)
	}
	if containsAny(err.Error(), rawStream, injected.Error()) {
		t.Fatalf("operationError() leaked sensitive text: %q", err)
	}
}

func containsAny(value string, values ...string) bool {
	for _, candidate := range values {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

type appendFake struct {
	calls int
	args  redis.XAddArgs
	id    string
	err   error
}

func (f *appendFake) XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd {
	f.calls++
	f.args = *args
	command := redis.NewStringCmd(ctx)
	if f.err != nil {
		command.SetErr(f.err)
		return command
	}
	command.SetVal(f.id)
	return command
}
