package falkordb

import (
	"context"
	"time"

	official "github.com/FalkorDB/falkordb-go/v2"
	"github.com/redis/go-redis/v9"

	"github.com/bluetape4k/bluetape-go/graph"
)

// Client는 caller-owned Redis client와 명시적인 graph namespace를 보관한다.
type Client struct {
	conn    redis.UniversalClient
	graph   string
	maxRows int
	timeout time.Duration
}

// Result는 bounded FalkorDB query header, rows, statistics를 보관한다.
type Result struct {
	// Columns는 응답 column name의 복사본이다.
	Columns []string
	// Rows는 응답 row와 scalar/node/edge 값의 defensive 복사본이다.
	Rows [][]any
	// Statistics는 provider statistics 문자열의 복사본이다.
	Statistics []string
}

var _ = official.NewQueryOptions

// NewClient는 caller-owned Redis client와 graph name으로 FalkorDB adapter를 만든다.
func NewClient(conn redis.UniversalClient, graphName string, options ...Option) (*Client, error) {
	if conn == nil {
		return nil, invalid("redis client is nil")
	}
	if !validGraphName(graphName) {
		return nil, invalid("graph name is invalid")
	}
	client := &Client{conn: conn, graph: graphName, maxRows: defaultMaxRows}
	for _, option := range options {
		if option == nil {
			return nil, invalid("nil option")
		}
		if err := option(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// GraphName은 adapter가 사용하는 명시적 graph namespace를 반환한다.
func (c *Client) GraphName() string {
	if c == nil {
		return ""
	}
	return c.graph
}

// VerifyConnectivity는 caller context를 사용해 Redis PING을 실행한다.
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	if err := c.validate(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.conn.Ping(ctx).Err(); err != nil {
		return classified(ErrProvider, err)
	}
	return nil
}

// Close는 caller-owned Redis client를 닫지 않고 adapter ownership을 종료한다.
func (c *Client) Close(context.Context) error {
	if c == nil {
		return classified(ErrInvalidOptions, nil)
	}
	return nil
}

// Query는 GRAPH.QUERY를 context-aware raw Redis 명령으로 실행한다.
func (c *Client) Query(ctx context.Context, query string, params map[string]any) (Result, error) {
	if err := c.validate(ctx); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	command, err := buildQueryCommand(c.graph, query, params, c.timeout)
	if err != nil {
		return Result{}, err
	}
	value, err := c.conn.Do(ctx, command...).Result()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, ctxErr
		}
		return Result{}, classified(ErrProvider, err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	result, err := parseResult(value, c.maxRows)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return result, nil
}

// DeleteGraph는 현재 graph namespace를 삭제하며 shared Redis client는 닫지 않는다.
func (c *Client) DeleteGraph(ctx context.Context) error {
	if err := c.validate(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.conn.Do(ctx, "GRAPH.DELETE", c.graph).Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return classified(ErrProvider, err)
	}
	return nil
}

// ReadVertices는 rows를 graph.Vertex로 제한적으로 변환한다.
func (c *Client) ReadVertices(ctx context.Context, query string, params map[string]any) ([]graph.Vertex, error) {
	result, err := c.Query(ctx, query, params)
	if err != nil {
		return nil, err
	}
	vertices := make([]graph.Vertex, 0, len(result.Rows))
	for _, row := range result.Rows {
		vertex, err := vertexFromRow(row)
		if err != nil {
			return nil, err
		}
		vertices = append(vertices, vertex)
	}
	return vertices, nil
}

// ReadEdges는 rows를 graph.Edge로 제한적으로 변환한다.
func (c *Client) ReadEdges(ctx context.Context, query string, params map[string]any) ([]graph.Edge, error) {
	result, err := c.Query(ctx, query, params)
	if err != nil {
		return nil, err
	}
	edges := make([]graph.Edge, 0, len(result.Rows))
	for _, row := range result.Rows {
		edge, err := edgeFromRow(row)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

func (c *Client) validate(ctx context.Context) error {
	if c == nil || c.conn == nil || !validGraphName(c.graph) {
		return classified(ErrInvalidOptions, nil)
	}
	if ctx == nil {
		return classified(ErrInvalidOptions, nil)
	}
	return nil
}

func serverTimeout(timeout time.Duration) (int, bool) {
	if timeout <= 0 {
		return 0, false
	}
	options := official.NewQueryOptions().SetTimeout(int(timeout.Milliseconds()))
	return options.GetTimeout(), true
}
