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

// Client provides small query helpers over a caller-owned Neo4j driver.
//
// Driver is safe for concurrent use, and Client holds only immutable
// configuration. Callers must still avoid closing the driver while operations
// are in flight.
type Client struct {
	driver   neo4jdriver.Driver
	database string
}

// NewClient creates a Client around a caller-owned Neo4j driver.
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

// VerifyConnectivity checks that the underlying driver can reach Neo4j.
func (c *Client) VerifyConnectivity(ctx context.Context) error {
	if c == nil || c.driver == nil {
		return errorWith(ErrInvalidOptions, "verify connectivity", nil)
	}
	if err := c.driver.VerifyConnectivity(ctx); err != nil {
		return errorWith(ErrDriver, "verify connectivity", err)
	}
	return nil
}

// Close closes the caller-owned driver through this client.
func (c *Client) Close(ctx context.Context) error {
	if c == nil || c.driver == nil {
		return errorWith(ErrInvalidOptions, "close client", nil)
	}
	if err := c.driver.Close(ctx); err != nil {
		return errorWith(ErrDriver, "close client", err)
	}
	return nil
}

// ExecuteWrite runs a write query and consumes the result summary.
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

// ReadVertices runs a read query and adapts the named result column to vertices.
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

// ReadEdges runs a read query and adapts the named result column to edges.
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

// VerticesFromRecords adapts the named column from each record to vertices.
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

// EdgesFromRecords adapts the named column from each record to edges.
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
