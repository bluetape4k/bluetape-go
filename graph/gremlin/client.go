package gremlin

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/bluetape4k/bluetape-go/graph"
)

// ResultStream은 공식 Gremlin-Go result channel을 fake-first 경계로 노출한다.
// 구현체는 Close를 여러 번 호출해도 안전해야 한다.
type ResultStream interface {
	Results() <-chan *gremlingo.Result
	Err() error
	Close()
}

// Executor는 remote Gremlin 제출과 bounded result stream을 caller가 주입하는 경계다.
type Executor interface {
	Submit(traversal string, bindings map[string]any, timeout time.Duration) (ResultStream, error)
}

// Submitter는 Executor의 호환 별칭이다.
type Submitter = Executor

// Result는 remote Gremlin query에서 수집한 defensive top-level values를 보관한다.
type Result struct {
	// Values는 서버가 반환한 순서의 값 복사본이다.
	Values []any
}

// Clone은 result slice를 복사해 caller mutation을 격리한다.
func (r Result) Clone() Result {
	return Result{Values: append([]any(nil), r.Values...)}
}

// Client는 caller-owned Gremlin executor 또는 내부에서 생성한 remote connection을 감싼다.
type Client struct {
	executor   Executor
	close      func()
	maxResults int
	timeout    time.Duration
	closed     atomic.Bool
	closeOnce  sync.Once
}

// NewClient는 caller-owned Executor를 감싸며 Executor를 닫지 않는다.
func NewClient(executor Executor, options ...Option) (*Client, error) {
	if executor == nil {
		return nil, invalid("executor is nil")
	}
	cfg, err := applyOptions(options)
	if err != nil {
		return nil, err
	}
	if len(cfg.connection) > 0 {
		return nil, invalid("connection configuration requires NewRemoteClient")
	}
	return &Client{executor: executor, maxResults: cfg.maxResults, timeout: cfg.timeout}, nil
}

// NewConnectionClient는 caller-owned 공식 DriverRemoteConnection을 감싼다.
func NewConnectionClient(connection *gremlingo.DriverRemoteConnection, options ...Option) (*Client, error) {
	if connection == nil {
		return nil, invalid("remote connection is nil")
	}
	return NewClient(driverExecutor{connection: connection}, options...)
}

// NewRemoteClient는 endpoint와 caller가 제공한 공식 connection 설정으로 adapter를 만든다.
// 반환된 Client가 생성한 connection은 Client.Close에서 닫힌다.
func NewRemoteClient(endpoint string, options ...Option) (*Client, error) {
	if err := validateEndpoint(endpoint); err != nil {
		return nil, err
	}
	cfg, err := applyOptions(options)
	if err != nil {
		return nil, err
	}
	connectionOptions := make([]func(*gremlingo.DriverRemoteConnectionSettings), 0, len(cfg.connection)+1)
	connectionOptions = append(connectionOptions, func(settings *gremlingo.DriverRemoteConnectionSettings) {
		settings.Logger = silentLogger{}
		settings.LogVerbosity = gremlingo.Off
	})
	connectionOptions = append(connectionOptions, cfg.connection...)
	connection, err := gremlingo.NewDriverRemoteConnection(endpoint, connectionOptions...)
	if err != nil {
		return nil, classified(ErrProvider, "create remote connection", err)
	}
	client := &Client{
		executor:   driverExecutor{connection: connection},
		close:      connection.Close,
		maxResults: cfg.maxResults,
		timeout:    cfg.timeout,
	}
	return client, nil
}

// New는 NewRemoteClient의 endpoint 중심 편의 생성자다.
func New(endpoint string, options ...Option) (*Client, error) {
	return NewRemoteClient(endpoint, options...)
}

// VerifyConnectivity는 최소 remote traversal로 연결 상태를 확인한다.
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	_, err := c.Query(ctx, "g.V().limit(0).count()")
	return err
}

// Query는 traversal을 제출하고 context checkpoint를 거쳐 bounded result를 수집한다.
func (c *Client) Query(ctx context.Context, traversal string, bindings ...map[string]any) (Result, error) {
	if err := c.validate(ctx); err != nil {
		return Result{}, err
	}
	if err := validateTraversal(traversal); err != nil {
		return Result{}, err
	}
	normalized, err := normalizeBindings(bindings)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	stream, err := c.executor.Submit(strings.TrimSpace(traversal), normalized, c.effectiveTimeout(ctx))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, ctxErr
		}
		return Result{}, classified(ErrProvider, "submit traversal", err)
	}
	if stream == nil || stream.Results() == nil {
		if stream != nil {
			stream.Close()
		}
		return Result{}, classified(ErrInvalidResult, "nil result stream", nil)
	}
	defer stream.Close()
	results := stream.Results()
	values := make([]any, 0, minInt(c.maxResults, 16))
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case item, ok := <-results:
			if !ok {
				if streamErr := stream.Err(); streamErr != nil {
					if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
						return Result{}, streamErr
					}
					return Result{}, classified(ErrProvider, "read result stream", streamErr)
				}
				if err := ctx.Err(); err != nil {
					return Result{}, err
				}
				return Result{Values: values}, nil
			}
			if item == nil {
				return Result{}, classified(ErrInvalidResult, "nil result", nil)
			}
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			if len(values) >= c.maxResults {
				return Result{}, classified(ErrInvalidResult, "result limit exceeded", nil)
			}
			values = append(values, item.Data)
		}
	}
}

// Execute는 traversal 결과를 버리고 provider 완료만 확인한다.
func (c *Client) Execute(ctx context.Context, traversal string, bindings ...map[string]any) error {
	_, err := c.Query(ctx, traversal, bindings...)
	return err
}

// ReadVertices는 query 결과를 graph.Vertex로 deterministic하게 변환한다.
func (c *Client) ReadVertices(ctx context.Context, traversal string, bindings ...map[string]any) ([]graph.Vertex, error) {
	result, err := c.Query(ctx, traversal, bindings...)
	if err != nil {
		return nil, err
	}
	values, err := expandValues(result.Values, c.maxResults)
	if err != nil {
		return nil, err
	}
	vertices := make([]graph.Vertex, 0, len(values))
	for _, value := range values {
		vertex, err := VertexFromValue(value)
		if err != nil {
			return nil, err
		}
		vertices = append(vertices, vertex)
	}
	return vertices, nil
}

// ReadEdges는 query 결과를 directed graph.Edge로 deterministic하게 변환한다.
func (c *Client) ReadEdges(ctx context.Context, traversal string, bindings ...map[string]any) ([]graph.Edge, error) {
	result, err := c.Query(ctx, traversal, bindings...)
	if err != nil {
		return nil, err
	}
	values, err := expandValues(result.Values, c.maxResults)
	if err != nil {
		return nil, err
	}
	edges := make([]graph.Edge, 0, len(values))
	for _, value := range values {
		edge, err := EdgeFromValue(value)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

// Traverse는 query 결과의 vertex/path 값을 logical key 문자열로 변환한다.
func (c *Client) Traverse(ctx context.Context, traversal string, bindings ...map[string]any) ([]string, error) {
	result, err := c.Query(ctx, traversal, bindings...)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, minInt(len(result.Values), c.maxResults))
	for _, value := range result.Values {
		part, err := traversalKeysBounded(value, c.maxResults-len(keys))
		if err != nil {
			return nil, err
		}
		keys = append(keys, part...)
	}
	return keys, nil
}

// RequireCapability는 지원하지 않는 server-side 기능을 명시적인 typed error로 반환한다.
func (c *Client) RequireCapability(capability string) error {
	if strings.TrimSpace(capability) == "" {
		return classified(ErrInvalidOptions, "capability is blank", nil)
	}
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case "remote-traversal", "vertex-read", "edge-read", "traversal":
		return nil
	default:
		return classified(ErrUnsupportedCapability, "capability "+strings.TrimSpace(capability), nil)
	}
}

// Close는 owned remote connection만 닫고 caller-owned Executor는 건드리지 않는다.
func (c *Client) Close(ctx context.Context) error {
	if c == nil {
		return invalid("close client")
	}
	if ctx == nil {
		return invalid("close context is nil")
	}
	c.closed.Store(true)
	c.closeOnce.Do(func() {
		if c.close != nil {
			c.close()
		}
	})
	return nil
}

func (c *Client) validate(ctx context.Context) error {
	if c == nil || c.executor == nil {
		return invalid("client is nil")
	}
	if c.closed.Load() {
		return classified(ErrClosed, "client is closed", nil)
	}
	if ctx == nil {
		return invalid("context is nil")
	}
	return nil
}

func (c *Client) effectiveTimeout(ctx context.Context) time.Duration {
	timeout := c.timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if timeout <= 0 || remaining < timeout {
			timeout = remaining
		}
	}
	if timeout < 0 {
		return 0
	}
	return timeout
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Fragment != "" {
		return invalid("endpoint is not a ws/wss URL")
	}
	if parsed.User != nil {
		return invalid("endpoint user info must be configured through auth settings")
	}
	return nil
}

func validateTraversal(traversal string) error {
	normalized := strings.TrimSpace(traversal)
	if normalized == "" || len(normalized) > maxQueryBytes || strings.IndexByte(normalized, 0) >= 0 {
		return classified(ErrInvalidQuery, "traversal is empty or too large", nil)
	}
	return nil
}

type driverExecutor struct {
	connection *gremlingo.DriverRemoteConnection
}

type silentLogger struct{}

func (silentLogger) Log(gremlingo.LogVerbosity, ...interface{})          {}
func (silentLogger) Logf(gremlingo.LogVerbosity, string, ...interface{}) {}

func (e driverExecutor) Submit(traversal string, bindings map[string]any, timeout time.Duration) (ResultStream, error) {
	builder := &gremlingo.RequestOptionsBuilder{}
	if bindings != nil {
		builder.SetBindings(bindings)
	}
	if timeout > 0 {
		milliseconds := int(timeout / time.Millisecond)
		if milliseconds < 1 {
			milliseconds = 1
		}
		builder.SetEvaluationTimeout(milliseconds)
	}
	resultSet, err := e.connection.SubmitWithOptions(traversal, builder.Create())
	if err != nil {
		return nil, err
	}
	return resultSetAdapter{resultSet: resultSet}, nil
}

type resultSetAdapter struct {
	resultSet gremlingo.ResultSet
}

func (s resultSetAdapter) Results() <-chan *gremlingo.Result {
	if s.resultSet == nil {
		return nil
	}
	return s.resultSet.Channel()
}

func (s resultSetAdapter) Err() error {
	if s.resultSet == nil {
		return nil
	}
	return s.resultSet.GetError()
}

func (s resultSetAdapter) Close() {
	if s.resultSet != nil {
		s.resultSet.Close()
	}
}

func expandValues(values []any, limit int) ([]any, error) {
	if limit < 1 {
		return nil, classified(ErrInvalidResult, "result limit is invalid", nil)
	}
	expanded := make([]any, 0, minInt(len(values), limit))
	var visit func(any, int) error
	visit = func(value any, depth int) error {
		if depth > maxExpansionDepth {
			return classified(ErrInvalidResult, "nested result is too deep", nil)
		}
		if reflected, ok := sliceValue(value); ok {
			for index := 0; index < reflected.Len(); index++ {
				item := reflected.Index(index)
				if !item.CanInterface() {
					return classified(ErrInvalidResult, "nested result item is inaccessible", nil)
				}
				if err := visit(item.Interface(), depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		if len(expanded) >= limit {
			return classified(ErrInvalidResult, "nested result limit exceeded", nil)
		}
		expanded = append(expanded, value)
		return nil
	}
	for _, value := range values {
		if err := visit(value, 0); err != nil {
			return nil, err
		}
	}
	return expanded, nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
