package redisvalue

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"
)

var _ cache.Cache[string, valueTestRecord] = (*ValueCache[valueTestRecord])(nil)

type scanPage struct {
	cursor uint64
	keys   []string
	err    error
}

type clearFake struct {
	pages       []scanPage
	pageIndex   int
	pattern     string
	count       int64
	scanCursors []uint64
	unlinked    [][]string
	unlink      func(context.Context, ...string) *redis.IntCmd
}

func (f *clearFake) GetRange(context.Context, string, int64, int64) *redis.StringCmd {
	panic("unexpected GetRange")
}

func (f *clearFake) Exists(context.Context, ...string) *redis.IntCmd {
	panic("unexpected Exists")
}

func (f *clearFake) Set(context.Context, string, any, time.Duration) *redis.StatusCmd {
	panic("unexpected Set")
}

func (f *clearFake) Del(context.Context, ...string) *redis.IntCmd {
	panic("unexpected Del")
}

func (f *clearFake) Scan(_ context.Context, cursor uint64, pattern string, count int64) *redis.ScanCmd {
	f.scanCursors = append(f.scanCursors, cursor)
	f.pattern = pattern
	f.count = count
	if f.pageIndex >= len(f.pages) {
		panic("unexpected Scan")
	}
	page := f.pages[f.pageIndex]
	f.pageIndex++
	return redis.NewScanCmdResult(page.keys, page.cursor, page.err)
}

func (f *clearFake) Unlink(ctx context.Context, keys ...string) *redis.IntCmd {
	if f.unlink != nil {
		return f.unlink(ctx, keys...)
	}
	f.unlinked = append(f.unlinked, slices.Clone(keys))
	return redis.NewIntResult(int64(len(keys)), nil)
}

func newClearFake(pages ...scanPage) *clearFake {
	return &clearFake{pages: pages}
}

func TestValueCacheClearStreamsAndRechunksPages(t *testing.T) {
	client := newClearFake(
		scanPage{cursor: 7, keys: []string{"k1", "k2", "k3", "k4", "k5"}},
		scanPage{cursor: 0, keys: []string{"k6"}},
	)
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})

	if err := c.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([][]string{{"k1", "k2"}, {"k3", "k4"}, {"k5"}, {"k6"}}, client.unlinked); diff != "" {
		t.Fatalf("unlink chunks (-want +got):\n%s", diff)
	}
	if client.pattern != "bluetape:cache:value:catalog:*" || client.count != 2 {
		t.Fatalf("scan = %q/%d", client.pattern, client.count)
	}
	if diff := cmp.Diff([]uint64{0, 7}, client.scanCursors); diff != "" {
		t.Fatalf("scan cursors (-want +got):\n%s", diff)
	}
}

func TestValueCacheClearReportsScanFailureProgressWithoutSecrets(t *testing.T) {
	cause := errors.New("scan provider 127.0.0.1:6379 exposed raw:key")
	client := newClearFake(
		scanPage{cursor: 9, keys: []string{"k1", "k2"}},
		scanPage{err: cause},
	)
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})

	err := c.Clear(context.Background())
	assertClearError(t, err, cause, ClearProgress{ScannedKeys: 2, UnlinkedBatches: 1})
	if errors.Is(err, btredis.ErrCommitUnknown) {
		t.Fatalf("read-only SCAN failure is commit-unknown: %v", err)
	}
}

func TestValueCacheClearReportsUnlinkFailureAndRestartsAtCursorZero(t *testing.T) {
	cause := errors.New("unlink provider 127.0.0.1:6379 exposed raw:key")
	client := newClearFake(
		scanPage{cursor: 0, keys: []string{"k1", "k2", "k3"}},
		scanPage{cursor: 0},
	)
	var unlinkCalls int
	client.unlink = func(_ context.Context, keys ...string) *redis.IntCmd {
		unlinkCalls++
		if unlinkCalls == 2 {
			return redis.NewIntResult(0, cause)
		}
		client.unlinked = append(client.unlinked, slices.Clone(keys))
		return redis.NewIntResult(int64(len(keys)), nil)
	}
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})

	err := c.Clear(context.Background())
	assertClearError(t, err, cause, ClearProgress{ScannedKeys: 3, UnlinkedBatches: 1})
	if !errors.Is(err, btredis.ErrCommitUnknown) {
		t.Fatalf("UNLINK failure omitted commit ambiguity: %v", err)
	}
	client.unlink = nil
	if err := c.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]uint64{0, 0}, client.scanCursors); diff != "" {
		t.Fatalf("new Clear did not restart at cursor zero (-want +got):\n%s", diff)
	}
}

func TestValueCacheClearChecksCancellationBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := newClearFake(scanPage{cursor: 0, keys: []string{"k1", "k2", "k3"}})
	client.unlink = func(_ context.Context, keys ...string) *redis.IntCmd {
		client.unlinked = append(client.unlinked, slices.Clone(keys))
		cancel()
		return redis.NewIntResult(int64(len(keys)), nil)
	}
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})

	err := c.Clear(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Clear() = %v", err)
	}
	var cacheErr *CacheError
	if !errors.As(err, &cacheErr) || cacheErr.Reason() != ReasonPartialClear {
		t.Fatalf("Clear() = %v, want partial-clear", err)
	}
	progress, ok := cacheErr.ClearProgress()
	if !ok || progress != (ClearProgress{ScannedKeys: 3, UnlinkedBatches: 1}) {
		t.Fatalf("progress = %+v/%v", progress, ok)
	}
	if diff := cmp.Diff([][]string{{"k1", "k2"}}, client.unlinked); diff != "" {
		t.Fatalf("unlink chunks (-want +got):\n%s", diff)
	}
}

func TestValueCacheClearEmptyNamespaceSucceeds(t *testing.T) {
	client := newClearFake(scanPage{cursor: 0})
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})
	if err := c.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.unlinked) != 0 {
		t.Fatalf("empty namespace unlinked keys: %v", client.unlinked)
	}
}

func TestValueCacheClearZeroValueReturnsUninitialized(t *testing.T) {
	var c ValueCache[string]
	if err := c.Clear(context.Background()); !hasReason(err, ReasonUninitialized) {
		t.Fatalf("clear = %v", err)
	}
}

func assertClearError(t *testing.T, err, cause error, want ClearProgress) {
	t.Helper()
	if !errors.Is(err, cause) {
		t.Fatalf("Clear() = %v, want cause %v", err, cause)
	}
	var cacheErr *CacheError
	if !errors.As(err, &cacheErr) || cacheErr.Reason() != ReasonPartialClear {
		t.Fatalf("Clear() = %v, want partial-clear CacheError", err)
	}
	progress, ok := cacheErr.ClearProgress()
	if !ok || progress != want {
		t.Fatalf("progress = %+v/%v, want %+v/true", progress, ok, want)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("Clear() = %v, want redis.OpError", err)
	}
	for _, secret := range []string{"raw:key", "127.0.0.1", "6379", "k1", "k2", "k3"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("Clear Error() leaked %q: %q", secret, err.Error())
		}
	}
}
