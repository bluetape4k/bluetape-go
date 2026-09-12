package redisbloom

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestCuckooUnsupported(t *testing.T) {
	client := newRedisClient(t)
	c, err := NewCuckoo(CuckooOptions{Client: client, Namespace: "cuckoo-unsupported"})
	if assertionErr := err; assertionErr != nil {
		t.Fatalf("unexpected error: %v", assertionErr)
	}
	err = c.Reserve(redisTestContext(t), CuckooReserveOptions{Capacity: 16})
	if assertionErr := err; !errors.Is(assertionErr, ErrCuckooUnsupported) {
		t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrCuckooUnsupported)
	}
	if assertionErr := err; errors.Is(assertionErr, btredis.ErrCommitUnknown) {
		t.Fatalf("error match NotErrorIs failed: %v; target: %v", assertionErr, btredis.ErrCommitUnknown)
	}
}

func TestCuckooModule(t *testing.T) {
	if os.Getenv("BLUETAPE_CUCKOO_INTEGRATION") != "1" {
		t.Skip("set BLUETAPE_CUCKOO_INTEGRATION=1 for the digest-pinned Redis 8 module fixture")
	}
	startup, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	container, err := tcredis.Run(startup, "redis@sha256:3eafabb4c93fcb8b36b666e07a43f096cb157bc6b07dce4b2492b895c63cf37f")
	if assertionErr := err; assertionErr != nil {
		t.Fatalf("unexpected error: %v", assertionErr)
	}
	testcleanup.Register(startup, t, "cuckoo", container)
	addr, err := container.PortEndpoint(startup, "6379/tcp", "")
	if assertionErr := err; assertionErr != nil {
		t.Fatalf("unexpected error: %v", assertionErr)
	}
	for _, protocol := range []int{2, 3} {
		t.Run(fmt.Sprintf("resp%d", protocol), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			client := newCuckooModuleClient(t, addr, protocol, -1)
			if assertionErr := client.Ping(ctx).Err(); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			c := newCuckooAt(t, client, fmt.Sprintf("cuckoo-resp%d", protocol))
			if assertionErr := c.Add(ctx, "item"); assertionErr == nil {
				t.Fatal("expected an error")
			}
			found, err := c.Exists(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if found {
				t.Fatal("expected false: condition")
			}
			count, err := c.Count(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if count != 0 {
				t.Fatalf("expected zero, got %v", count)
			}
			if assertionErr := c.Reserve(ctx, CuckooReserveOptions{Capacity: 64}); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if assertionErr := c.Reserve(ctx, CuckooReserveOptions{Capacity: 64}); assertionErr == nil {
				t.Fatal("expected an error")
			}
			if assertionErr := c.Add(ctx, "item"); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if assertionErr := c.Add(ctx, "item"); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			count, err = c.Count(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if assertionExpected, assertionActual := int64(2), count; !reflect.DeepEqual(assertionExpected, assertionActual) {
				t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
			}
			found, err = c.Exists(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if !(found) {
				t.Fatal("expected true: condition")
			}
			deleted, err := c.Delete(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if !(deleted) {
				t.Fatal("expected true: condition")
			}
			count, err = c.Count(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if assertionExpected, assertionActual := int64(1), count; !reflect.DeepEqual(assertionExpected, assertionActual) {
				t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
			}
			other := newCuckooAt(t, client, fmt.Sprintf("cuckoo-other%d", protocol))
			found, err = other.Exists(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if found {
				t.Fatal("expected false: condition")
			}
			if assertionErr := client.Set(ctx, other.(*cuckoo).key, "wrong-type", 0).Err(); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			found, err = other.Exists(ctx, "item")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if found {
				t.Fatal("expected false: condition")
			}

			full := newCuckooAt(t, client, fmt.Sprintf("cuckoo-full%d", protocol))
			if assertionErr := full.Reserve(ctx, CuckooReserveOptions{Capacity: 4}); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			inserted := int64(0)
			before := int64(0)
			saturated := false
			for range 128 {
				before, err = full.Count(ctx, "same")
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				err = full.Add(ctx, "same")
				if err != nil {
					if assertionErr := err; !errors.Is(assertionErr, ErrCuckooFull) {
						t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrCuckooFull)
					}
					if assertionErr := err; errors.Is(assertionErr, btredis.ErrCommitUnknown) {
						t.Fatalf("error match NotErrorIs failed: %v; target: %v", assertionErr, btredis.ErrCommitUnknown)
					}
					saturated = true
					break
				}
				inserted++
			}
			if !(saturated) {
				t.Fatal("expected true: 'bounded saturation was not observed'")
			}
			count, err = full.Count(ctx, "same")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if inserted <= 0 {
				t.Fatalf("expected positive value, got %v", inserted)
			}
			if assertionExpected, assertionActual := before, count; !reflect.DeepEqual(assertionExpected, assertionActual) {
				t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
			}
			// 고정 fixture의 두 후보 bucket이 겹쳐 같은 item도 중복 집계된다.
			if assertionExpected, assertionActual := 2*inserted, count; !reflect.DeepEqual(assertionExpected, assertionActual) {
				t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
			}

			concurrent := newCuckooAt(t, client, fmt.Sprintf("cuckoo-concurrent%d", protocol))
			if assertionErr := concurrent.Reserve(ctx, CuckooReserveOptions{Capacity: 512, BucketSize: 64}); assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			results := make(chan error, 64)
			var wg sync.WaitGroup
			for range 64 {
				wg.Go(func() { results <- concurrent.Add(ctx, "same") })
			}
			wg.Wait()
			close(results)
			for err := range results {
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
			}
			count, err = concurrent.Count(ctx, "same")
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if assertionExpected, assertionActual := int64(64), count; !reflect.DeepEqual(assertionExpected, assertionActual) {
				t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
			}
		})
	}
	t.Run("response-loss", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		observer := newCuckooModuleClient(t, addr, 3, -1)
		for _, tc := range []struct {
			name    string
			retries int
			want    int64
		}{{"disabled", -1, 1}, {"unsafe-default", 0, 2}} {
			t.Run(tc.name, func(t *testing.T) {
				hook := &cuckooLostResponse{}
				client := newCuckooModuleClient(t, addr, 3, tc.retries)
				client.AddHook(hook)
				if assertionErr := client.Ping(ctx).Err(); assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				c := newCuckooAt(t, client, "cuckoo-lost-"+tc.name)
				if assertionErr := c.Reserve(ctx, CuckooReserveOptions{Capacity: 64}); assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				hook.loseNext.Store(true)
				err := c.Add(ctx, "item")
				if hook.loseNext.Load() {
					t.Fatal("expected false: 'transport injection not consumed'")
				}
				if assertionExpected, assertionActual := tc.want, hook.inserts.Load(); !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if tc.retries == -1 {
					if assertionErr := err; !errors.Is(assertionErr, io.EOF) {
						t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, io.EOF)
					}
					if assertionErr := err; !errors.Is(assertionErr, btredis.ErrCommitUnknown) {
						t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, btredis.ErrCommitUnknown)
					}
				} else {
					if assertionErr := err; assertionErr != nil {
						t.Fatalf("unexpected error: %v", assertionErr)
					}
				}
				count, err := newCuckooAt(t, observer, "cuckoo-lost-"+tc.name).Count(ctx, "item")
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				if assertionExpected, assertionActual := tc.want, count; !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
			})
		}
	})
}

func newCuckooModuleClient(t *testing.T, addr string, protocol, maxRetries int) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: addr, Protocol: protocol, MaxRetries: maxRetries, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, ContextTimeoutEnabled: true})
	t.Cleanup(func() {
		if assertionErr := client.Close(); assertionErr != nil {
			t.Fatalf("unexpected error: %v", assertionErr)
		}
	})
	return client
}

func newCuckooAt(t *testing.T, client CuckooClient, namespace string) Cuckoo {
	t.Helper()
	c, err := NewCuckoo(CuckooOptions{Client: client, Namespace: namespace})
	if assertionErr := err; assertionErr != nil {
		t.Fatalf("unexpected error: %v", assertionErr)
	}
	return c
}

type cuckooLostResponse struct {
	loseNext atomic.Bool
	inserts  atomic.Int64
}

func (h *cuckooLostResponse) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := next(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &cuckooResponseConn{Conn: conn, hook: h}, nil
	}
}
func (*cuckooLostResponse) ProcessHook(next redis.ProcessHook) redis.ProcessHook { return next }
func (*cuckooLostResponse) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

type cuckooResponseConn struct {
	net.Conn
	hook          *cuckooLostResponse
	lost          atomic.Bool
	pendingInsert atomic.Bool
}

func (c *cuckooResponseConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	// 이 fixture는 작은 단일 CF.INSERT만 보낸다. 불완전한 write는 주입 대상으로 세지 않는다.
	if err == nil && n == len(p) && bytes.HasPrefix(bytes.ToLower(p), []byte("*5\r\n$9\r\ncf.insert\r\n")) {
		c.hook.inserts.Add(1)
		c.pendingInsert.Store(true)
	}
	return n, err
}
func (c *cuckooResponseConn) Read(p []byte) (int, error) {
	if c.lost.Load() {
		return 0, io.EOF
	}
	n, err := c.Conn.Read(p)
	if n > 0 && c.pendingInsert.Swap(false) && c.hook.loseNext.Swap(false) {
		c.lost.Store(true)
		return 0, io.EOF
	}
	return n, err
}

func TestCuckooResponseLossIgnoresOtherReplies(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := server.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	hook := &cuckooLostResponse{}
	hook.loseNext.Store(true)
	conn := &cuckooResponseConn{Conn: client, hook: hook}
	written := make(chan error, 1)
	go func() { _, err := server.Write([]byte("+PONG\r\n")); written <- err }()
	buffer := make([]byte, 32)
	n, err := conn.Read(buffer)
	if writeErr := <-written; writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil || string(buffer[:n]) != "+PONG\r\n" || !hook.loseNext.Load() {
		t.Fatalf("unrelated response consumed injection: n=%d err=%v", n, err)
	}
}
