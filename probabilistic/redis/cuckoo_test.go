package redisbloom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

type cuckooFake struct {
	mu         sync.Mutex
	calls      [][]any
	result     any
	err        error
	after      func()
	nilCommand bool
}

type cuckooNilMap map[string]string

func (cuckooNilMap) Do(context.Context, ...any) *redis.Cmd { panic("nil client must not dispatch") }

type cuckooNilSlice []string

func (cuckooNilSlice) Do(context.Context, ...any) *redis.Cmd { panic("nil client must not dispatch") }

type cuckooNilFunc func()

func (cuckooNilFunc) Do(context.Context, ...any) *redis.Cmd { panic("nil client must not dispatch") }

type cuckooNilChan chan string

func (cuckooNilChan) Do(context.Context, ...any) *redis.Cmd { panic("nil client must not dispatch") }

type cuckooMethod struct {
	name     string
	mutation bool
	good     any
	zero     any
	invoke   func(Cuckoo, context.Context) (any, error)
	args     []any
}

func cuckooMethods(item string) []cuckooMethod {
	return []cuckooMethod{
		{"reserve", true, "OK", nil, func(c Cuckoo, ctx context.Context) (any, error) {
			return nil, c.Reserve(ctx, CuckooReserveOptions{Capacity: 16})
		}, []any{"CF.RESERVE", int64(16), "BUCKETSIZE", uint8(2), "MAXITERATIONS", uint16(20), "EXPANSION", uint16(0)}},
		{"add", true, []any{int64(1)}, nil, func(c Cuckoo, ctx context.Context) (any, error) { return nil, c.Add(ctx, item) }, []any{"CF.INSERT", "NOCREATE", "ITEMS", item}},
		{"exists", false, int64(1), false, func(c Cuckoo, ctx context.Context) (any, error) { return c.Exists(ctx, item) }, []any{"CF.EXISTS", item}},
		{"count", false, int64(2), int64(0), func(c Cuckoo, ctx context.Context) (any, error) { return c.Count(ctx, item) }, []any{"CF.COUNT", item}},
		{"delete", true, int64(1), false, func(c Cuckoo, ctx context.Context) (any, error) { return c.Delete(ctx, item) }, []any{"CF.DEL", item}},
	}
}

func newCuckooFake(t *testing.T, f *cuckooFake) Cuckoo {
	t.Helper()
	c, err := NewCuckoo(CuckooOptions{Client: f, Namespace: "tenant"})
	if assertionErr := err; assertionErr != nil {
		t.Fatalf("unexpected error: %v", assertionErr)
	}
	return c
}

func TestCuckooCommands(t *testing.T) {
	for _, item := range []string{"", "a b\r\n\x00CF.DEL", strings.Repeat("a", 65536)} {
		for _, m := range cuckooMethods(item) {
			t.Run(fmt.Sprintf("%s/%d", m.name, len(item)), func(t *testing.T) {
				f := &cuckooFake{result: m.good}
				c := newCuckooFake(t, f)
				got, err := m.invoke(c, context.Background())
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				switch m.name {
				case "exists", "delete":
					if assertionExpected, assertionActual := true, got; !reflect.DeepEqual(assertionExpected, assertionActual) {
						t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
					}
				case "count":
					if assertionExpected, assertionActual := int64(2), got; !reflect.DeepEqual(assertionExpected, assertionActual) {
						t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
					}
				}
				if assertionLength := len(f.calls); assertionLength != 1 {
					t.Fatalf("length = %d, want %d", assertionLength, 1)
				}
				builder, err := keyBuilderForNamespace("bluetape:probabilistic:cuckoo:v1", "tenant")
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				key, err := structuralKeyValue(builder)
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				want := append([]any{m.args[0], key}, m.args[1:]...)
				if assertionExpected, assertionActual := want, f.calls[0]; !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				hll, err := buildHyperLogLogKey("tenant")
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				bloom, err := buildKeys("tenant")
				if assertionErr := err; assertionErr != nil {
					t.Fatalf("unexpected error: %v", assertionErr)
				}
				if assertionExpected, assertionActual := hll.key, key; reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("NotEqual failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if assertionExpected, assertionActual := bloom.bits, key; reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("NotEqual failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
			})
		}
	}
}

func TestCuckooInputs(t *testing.T) {
	for _, m := range cuckooMethods(strings.Repeat("x", 65537)) {
		if m.name == "reserve" {
			continue
		}
		f := &cuckooFake{result: m.good}
		got, err := m.invoke(newCuckooFake(t, f), context.Background())
		if assertionErr := err; !errors.Is(assertionErr, ErrInvalidOptions) {
			t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrInvalidOptions)
		}
		if assertionExpected, assertionActual := m.zero, got; !reflect.DeepEqual(assertionExpected, assertionActual) {
			t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
		}
		if len(f.calls) != 0 {
			t.Fatal("expected empty captured calls")
		}
	}
	for _, opts := range []CuckooReserveOptions{{}, {Capacity: 3}, {Capacity: 1<<30 + 1}, {Capacity: 4, BucketSize: 3}, {Capacity: 16, Expansion: 32769}} {
		f := &cuckooFake{result: "OK"}
		if assertionErr := newCuckooFake(t, f).Reserve(context.Background(), opts); !errors.Is(assertionErr, ErrInvalidOptions) {
			t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrInvalidOptions)
		}
		if len(f.calls) != 0 {
			t.Fatal("expected empty captured calls")
		}
	}
	for _, opts := range []CuckooReserveOptions{{Capacity: 4}, {Capacity: 1 << 30}, {Capacity: 512, BucketSize: 255, MaxIterations: 65535, Expansion: 32768}} {
		f := &cuckooFake{result: "OK"}
		if assertionErr := newCuckooFake(t, f).Reserve(context.Background(), opts); assertionErr != nil {
			t.Fatalf("unexpected error: %v", assertionErr)
		}
		if assertionLength := len(f.calls); assertionLength != 1 {
			t.Fatalf("length = %d, want %d", assertionLength, 1)
		}
	}
}

type cuckooServerError string

func (e cuckooServerError) Error() string { return string(e) }
func (cuckooServerError) RedisError()     {}

func TestCuckooFailures(t *testing.T) {
	for _, m := range cuckooMethods("payload-secret") {
		unsupported := cuckooServerError(fmt.Sprintf("ERR unknown command '%s', with args beginning with: secret", m.args[0]))
		for _, tc := range []struct {
			name        string
			result      any
			err         error
			nilCmd      bool
			unsupported bool
		}{
			{"unsupported", nil, unsupported, false, true},
			{"typed-wrong-command", nil, cuckooServerError("ERR unknown command 'GET'"), false, false},
			{"text-only", nil, errors.New(unsupported.Error()), false, false},
			{"wrapped", nil, fmt.Errorf("middleware: %w", unsupported), false, false},
			{"joined", nil, errors.Join(unsupported, io.EOF), false, false},
			{"acl", nil, cuckooServerError("NOPERM secret"), false, false},
			{"wrong-type", nil, cuckooServerError("WRONGTYPE secret"), false, false},
			{"transport", nil, io.EOF, false, false},
			{"output-error", m.good, unsupported, false, false},
			{"nil-command", nil, nil, true, false},
		} {
			t.Run(m.name+"/"+tc.name, func(t *testing.T) {
				f := &cuckooFake{result: tc.result, err: tc.err, nilCommand: tc.nilCmd}
				got, err := m.invoke(newCuckooFake(t, f), context.Background())
				if assertionErr := err; assertionErr == nil {
					t.Fatal("expected an error")
				}
				if assertionExpected, assertionActual := m.zero, got; !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if assertionExpected, assertionActual := tc.unsupported, errors.Is(err, ErrCuckooUnsupported); !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if assertionExpected, assertionActual := m.mutation && !tc.unsupported, errors.Is(err, btredis.ErrCommitUnknown); !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if tc.err != nil {
					if assertionErr := err; !errors.Is(assertionErr, tc.err) {
						t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, tc.err)
					}
				} else {
					if assertionErr := err; !errors.Is(assertionErr, ErrCuckooReply) {
						t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrCuckooReply)
					}
				}
				var server redis.Error
				if assertionExpected, assertionActual := errors.As(tc.err, &server), errors.As(err, &server); !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				for _, format := range []string{"%v", "%+v"} {
					output := fmt.Sprintf(format, err)
					for _, s := range []string{"secret", "payload", "tenant"} {
						if strings.Contains(output, s) {
							t.Fatal("sensitive input leaked in public error")
						}
					}
				}
				if assertionLength := len(f.calls); assertionLength != 1 {
					t.Fatalf("length = %d, want %d", assertionLength, 1)
				}
			})
		}
	}
}

func TestCuckooCancellation(t *testing.T) {
	for _, m := range cuckooMethods("item") {
		for _, mode := range []string{"nil", "pre", "deadline", "during", "after", "unsupported-after"} {
			t.Run(m.name+"/"+mode, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				f := &cuckooFake{result: m.good}
				want := context.Canceled
				calls := 0
				switch mode {
				case "nil":
					ctx = nil
					want = ErrInvalidOptions
				case "pre":
					cancel()
				case "deadline":
					var deadlineCancel context.CancelFunc
					ctx, deadlineCancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
					defer deadlineCancel()
					want = context.DeadlineExceeded
				case "during":
					f.after = func() { cancel(); <-ctx.Done() }
					calls = 1
				case "after":
					f.after = cancel
					calls = 1
				case "unsupported-after":
					f.result = nil
					f.err = cuckooServerError(fmt.Sprintf("ERR unknown command '%s'", m.args[0]))
					f.after = cancel
					calls = 1
				}
				got, err := m.invoke(newCuckooFake(t, f), ctx)
				if assertionErr := err; !errors.Is(assertionErr, want) {
					t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, want)
				}
				if assertionExpected, assertionActual := m.zero, got; !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if assertionLength := len(f.calls); assertionLength != calls {
					t.Fatalf("length = %d, want %d", assertionLength, calls)
				}
				if assertionExpected, assertionActual := m.mutation && calls == 1, errors.Is(err, btredis.ErrCommitUnknown); !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if f.err != nil {
					if assertionErr := err; !errors.Is(assertionErr, f.err) {
						t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, f.err)
					}
				}
			})
		}
	}
}

func TestCuckooReplies(t *testing.T) {
	for _, m := range cuckooMethods("item") {
		for _, value := range []any{nil, "1", float64(1), int64(-2), int64(2), []any{}, []any{int64(1), int64(1)}, []any{int64(0)}, []any{cuckooServerError("secret")}} {
			if m.name == "count" && value == int64(2) {
				continue
			}
			t.Run(fmt.Sprintf("%s/%T/%v", m.name, value, value), func(t *testing.T) {
				f := &cuckooFake{result: value}
				got, err := m.invoke(newCuckooFake(t, f), context.Background())
				if assertionErr := err; !errors.Is(assertionErr, ErrCuckooReply) {
					t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrCuckooReply)
				}
				if assertionExpected, assertionActual := m.zero, got; !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
				if assertionExpected, assertionActual := m.mutation, errors.Is(err, btredis.ErrCommitUnknown); !reflect.DeepEqual(assertionExpected, assertionActual) {
					t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
				}
			})
		}
	}
	for _, m := range cuckooMethods("item") {
		if m.name != "exists" && m.name != "delete" {
			continue
		}
		for _, value := range []any{int64(0), int64(1), false, true} {
			got, err := m.invoke(newCuckooFake(t, &cuckooFake{result: value}), context.Background())
			if assertionErr := err; assertionErr != nil {
				t.Fatalf("unexpected error: %v", assertionErr)
			}
			if assertionExpected, assertionActual := value == int64(1) || value == true, got; !reflect.DeepEqual(assertionExpected, assertionActual) {
				t.Fatalf("Equal failed: got %#v, comparison %#v", assertionActual, assertionExpected)
			}
		}
	}
	f := &cuckooFake{result: []any{true}}
	if assertionErr := newCuckooFake(t, f).Add(context.Background(), "item"); assertionErr != nil {
		t.Fatalf("unexpected error: %v", assertionErr)
	}
	for _, value := range []any{int64(-1), false} {
		f = &cuckooFake{result: []any{value}}
		err := newCuckooFake(t, f).Add(context.Background(), "item")
		if assertionErr := err; !errors.Is(assertionErr, ErrCuckooFull) {
			t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrCuckooFull)
		}
		if assertionErr := err; errors.Is(assertionErr, btredis.ErrCommitUnknown) {
			t.Fatalf("error match NotErrorIs failed: %v; target: %v", assertionErr, btredis.ErrCommitUnknown)
		}
		if assertionLength := len(f.calls); assertionLength != 1 {
			t.Fatalf("length = %d, want %d", assertionLength, 1)
		}
	}
}

func (f *cuckooFake) Do(ctx context.Context, args ...any) *redis.Cmd {
	f.mu.Lock()
	f.calls = append(f.calls, append([]any(nil), args...))
	f.mu.Unlock()
	if f.after != nil {
		f.after()
	}
	if f.nilCommand {
		return nil
	}
	cmd := redis.NewCmd(ctx, args...)
	cmd.SetVal(f.result)
	cmd.SetErr(f.err)
	return cmd
}

func TestCuckooConstructor(t *testing.T) {
	var typedNil *cuckooFake
	for _, client := range []CuckooClient{nil, typedNil, cuckooNilMap(nil), cuckooNilSlice(nil), cuckooNilFunc(nil), cuckooNilChan(nil)} {
		got, err := NewCuckoo(CuckooOptions{Client: client, Namespace: "tenant"})
		if assertionErr := err; !errors.Is(assertionErr, ErrInvalidOptions) {
			t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrInvalidOptions)
		}
		if got != nil {
			t.Fatal("expected nil value")
		}
	}
	f := &cuckooFake{}
	for _, ns := range []string{"", " tenant", "tenant ", "tenant{a}", strings.Repeat("a", 129), "secret", "a\n", "é", ":a", "a:", "a:bits", "a:config"} {
		got, err := NewCuckoo(CuckooOptions{Client: f, Namespace: ns})
		if assertionErr := err; !errors.Is(assertionErr, ErrInvalidOptions) {
			t.Fatalf("error match ErrorIs failed: %v; target: %v", assertionErr, ErrInvalidOptions)
		}
		if got != nil {
			t.Fatal("expected nil value")
		}
	}
	for _, ns := range []string{"tenant", "a:._-Z0", strings.Repeat("a", 128)} {
		got, err := NewCuckoo(CuckooOptions{Client: f, Namespace: ns})
		if assertionErr := err; assertionErr != nil {
			t.Fatalf("unexpected error: %v", assertionErr)
		}
		if got == nil {
			t.Fatal("expected non-nil value")
		}
	}
	if len(f.calls) != 0 {
		t.Fatal("expected empty captured calls")
	}
}

func TestCuckooConcurrentFake(t *testing.T) {
	f := &cuckooFake{result: []any{true}}
	c := newCuckooFake(t, f)
	tasks := make([]concurrencytest.Task, 64)
	for i := range tasks {
		item := fmt.Sprintf("item-%d", i)
		tasks[i] = func(ctx context.Context) error { return c.Add(ctx, item) }
	}
	concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 16, Timeout: 5 * time.Second}).RunT(t, tasks...)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != len(tasks) {
		t.Fatalf("calls = %d, want %d", len(f.calls), len(tasks))
	}
	seen := make(map[string]bool, len(tasks))
	for _, call := range f.calls {
		if len(call) != 5 || call[0] != "CF.INSERT" {
			t.Fatalf("unexpected command: %#v", call)
		}
		seen[call[4].(string)] = true
	}
	if len(seen) != len(tasks) {
		t.Fatalf("distinct captured items = %d, want %d", len(seen), len(tasks))
	}
}

func TestCuckooNonCooperativeClient(t *testing.T) {
	for _, m := range cuckooMethods("item") {
		t.Run(m.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered, release := make(chan struct{}), make(chan struct{})
				f := &cuckooFake{result: m.good, after: func() { close(entered); <-release }}
				c := newCuckooFake(t, f)
				done := make(chan error, 1)
				go func() { _, err := m.invoke(c, ctx); done <- err }()
				<-entered
				cancel()
				synctest.Wait()
				select {
				case err := <-done:
					close(release)
					t.Fatalf("adapter detached caller-owned Do: %v", err)
				default:
				}
				close(release)
				synctest.Wait()
				err := <-done
				if !errors.Is(err, context.Canceled) || errors.Is(err, btredis.ErrCommitUnknown) != m.mutation {
					t.Fatalf("late cancellation contract: %v", err)
				}
				if len(f.calls) != 1 {
					t.Fatalf("calls = %d, want 1", len(f.calls))
				}
			})
		})
	}
}
