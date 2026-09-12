package sqlratelimit_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	redisratelimit "github.com/bluetape4k/bluetape-go/ratelimit/redis"
	sqlratelimit "github.com/bluetape4k/bluetape-go/ratelimit/sql"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func TestProviderCutoverIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", postgrestestcontainer.Start(ctx, t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, sqlratelimit.SchemaSQL); err != nil {
		t.Fatal(err)
	}
	// 전체 실행 동안 한 token도 채워지지 않도록 refill 시간을 실행 제한보다 길게 둔다.
	const rate = 0.001
	const burst = int64(3)
	const idleTTL = time.Hour
	factories := []struct {
		name string
		new  func(string) (ratelimit.Limiter, error)
	}{
		{"local", func(_ string) (ratelimit.Limiter, error) {
			return ratelimit.New(ratelimit.Options{RatePerSecond: rate, Burst: burst, IdleTTL: idleTTL})
		}},
		{"redis", func(namespace string) (ratelimit.Limiter, error) {
			return redisratelimit.New(redisratelimit.Options{Client: client, Namespace: namespace, RatePerSecond: rate, Burst: burst, IdleTTL: idleTTL})
		}},
		{"sql", func(namespace string) (ratelimit.Limiter, error) {
			return sqlratelimit.New(db, sqlratelimit.Options{Namespace: namespace, RatePerSecond: rate, Burst: burst, IdleTTL: idleTTL})
		}},
	}
	for _, from := range factories {
		for _, to := range factories {
			t.Run(from.name+"-to-"+to.name, func(t *testing.T) {
				oldProvider, err := from.new(t.Name() + ":old")
				if err != nil {
					t.Fatal(err)
				}
				newProvider, err := to.new(t.Name() + ":new")
				if err != nil {
					t.Fatal(err)
				}
				const key = "same-caller"
				assertCutoverAllow(ctx, t, oldProvider, key, burst, true)
				stopped, stop := context.WithCancel(ctx)
				stop()
				result, err := oldProvider.Allow(stopped, "stopped-cohort", burst)
				if result != (ratelimit.Result{}) || !errors.Is(err, context.Canceled) {
					t.Fatalf("quiesced request = %+v, %v", result, err)
				}
				assertCutoverAllow(ctx, t, oldProvider, "stopped-cohort", burst, true)
				// 이전 호출이 반환된 뒤 전환한다. state 이전이 없으므로 신규 provider는 full burst다.
				assertCutoverAllow(ctx, t, newProvider, key, burst, true)
				assertCutoverAllow(ctx, t, oldProvider, key, 1, false)
				assertCutoverAllow(ctx, t, newProvider, key, 1, false)
				// 별도 cohort는 독립 quota다. rollback은 기존 객체/namespace를 재사용해야 한다.
				assertCutoverAllow(ctx, t, oldProvider, "other-cohort", burst, true)
			})
		}
	}
}

func assertCutoverAllow(ctx context.Context, t *testing.T, limiter ratelimit.Limiter, key string, tokens int64, allowed bool) {
	t.Helper()
	result, err := limiter.Allow(ctx, key, tokens)
	if err != nil || result.Allowed != allowed || result.Requested != tokens {
		t.Fatalf("Allow(%q, %d) = %+v, %v; allowed=%v", key, tokens, result, err, allowed)
	}
	if allowed && result.Remaining != 0 {
		t.Fatalf("full burst remaining = %d, want 0", result.Remaining)
	}
	if !allowed && result.RetryAfter <= 0 {
		t.Fatalf("rejection RetryAfter = %s", result.RetryAfter)
	}
}

func TestProviderCutoverRedisUnknownDoesNotReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	addr := redistestcontainer.Start(ctx, t)
	for _, tc := range []struct {
		name       string
		maxRetries int
		debits     int64
	}{
		{"disabled", -1, 1},
		{"unsafe-default-control", 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hook := &cutoverLostResponse{}
			client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: tc.maxRetries})
			client.AddHook(hook)
			t.Cleanup(func() { _ = client.Close() })
			if err := client.Ping(ctx).Err(); err != nil {
				t.Fatal(err)
			}
			limiter, err := redisratelimit.New(redisratelimit.Options{
				Client: client, Namespace: "cutover-" + tc.name, RatePerSecond: 0.001, Burst: 3, IdleTTL: time.Hour,
			})
			if err != nil {
				t.Fatal(err)
			}
			hook.loseNext.Store(true)
			result, err := limiter.Allow(ctx, "caller", 1)
			if hook.loseNext.Load() || hook.debits.Load() != tc.debits {
				t.Fatalf("transport injection: armed=%v, actual debits=%d, want %d", hook.loseNext.Load(), hook.debits.Load(), tc.debits)
			}
			if tc.maxRetries == -1 {
				if result != (ratelimit.Result{}) || !errors.Is(err, ratelimit.ErrCommitUnknown) || !errors.Is(err, io.EOF) {
					t.Fatalf("lost response = %+v, %v", result, err)
				}
			} else if err != nil || !result.Allowed || result.Remaining != 1 {
				t.Fatalf("unsafe default retry control = %+v, %v", result, err)
			}
			// 실패 요청의 replay가 아닌 별도 검증 요청으로 실제 잔여 quota를 확인한다.
			assertCutoverAllow(ctx, t, limiter, "caller", 3-tc.debits, true)
			assertCutoverAllow(ctx, t, limiter, "caller", 1, false)
			if got := hook.debits.Load(); got != tc.debits+2 {
				t.Fatalf("total debit commands = %d, want %d", got, tc.debits+2)
			}
		})
	}
}

type cutoverLostResponse struct {
	loseNext atomic.Bool
	debits   atomic.Int64
}

func (h *cutoverLostResponse) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := next(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &cutoverResponseConn{Conn: conn, hook: h}, nil
	}
}

func (*cutoverLostResponse) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}

func (*cutoverLostResponse) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

type cutoverResponseConn struct {
	net.Conn
	hook *cutoverLostResponse
	lost atomic.Bool
}

func (c *cutoverResponseConn) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("$4\r\neval\r\n")) {
		c.hook.debits.Add(1)
	}
	return c.Conn.Write(p)
}

func (c *cutoverResponseConn) Read(p []byte) (int, error) {
	if c.lost.Load() {
		return 0, io.EOF
	}
	n, err := c.Conn.Read(p)
	if n > 0 && c.hook.loseNext.Swap(false) {
		// 실제 응답을 소비한 뒤 EOF를 반환해 client 내부 transport retry 경계를 통과한다.
		c.lost.Store(true)
		return 0, io.EOF
	}
	return n, err
}
