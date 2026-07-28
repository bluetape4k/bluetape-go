package neo4j

import (
	"context"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j/dbtype"
)

const sessionCloseTimeout = 5 * time.Second

// Client graph IO Neo4j backend에서 제공하는 기능과 사용 경계를 설명한다.
//
// Driver graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
// 이 주석은 graph IO Neo4j backend의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
// 세부 조건은 GraphML, NDJSON, CSV, Neo4j 계약과 caller-owned graph model을 따른다.
type Client struct {
	driver   neo4jdriver.Driver
	database string
}

// NewClient graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewClient(driver neo4jdriver.Driver, options ...Option) (*Client, error) {
	if driver == nil {
		return nil, errorWith(ErrInvalidOptions, "new client", nil)
	}
	cfg, err := applyOptions(options)
	if err != nil {
		return nil, err
	}
	return &Client{driver: driver, database: cfg.database}, nil
}

// VerifyConnectivity graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	if c == nil || c.driver == nil {
		return errorWith(ErrInvalidOptions, "verify connectivity", nil)
	}
	if err := c.driver.VerifyConnectivity(ctx); err != nil {
		return errorWith(ErrDriver, "verify connectivity", err)
	}
	return nil
}

// Close graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
func (c *Client) Close(ctx context.Context) error {
	if c == nil || c.driver == nil {
		return errorWith(ErrInvalidOptions, "close client", nil)
	}
	if err := c.driver.Close(ctx); err != nil {
		return errorWith(ErrDriver, "close client", err)
	}
	return nil
}

// ExecuteWrite graph IO Neo4j backend에서 실행, cancellation, cleanup 계약을 설명한다.
func (c *Client) ExecuteWrite(ctx context.Context, cypher string, params map[string]any) error {
	if c == nil || c.driver == nil {
		return errorWith(ErrInvalidOptions, "execute write", nil)
	}
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{
		AccessMode:   neo4jdriver.AccessModeWrite,
		DatabaseName: c.database,
	})
	defer closeSession(ctx, session)

	_, err := session.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		_, err = result.Consume(ctx)
		return nil, err
	})
	if err != nil {
		return errorWith(ErrDriver, "execute write", err)
	}
	return nil
}

// ReadVertices graph IO Neo4j backend에서 실행, cancellation, cleanup 계약을 설명한다.
func (c *Client) ReadVertices(ctx context.Context, cypher string, params map[string]any, column string) ([]graph.Vertex, error) {
	if c == nil || c.driver == nil {
		return nil, errorWith(ErrInvalidOptions, "read vertices", nil)
	}
	normalizedColumn, err := normalizeColumn(column)
	if err != nil {
		return nil, columnError(ErrInvalidOptions, "read vertices", column, err)
	}
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{
		AccessMode:   neo4jdriver.AccessModeRead,
		DatabaseName: c.database,
	})
	defer closeSession(ctx, session)

	values, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		records, err := result.Collect(ctx)
		if err != nil {
			return nil, err
		}
		return VerticesFromRecords(records, normalizedColumn)
	})
	if err != nil {
		return nil, errorWith(ErrDriver, "read vertices", err)
	}
	return values.([]graph.Vertex), nil
}

// ReadEdges graph IO Neo4j backend에서 실행, cancellation, cleanup 계약을 설명한다.
func (c *Client) ReadEdges(ctx context.Context, cypher string, params map[string]any, column string) ([]graph.Edge, error) {
	if c == nil || c.driver == nil {
		return nil, errorWith(ErrInvalidOptions, "read edges", nil)
	}
	normalizedColumn, err := normalizeColumn(column)
	if err != nil {
		return nil, columnError(ErrInvalidOptions, "read edges", column, err)
	}
	session := c.driver.NewSession(ctx, neo4jdriver.SessionConfig{
		AccessMode:   neo4jdriver.AccessModeRead,
		DatabaseName: c.database,
	})
	defer closeSession(ctx, session)

	values, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		records, err := result.Collect(ctx)
		if err != nil {
			return nil, err
		}
		return EdgesFromRecords(records, normalizedColumn)
	})
	if err != nil {
		return nil, errorWith(ErrDriver, "read edges", err)
	}
	return values.([]graph.Edge), nil
}

// VerticesFromRecords graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func VerticesFromRecords(records []*neo4jdriver.Record, column string) ([]graph.Vertex, error) {
	normalizedColumn, err := normalizeColumn(column)
	if err != nil {
		return nil, columnError(ErrInvalidOptions, "adapt vertices", column, err)
	}
	vertices := make([]graph.Vertex, 0, len(records))
	for _, record := range records {
		node, err := nodeFromRecord(record, normalizedColumn)
		if err != nil {
			return nil, err
		}
		vertex, err := VertexFromNode(node)
		if err != nil {
			return nil, err
		}
		vertices = append(vertices, vertex)
	}
	return vertices, nil
}

// EdgesFromRecords graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func EdgesFromRecords(records []*neo4jdriver.Record, column string) ([]graph.Edge, error) {
	normalizedColumn, err := normalizeColumn(column)
	if err != nil {
		return nil, columnError(ErrInvalidOptions, "adapt edges", column, err)
	}
	edges := make([]graph.Edge, 0, len(records))
	for _, record := range records {
		relationship, err := relationshipFromRecord(record, normalizedColumn)
		if err != nil {
			return nil, err
		}
		edge, err := EdgeFromRelationship(relationship)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

func nodeFromRecord(record *neo4jdriver.Record, column string) (dbtype.Node, error) {
	if record == nil {
		return dbtype.Node{}, columnError(ErrInvalidRecord, "adapt node record", column, nil)
	}
	value, ok := record.Get(column)
	if !ok {
		return dbtype.Node{}, columnError(ErrInvalidRecord, "adapt node record", column, nil)
	}
	node, ok := value.(dbtype.Node)
	if !ok {
		return dbtype.Node{}, columnError(ErrInvalidRecord, "adapt node record", column, nil)
	}
	return node, nil
}

func relationshipFromRecord(record *neo4jdriver.Record, column string) (dbtype.Relationship, error) {
	if record == nil {
		return dbtype.Relationship{}, columnError(ErrInvalidRecord, "adapt relationship record", column, nil)
	}
	value, ok := record.Get(column)
	if !ok {
		return dbtype.Relationship{}, columnError(ErrInvalidRecord, "adapt relationship record", column, nil)
	}
	relationship, ok := value.(dbtype.Relationship)
	if !ok {
		return dbtype.Relationship{}, columnError(ErrInvalidRecord, "adapt relationship record", column, nil)
	}
	return relationship, nil
}

func normalizeColumn(column string) (string, error) {
	normalized := strings.TrimSpace(column)
	if normalized == "" {
		return "", ErrInvalidOptions
	}
	return normalized, nil
}

func closeSession(ctx context.Context, session neo4jdriver.Session) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionCloseTimeout)
	defer cancel()
	_ = session.Close(closeCtx)
}
