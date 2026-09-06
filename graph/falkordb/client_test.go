package falkordb_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph/falkordb"
	"github.com/redis/go-redis/v9"
)

func TestClientQueryBuildsDeterministicContextAwareCommand(t *testing.T) {
	fake := &fakeRedis{value: []interface{}{
		[]interface{}{[]interface{}{int64(1), "id"}},
		[]interface{}{[]interface{}{"v1"}},
		[]interface{}{"Nodes created: 1"},
	}}
	client, err := falkordb.NewClient(fake, "tenant-graph", falkordb.WithTimeout(1500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Query(context.Background(), "RETURN $name", map[string]any{"z": int64(2), "name": "Ada"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !reflect.DeepEqual(result.Columns, []string{"id"}) || len(result.Rows) != 1 {
		t.Fatalf("result=%#v", result)
	}
	want := []any{"GRAPH.QUERY", "tenant-graph", `CYPHER name="Ada" z=2 RETURN $name`, "--compact", "timeout", 1500}
	if !reflect.DeepEqual(fake.args, want) {
		t.Fatalf("args=%#v want=%#v", fake.args, want)
	}
}

func TestClientPreservesCancellationAndRedactsProviderError(t *testing.T) {
	fake := &fakeRedis{err: errors.New("redis://secret:password provider payload")}
	client, err := falkordb.NewClient(fake, "graph")
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Query(canceled, "RETURN 1", nil); !errors.Is(err, context.Canceled) || fake.called {
		t.Fatalf("pre-cancel err=%v called=%t", err, fake.called)
	}
	_, err = client.Query(context.Background(), "RETURN 1", nil)
	if !errors.Is(err, falkordb.ErrProvider) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("provider err=%v", err)
	}
}

func TestClientRejectsMalformedAndOversizedResults(t *testing.T) {
	for _, value := range []any{
		[]interface{}{},
		[]interface{}{[]interface{}{[]interface{}{int64(1)}}},
		[]interface{}{[]interface{}{[]interface{}{int64(1), "id"}}, []interface{}{[]interface{}{}}},
	} {
		fake := &fakeRedis{value: value}
		client, err := falkordb.NewClient(fake, "graph", falkordb.WithMaxRows(1))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Query(context.Background(), "RETURN 1", nil); !errors.Is(err, falkordb.ErrInvalidResult) {
			t.Fatalf("value=%#v err=%v", value, err)
		}
	}
}

type fakeRedis struct {
	redis.UniversalClient
	args   []any
	value  any
	err    error
	called bool
}

func (f *fakeRedis) Do(ctx context.Context, args ...any) *redis.Cmd {
	f.called = true
	f.args = append([]any(nil), args...)
	cmd := redis.NewCmd(ctx, args...)
	if f.err != nil {
		cmd.SetErr(f.err)
	} else {
		cmd.SetVal(f.value)
	}
	return cmd
}
