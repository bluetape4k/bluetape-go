package gremlin

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/bluetape4k/bluetape-go/graph"
)

func TestQueryUsesCheckpointAndDefensiveBindings(t *testing.T) {
	stream := newFakeStream(&gremlingo.Result{Data: int64(2)}, &gremlingo.Result{Data: "ok"})
	fake := &fakeExecutor{stream: stream}
	client, err := NewClient(fake, WithTimeout(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]any{"name": "before"}
	result, err := client.Query(context.Background(), "  g.V().has('name', name)  ", bindings)
	if err != nil {
		t.Fatal(err)
	}
	bindings["name"] = "after"
	if got := result.Values; len(got) != 2 || got[0] != int64(2) || got[1] != "ok" {
		t.Fatalf("unexpected result: %#v", got)
	}
	if fake.query != "g.V().has('name', name)" || fake.bindings["name"] != "before" {
		t.Fatalf("submit arguments = %q %#v", fake.query, fake.bindings)
	}
	if fake.timeout <= 0 || !stream.isClosed() {
		t.Fatalf("timeout=%s streamClosed=%t", fake.timeout, stream.isClosed())
	}
}

func TestQueryPreCanceledDoesNotSubmit(t *testing.T) {
	fake := &fakeExecutor{stream: newFakeStream()}
	client, err := NewClient(fake)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Query(ctx, "g.V()")
	if !errors.Is(err, context.Canceled) || fake.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, fake.calls)
	}
}

func TestQueryInFlightCancellationClosesStreamAndDoesNotPublish(t *testing.T) {
	stream := newFakeStream()
	fake := &fakeExecutor{stream: stream, submitted: make(chan struct{})}
	client, err := NewClient(fake)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan Result, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := client.Query(ctx, "g.V()")
		resultCh <- result
		errCh <- err
	}()
	select {
	case <-fake.submitted:
	case <-time.After(time.Second):
		t.Fatal("query was not submitted")
	}
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("query did not observe cancellation")
	}
	result := <-resultCh
	if result.Values != nil || !stream.isClosed() {
		t.Fatalf("late result published: %#v closed=%t", result.Values, stream.isClosed())
	}
}

func TestQueryRedactsProviderErrorAndBoundsResults(t *testing.T) {
	secret := errors.New("wss://user:password@secret.invalid GREMLIN marker")
	provider := &fakeExecutor{submitErr: secret}
	client, err := NewClient(provider)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "g.V()")
	if !errors.Is(err, ErrProvider) || strings.Contains(err.Error(), "secret.invalid") || strings.Contains(err.Error(), "password") {
		t.Fatalf("provider error was not redacted: %v", err)
	}
	stream := newFakeStream(&gremlingo.Result{Data: 1}, &gremlingo.Result{Data: 2})
	client, err = NewClient(&fakeExecutor{stream: stream}, WithMaxResults(1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "g.V()")
	if !errors.Is(err, ErrInvalidResult) || !stream.closed {
		t.Fatalf("limit error=%v closed=%t", err, stream.closed)
	}
}

func TestClientCloseDoesNotCloseBorrowedExecutor(t *testing.T) {
	fake := &fakeExecutor{stream: newFakeStream()}
	client, err := NewClient(fake)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.closed {
		t.Fatal("borrowed executor was closed")
	}
	_, err = client.Query(context.Background(), "g.V()")
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("query after close error=%v", err)
	}
}

func TestReadVerticesBoundsNestedResults(t *testing.T) {
	client, err := NewClient(&fakeExecutor{stream: newFakeStream(&gremlingo.Result{Data: []any{
		map[string]any{"id": "v1", "label": "person"},
		map[string]any{"id": "v2", "label": "person"},
	}})}, WithMaxResults(1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ReadVertices(context.Background(), "g.V()")
	if !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("nested result error=%v", err)
	}
}

func TestTraverseBoundsNestedResults(t *testing.T) {
	client, err := NewClient(&fakeExecutor{stream: newFakeStream(&gremlingo.Result{Data: []any{
		"left", "right",
	}})}, WithMaxResults(1))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := client.Traverse(context.Background(), "g.V()")
	if !errors.Is(err, ErrInvalidResult) || keys != nil {
		t.Fatalf("keys=%#v nested result error=%v", keys, err)
	}
}

func TestReadVerticesStillConvertsWithinNestedLimit(t *testing.T) {
	client, err := NewClient(&fakeExecutor{stream: newFakeStream(&gremlingo.Result{Data: []any{
		map[string]any{"id": "v1", "label": "person"},
	}})}, WithMaxResults(1))
	if err != nil {
		t.Fatal(err)
	}
	vertices, err := client.ReadVertices(context.Background(), "g.V()")
	if err != nil || len(vertices) != 1 || vertices[0].ID() != graph.MustElementID("v1") {
		t.Fatalf("vertices=%#v err=%v", vertices, err)
	}
}

func TestValidateEndpointAndCapability(t *testing.T) {
	for _, endpoint := range []string{"", "http://localhost:8182/gremlin", "ws://user:pass@localhost:8182/gremlin", "ws://localhost:8182/gremlin#fragment"} {
		if err := validateEndpoint(endpoint); err == nil {
			t.Fatalf("endpoint %q unexpectedly accepted", endpoint)
		}
	}
	if err := validateEndpoint("wss://localhost:8182/gremlin"); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(&fakeExecutor{stream: newFakeStream()})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.RequireCapability("transactions"); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("unsupported capability error=%v", err)
	}
}

type fakeExecutor struct {
	mu        sync.Mutex
	stream    *fakeStream
	submitErr error
	query     string
	bindings  map[string]any
	timeout   time.Duration
	calls     int
	closed    bool
	submitted chan struct{}
}

func (f *fakeExecutor) Submit(query string, bindings map[string]any, timeout time.Duration) (ResultStream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.query = query
	f.bindings = make(map[string]any, len(bindings))
	for key, value := range bindings {
		f.bindings[key] = value
	}
	f.timeout = timeout
	if f.submitted == nil {
		f.submitted = make(chan struct{})
	}
	select {
	case <-f.submitted:
	default:
		close(f.submitted)
	}
	if f.submitErr != nil {
		return nil, f.submitErr
	}
	return f.stream, nil
}

type fakeStream struct {
	results chan *gremlingo.Result
	err     error
	mu      sync.Mutex
	closed  bool
}

func newFakeStream(results ...*gremlingo.Result) *fakeStream {
	stream := &fakeStream{results: make(chan *gremlingo.Result, len(results))}
	for _, result := range results {
		stream.results <- result
	}
	if len(results) > 0 {
		close(stream.results)
	}
	return stream
}

func (s *fakeStream) Results() <-chan *gremlingo.Result { return s.results }
func (s *fakeStream) Err() error                        { return s.err }
func (s *fakeStream) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *fakeStream) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
