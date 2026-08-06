package redisstream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

func TestAppendAndReadFromRedisStream(t *testing.T) {
	ctx, client, stream := redisStreamClient(t)

	firstID, err := Append(ctx, client, redis.XAddArgs{Stream: stream, Values: map[string]any{"kind": "created", "id": "1"}})
	if err != nil {
		t.Fatalf("Append first: %v", err)
	}
	if _, err := Append(ctx, client, redis.XAddArgs{Stream: stream, MaxLen: 32, Approx: true, Values: map[string]any{"kind": "created", "id": "2"}}); err != nil {
		t.Fatalf("Append optional trim: %v", err)
	}

	streams, err := Read(ctx, client, redis.XReadArgs{Streams: []string{stream, "0"}, Count: 10})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 2 {
		t.Fatalf("Read messages = %#v, want two entries", streams)
	}
	if streams[0].Messages[0].ID != firstID {
		t.Fatalf("first message id = %q, want %q", streams[0].Messages[0].ID, firstID)
	}
	if got := fmt.Sprint(streams[0].Messages[0].Values["kind"]); got != "created" {
		t.Fatalf("first message kind = %q, want created", got)
	}
}

func TestConsumerGroupPendingAcknowledgeAndAutoClaim(t *testing.T) {
	ctx, client, stream := redisStreamClient(t)
	if err := CreateGroup(ctx, client, stream, "workers", "0"); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	firstID, err := Append(ctx, client, redis.XAddArgs{Stream: stream, Values: map[string]any{"kind": "first"}})
	if err != nil {
		t.Fatalf("Append first: %v", err)
	}
	secondID, err := Append(ctx, client, redis.XAddArgs{Stream: stream, Values: map[string]any{"kind": "second"}})
	if err != nil {
		t.Fatalf("Append second: %v", err)
	}

	streams, err := ReadGroup(ctx, client, redis.XReadGroupArgs{Group: "workers", Consumer: "consumer-a", Streams: []string{stream, ">"}, Count: 2})
	if err != nil {
		t.Fatalf("ReadGroup: %v", err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 2 {
		t.Fatalf("ReadGroup messages = %#v, want two entries", streams)
	}

	pending, err := Pending(ctx, client, redis.XPendingExtArgs{Stream: stream, Group: "workers", Start: "-", End: "+", Count: 10})
	if err != nil {
		t.Fatalf("Pending before acknowledge: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("Pending entries = %#v, want two", pending)
	}

	acknowledged, err := Acknowledge(ctx, client, stream, "workers", firstID)
	if err != nil || acknowledged != 1 {
		t.Fatalf("Acknowledge = %d, %v; want 1, nil", acknowledged, err)
	}
	pending, err = Pending(ctx, client, redis.XPendingExtArgs{Stream: stream, Group: "workers", Start: "-", End: "+", Count: 10})
	if err != nil {
		t.Fatalf("Pending after acknowledge: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != secondID {
		t.Fatalf("Pending entries = %#v, want only %q", pending, secondID)
	}

	messages, next, err := AutoClaim(ctx, client, redis.XAutoClaimArgs{Stream: stream, Group: "workers", Consumer: "consumer-b", Start: "0-0", MinIdle: 0, Count: 10})
	if err != nil {
		t.Fatalf("AutoClaim: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != secondID || next == "" {
		t.Fatalf("AutoClaim = %#v, next %q; want %q", messages, next, secondID)
	}
}

func TestExplicitTrimAndDelete(t *testing.T) {
	ctx, client, stream := redisStreamClient(t)
	for index := range 3 {
		if _, err := Append(ctx, client, redis.XAddArgs{Stream: stream, Values: map[string]any{"index": index}}); err != nil {
			t.Fatalf("Append %d: %v", index, err)
		}
	}
	if _, err := TrimMaxLen(ctx, client, stream, 1); err != nil {
		t.Fatalf("TrimMaxLen: %v", err)
	}
	if length := client.XLen(ctx, stream).Val(); length > 1 {
		t.Fatalf("stream length after explicit trim = %d, want <= 1", length)
	}

	id, err := Append(ctx, client, redis.XAddArgs{Stream: stream, Values: map[string]any{"kind": "delete"}})
	if err != nil {
		t.Fatalf("Append delete target: %v", err)
	}
	deleted, err := Delete(ctx, client, stream, id)
	if err != nil || deleted != 1 {
		t.Fatalf("Delete = %d, %v; want 1, nil", deleted, err)
	}
}

func TestReadDeadlinePreservesTypedError(t *testing.T) {
	_, client, stream := redisStreamClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := Read(ctx, client, redis.XReadArgs{Streams: []string{stream, "$"}, Block: time.Second})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Read() error = %v, want context.DeadlineExceeded", err)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("Read() error = %T, want *btredis.OpError", err)
	}
}

func TestConcurrentAppendStress(t *testing.T) {
	ctx, client, stream := redisStreamClient(t)
	const taskCount = 24
	tasks := make([]concurrencytest.Task, 0, taskCount)
	for index := range taskCount {
		index := index
		tasks = append(tasks, func(ctx context.Context) error {
			_, err := Append(ctx, client, redis.XAddArgs{Stream: stream, Values: map[string]any{"index": index}})
			return err
		})
	}

	report := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       6,
		RoundsPerTask: 1,
		Timeout:       30 * time.Second,
	}).RunT(t, tasks...)
	if report.Completed != taskCount {
		t.Fatalf("completed tasks = %d, want %d", report.Completed, taskCount)
	}
	if length := client.XLen(ctx, stream).Val(); length != taskCount {
		t.Fatalf("stream length = %d, want %d", length, taskCount)
	}
}

func redisStreamClient(tb testing.TB) (context.Context, *redis.Client, string) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	tb.Cleanup(cancel)

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, tb)})
	stream := "bluetape:redis:stream:test:" + strings.ReplaceAll(tb.Name(), "/", ":") + ":" + uuid.NewString()
	tb.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.Del(cleanupCtx, stream).Err()
		_ = client.Close()
	})
	return ctx, client, stream
}
