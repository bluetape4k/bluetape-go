package btredis

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func TestCompareAndDeleteValidationDoesNotDispatch(t *testing.T) {
	lease := testLease(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		run  func(*fakeScripter) (bool, error)
		want error
	}{
		{
			name: "nil context",
			run: func(fake *fakeScripter) (bool, error) {
				//nolint:staticcheck // This test verifies the public nil-context guard.
				return CompareAndDelete(nil, fake, lease, "redis test")
			},
			want: ErrInvalidKey,
		},
		{
			name: "nil client",
			run: func(_ *fakeScripter) (bool, error) {
				return CompareAndDelete(ctx, nil, lease, "redis test")
			},
			want: ErrInvalidKey,
		},
		{
			name: "typed nil client",
			run: func(_ *fakeScripter) (bool, error) {
				var client *typedNilScripter
				return CompareAndDelete(ctx, client, lease, "redis test")
			},
			want: ErrInvalidKey,
		},
		{
			name: "canceled context",
			run: func(fake *fakeScripter) (bool, error) {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				return CompareAndDelete(canceled, fake, lease, "redis test")
			},
			want: context.Canceled,
		},
		{
			name: "invalid lease",
			run: func(fake *fakeScripter) (bool, error) {
				return CompareAndDelete(ctx, fake, Lease{}, "redis test")
			},
			want: ErrInvalidKey,
		},
		{
			name: "invalid family label",
			run: func(fake *fakeScripter) (bool, error) {
				return CompareAndDelete(ctx, fake, lease, "redis:test")
			},
			want: ErrInvalidKey,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeScripter{}
			ok, err := tc.run(fake)
			if ok {
				t.Fatal("CompareAndDelete() ok = true, want false")
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("CompareAndDelete() error = %v, want %v", err, tc.want)
			}
			if fake.calls != 0 {
				t.Fatalf("fake calls = %d, want 0", fake.calls)
			}
		})
	}
}

func TestCompareAndExtendValidationDoesNotDispatch(t *testing.T) {
	lease := testLease(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		run  func(*fakeScripter) (bool, error)
		want error
	}{
		{
			name: "nil client",
			run: func(_ *fakeScripter) (bool, error) {
				return CompareAndExtend(ctx, nil, lease, time.Second, "redis test")
			},
			want: ErrInvalidKey,
		},
		{
			name: "invalid ttl",
			run: func(fake *fakeScripter) (bool, error) {
				return CompareAndExtend(ctx, fake, lease, time.Nanosecond, "redis test")
			},
			want: ErrInvalidTTL,
		},
		{
			name: "invalid family label",
			run: func(fake *fakeScripter) (bool, error) {
				return CompareAndExtend(ctx, fake, lease, time.Second, "redis:test")
			},
			want: ErrInvalidKey,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeScripter{}
			ok, err := tc.run(fake)
			if ok {
				t.Fatal("CompareAndExtend() ok = true, want false")
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("CompareAndExtend() error = %v, want %v", err, tc.want)
			}
			if fake.calls != 0 {
				t.Fatalf("fake calls = %d, want 0", fake.calls)
			}
		})
	}
}

func TestCompareAndHelpersParseScriptResults(t *testing.T) {
	lease := testLease(t)

	for _, result := range []struct {
		value int64
		want  bool
	}{
		{value: 1, want: true},
		{value: 0, want: false},
	} {
		t.Run(fmt.Sprintf("delete-%d", result.value), func(t *testing.T) {
			ok, err := CompareAndDelete(context.Background(), &fakeScripter{result: result.value}, lease, "redis test")
			if err != nil {
				t.Fatalf("CompareAndDelete() error = %v", err)
			}
			if ok != result.want {
				t.Fatalf("CompareAndDelete() = %t, want %t", ok, result.want)
			}
		})
		t.Run(fmt.Sprintf("extend-%d", result.value), func(t *testing.T) {
			ok, err := CompareAndExtend(context.Background(), &fakeScripter{result: result.value}, lease, time.Second, "redis test")
			if err != nil {
				t.Fatalf("CompareAndExtend() error = %v", err)
			}
			if ok != result.want {
				t.Fatalf("CompareAndExtend() = %t, want %t", ok, result.want)
			}
		})
	}
}

func TestCompareAndHelpersWrapRedisErrors(t *testing.T) {
	lease := testLease(t)
	token := lease.Token()
	cause := fmt.Errorf("provider failed for raw:key token=%s", token.RedisValue())
	_, err := CompareAndDelete(context.Background(), &fakeScripter{err: cause}, lease, "redis test")

	var opErr *OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("CompareAndDelete() error = %T, want OpError", err)
	}
	if !errors.Is(err, cause) {
		t.Fatal("CompareAndDelete() did not preserve cause")
	}
	if contains(err.Error(), lease.Key()) || contains(err.Error(), token.RedisValue()) {
		t.Fatal("CompareAndDelete() error leaked key or token")
	}
	if opErr.Operation() != "compare-delete" {
		t.Fatalf("OpError.Operation() = %q, want compare-delete", opErr.Operation())
	}
}

func testLease(tb testing.TB) Lease {
	tb.Helper()
	token, err := NewOwnerToken()
	if err != nil {
		tb.Fatalf("NewOwnerToken() error = %v", err)
	}
	lease, err := NewLease("raw:key", token)
	if err != nil {
		tb.Fatalf("NewLease() error = %v", err)
	}
	return lease
}

type fakeScripter struct {
	calls  int
	result int64
	err    error
}

func (f *fakeScripter) Eval(_ context.Context, _ string, _ []string, _ ...interface{}) *redis.Cmd {
	f.calls++
	return redis.NewCmdResult(f.result, f.err)
}

func (f *fakeScripter) EvalSha(_ context.Context, _ string, _ []string, _ ...interface{}) *redis.Cmd {
	f.calls++
	return redis.NewCmdResult(f.result, f.err)
}

func (f *fakeScripter) EvalRO(_ context.Context, _ string, _ []string, _ ...interface{}) *redis.Cmd {
	f.calls++
	return redis.NewCmdResult(f.result, f.err)
}

func (f *fakeScripter) EvalShaRO(_ context.Context, _ string, _ []string, _ ...interface{}) *redis.Cmd {
	f.calls++
	return redis.NewCmdResult(f.result, f.err)
}

func (f *fakeScripter) ScriptExists(_ context.Context, _ ...string) *redis.BoolSliceCmd {
	return redis.NewBoolSliceResult(nil, nil)
}

func (f *fakeScripter) ScriptLoad(_ context.Context, _ string) *redis.StringCmd {
	return redis.NewStringResult("", nil)
}

type typedNilScripter struct {
	fakeScripter
}
