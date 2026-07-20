package graph_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/testcontainers/testcontainers-go"
	tcneo4j "github.com/testcontainers/testcontainers-go/modules/neo4j"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	graphProviderBenchmarkEnv     = "BLUETAPE_GRAPH_PROVIDER_BENCH"
	graphProviderStartupLimit     = 90 * time.Second
	graphProviderOperationLimit   = 10 * time.Second
	graphProviderCleanupLimit     = 30 * time.Second
	graphProviderNeo4jImage       = "neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5"
	graphProviderMemgraphImage    = "memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1"
	graphProviderPostgresImage    = "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
	graphProviderMemgraphBoltPort = "7687/tcp"
	graphProviderNeo4jVersion     = "5.26.0"
	graphProviderMemgraphVersion  = "3.5.0"
	graphProviderPostgresVersion  = "16"
	graphProviderVertexLabel      = "BTProviderBenchVertex"
	graphProviderRelationshipType = "BT_PROVIDER_BENCH_EDGE"
)

var graphProviderTraversalSink []string

type traversalEdge struct {
	from string
	to   string
}

type traversalShape struct {
	name        string
	rootID      string
	maxDepth    int
	vertices    []string
	edges       []traversalEdge
	expectedIDs []string
}

func longChainShape(depth, vertices int) traversalShape {
	if depth < 0 || vertices < 1 || depth >= vertices {
		panic("invalid long-chain shape")
	}
	shape := traversalShape{
		name:     fmt.Sprintf("LongChain/Depth%d", depth),
		rootID:   traversalVertexID(0),
		maxDepth: depth,
		vertices: make([]string, vertices),
		edges:    make([]traversalEdge, 0, vertices-1),
	}
	for i := range vertices {
		shape.vertices[i] = traversalVertexID(i)
		if i > 0 {
			shape.edges = append(shape.edges, traversalEdge{from: traversalVertexID(i - 1), to: traversalVertexID(i)})
		}
	}
	shape.expectedIDs = append([]string(nil), shape.vertices[:depth+1]...)
	return shape
}

func deepWideShape(depth, fanout int) traversalShape {
	if depth < 0 || fanout < 1 {
		panic("invalid deep-wide shape")
	}
	total := 1
	levelWidth := 1
	for range depth {
		levelWidth *= fanout
		total += levelWidth
	}
	shape := traversalShape{
		name:        fmt.Sprintf("DeepWide/Depth%dFanout%d", depth, fanout),
		rootID:      traversalVertexID(0),
		maxDepth:    depth,
		vertices:    make([]string, total),
		edges:       make([]traversalEdge, 0, total-1),
		expectedIDs: make([]string, total),
	}
	for i := range total {
		id := traversalVertexID(i)
		shape.vertices[i] = id
		shape.expectedIDs[i] = id
		if i > 0 {
			parent := (i - 1) / fanout
			shape.edges = append(shape.edges, traversalEdge{from: traversalVertexID(parent), to: id})
		}
	}
	return shape
}

func traversalVertexID(index int) string {
	return fmt.Sprintf("v%06d", index)
}

func normalizeTraversalIDs(actual, expected []string) ([]string, error) {
	expectedSet := make(map[string]struct{}, len(expected))
	for _, id := range expected {
		if _, exists := expectedSet[id]; exists {
			return nil, fmt.Errorf("expected traversal IDs contain duplicate %q", id)
		}
		expectedSet[id] = struct{}{}
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, id := range actual {
		if _, exists := actualSet[id]; exists {
			return nil, fmt.Errorf("traversal result contains duplicate %q", id)
		}
		if _, exists := expectedSet[id]; !exists {
			return nil, fmt.Errorf("traversal result contains unexpected ID %q", id)
		}
		actualSet[id] = struct{}{}
	}
	for _, id := range expected {
		if _, exists := actualSet[id]; !exists {
			return nil, fmt.Errorf("traversal result is missing ID %q", id)
		}
	}
	return append([]string(nil), expected...), nil
}

func newGraphProviderRunID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate graph provider run ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

type graphProviderLifecycle struct {
	cleanupData        func(context.Context) error
	closeClient        func(context.Context) error
	terminateContainer func(context.Context) error
}

func runGraphProviderLifecycleCleanup(parent context.Context, timeout time.Duration, lifecycle graphProviderLifecycle) error {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	stages := []struct {
		name string
		run  func(context.Context) error
	}{
		{name: "cleanup data", run: lifecycle.cleanupData},
		{name: "close client", run: lifecycle.closeClient},
		{name: "terminate container", run: lifecycle.terminateContainer},
	}
	var joined error
	for _, stage := range stages {
		if stage.run == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
		err := stage.run(ctx)
		cancel()
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("%s: %w", stage.name, err))
		}
	}
	return joined
}

type traversalRuntime struct {
	name    string
	seed    func(context.Context, traversalShape) error
	prepare func(traversalShape) (preparedTraversal, error)
	cleanup func(context.Context) error
	close   func(context.Context) error
}

type preparedTraversal func(context.Context) ([]string, error)

type graphProviderMetadata struct {
	providerVersion string
	imageReference  string
}

type graphProviderFactory struct {
	name string
	open func(testing.TB) (traversalRuntime, graphProviderMetadata)
}

func graphProviderTraversalCypher(depth int) (string, error) {
	if err := validateGraphProviderTraversalDepth(depth); err != nil {
		return "", err
	}
	return fmt.Sprintf(`
MATCH (root:%s {run: $run, id: $root})
MATCH (root)-[:%s*0..%d]->(n:%s {run: $run})
RETURN DISTINCT n.id AS id
ORDER BY id
`, graphProviderVertexLabel, graphProviderRelationshipType, depth, graphProviderVertexLabel), nil
}

func validateGraphProviderTraversalDepth(depth int) error {
	switch depth {
	case 4, 16, 64:
		return nil
	default:
		return fmt.Errorf("unreviewed graph traversal depth %d", depth)
	}
}

const graphProviderSeedVerticesCypher = `
UNWIND $vertices AS id
CREATE (:%s {run: $run, id: id})
`

const graphProviderSeedEdgesCypher = `
UNWIND $edges AS edge
MATCH (source:%s {run: $run, id: edge.source})
MATCH (target:%s {run: $run, id: edge.target})
CREATE (source)-[:%s {run: $run}]->(target)
`

func BenchmarkGraphProviderTraversalContainers(b *testing.B) {
	if os.Getenv(graphProviderBenchmarkEnv) != "1" {
		b.Skipf("set %s=1 to run serial Testcontainers-backed graph provider benchmarks", graphProviderBenchmarkEnv)
	}

	shapes := []traversalShape{
		longChainShape(16, 64),
		longChainShape(64, 128),
		deepWideShape(4, 4),
	}
	factories := []graphProviderFactory{
		{name: "Neo4j", open: newNeo4jTraversalRuntime},
		{name: "Memgraph", open: newMemgraphTraversalRuntime},
		{name: "PostgreSQLRecursiveCTE", open: newPostgresTraversalRuntime},
	}
	for _, factory := range factories {
		b.Run(factory.name, func(b *testing.B) {
			runtime, metadata := factory.open(b)
			b.Logf("provider_version=%q image_reference=%q", sanitizeGraphProviderMetadata(metadata.providerVersion), sanitizeGraphProviderMetadata(metadata.imageReference))
			for _, shape := range shapes {
				b.Run(shape.name, func(b *testing.B) {
					benchmarkTraversalRuntime(b, runtime, shape)
				})
			}
		})
	}
}

func benchmarkTraversalRuntime(b *testing.B, runtime traversalRuntime, shape traversalShape) {
	b.Helper()
	setupCtx, setupCancel := context.WithTimeout(context.Background(), graphProviderStartupLimit)
	if err := runtime.seed(setupCtx, shape); err != nil {
		setupCancel()
		b.Fatalf("seed %s on %s: %v", shape.name, runtime.name, err)
	}
	setupCancel()
	prepared, err := runtime.prepare(shape)
	if err != nil {
		b.Fatalf("prepare %s on %s: %v", shape.name, runtime.name, err)
	}

	preflightCtx, preflightCancel := graphProviderOperationContext()
	preflightIDs, err := prepared(preflightCtx)
	preflightCancel()
	if err != nil {
		b.Fatalf("preflight %s on %s: %v", shape.name, runtime.name, err)
	}
	if _, err := normalizeTraversalIDs(preflightIDs, shape.expectedIDs); err != nil {
		b.Fatalf("preflight %s on %s: %v", shape.name, runtime.name, err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var last []string
	for b.Loop() {
		opCtx, cancel := graphProviderOperationContext()
		last, err = prepared(opCtx)
		cancel()
		if err != nil {
			b.Fatalf("query %s on %s: %v", shape.name, runtime.name, err)
		}
		graphProviderTraversalSink = last
	}
	b.StopTimer()
	if _, err := normalizeTraversalIDs(last, shape.expectedIDs); err != nil {
		b.Fatalf("final result %s on %s: %v", shape.name, runtime.name, err)
	}
}

func newNeo4jTraversalRuntime(tb testing.TB) (traversalRuntime, graphProviderMetadata) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), graphProviderStartupLimit)
	defer cancel()
	container, err := tcneo4j.Run(ctx, graphProviderNeo4jImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("neo4j", graphProviderNeo4jImage, err))
	}
	return newCypherTraversalRuntime(ctx, tb, "Neo4j", graphProviderNeo4jImage, graphProviderNeo4jVersion, container, func(ctx context.Context) (string, error) {
		return container.BoltUrl(ctx)
	}, false)
}

func newMemgraphTraversalRuntime(tb testing.TB) (traversalRuntime, graphProviderMetadata) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), graphProviderStartupLimit)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        graphProviderMemgraphImage,
			ExposedPorts: []string{graphProviderMemgraphBoltPort},
			WaitingFor:   wait.ForListeningPort(graphProviderMemgraphBoltPort).WithStartupTimeout(graphProviderStartupLimit),
		},
		Started: true,
	})
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("memgraph", graphProviderMemgraphImage, err))
	}
	return newCypherTraversalRuntime(ctx, tb, "Memgraph", graphProviderMemgraphImage, graphProviderMemgraphVersion, container, func(ctx context.Context) (string, error) {
		return container.PortEndpoint(ctx, graphProviderMemgraphBoltPort, "bolt")
	}, true)
}

func newCypherTraversalRuntime(
	ctx context.Context,
	tb testing.TB,
	name string,
	imageReference string,
	versionAuthority string,
	container testcontainers.Container,
	endpoint func(context.Context) (string, error),
	memgraph bool,
) (traversalRuntime, graphProviderMetadata) {
	tb.Helper()
	var driver neo4jdriver.Driver
	var runID string
	lifecycle := graphProviderLifecycle{
		cleanupData: func(cleanupCtx context.Context) error {
			if driver == nil || runID == "" {
				return nil
			}
			return executeCypherWrite(cleanupCtx, driver, fmt.Sprintf(`
MATCH (n:%s {run: $run})
DETACH DELETE n
`, graphProviderVertexLabel), map[string]any{"run": runID})
		},
		closeClient: func(closeCtx context.Context) error {
			if driver == nil {
				return nil
			}
			return driver.Close(closeCtx)
		},
		terminateContainer: func(terminateCtx context.Context) error {
			return container.Terminate(terminateCtx)
		},
	}
	registerGraphProviderLifecycleCleanup(ctx, tb, name, lifecycle)

	boltURL, err := endpoint(ctx)
	if err != nil {
		tb.Fatalf("%s Bolt endpoint: %v", name, err)
	}
	driver, err = neo4jdriver.NewDriver(boltURL, neo4jdriver.NoAuth())
	if err != nil {
		tb.Fatalf("new %s driver: %v", name, err)
	}
	if err := waitForGraphProviderConnectivity(ctx, driver); err != nil {
		tb.Fatalf("verify %s connectivity: %v", name, err)
	}
	runID, err = newGraphProviderRunID()
	if err != nil {
		tb.Fatal(err)
	}
	providerVersion, err := cypherProviderVersion(ctx, driver, memgraph)
	if err != nil {
		tb.Fatalf("query %s provider version: %v", name, err)
	}
	if !graphProviderVersionMatchesAuthority(providerVersion, versionAuthority) {
		tb.Fatalf("%s provider version %q does not match pinned image authority %q", name, sanitizeGraphProviderMetadata(providerVersion), versionAuthority)
	}

	runtime := traversalRuntime{name: name}
	runtime.cleanup = lifecycle.cleanupData
	runtime.close = lifecycle.closeClient
	runtime.seed = func(seedCtx context.Context, shape traversalShape) error {
		if err := runtime.cleanup(seedCtx); err != nil {
			return fmt.Errorf("clear prior graph data: %w", err)
		}
		if err := executeCypherWrite(seedCtx, driver, fmt.Sprintf(graphProviderSeedVerticesCypher, graphProviderVertexLabel), map[string]any{
			"run":      runID,
			"vertices": shape.vertices,
		}); err != nil {
			return fmt.Errorf("seed vertices: %w", err)
		}
		edges := make([]map[string]any, len(shape.edges))
		for i, edge := range shape.edges {
			edges[i] = map[string]any{"source": edge.from, "target": edge.to}
		}
		if err := executeCypherWrite(seedCtx, driver, fmt.Sprintf(graphProviderSeedEdgesCypher, graphProviderVertexLabel, graphProviderVertexLabel, graphProviderRelationshipType), map[string]any{
			"run":   runID,
			"edges": edges,
		}); err != nil {
			return fmt.Errorf("seed edges: %w", err)
		}
		return nil
	}
	runtime.prepare = func(shape traversalShape) (preparedTraversal, error) {
		query, err := graphProviderTraversalCypher(shape.maxDepth)
		if err != nil {
			return nil, err
		}
		params := map[string]any{"run": runID, "root": shape.rootID}
		return func(queryCtx context.Context) ([]string, error) {
			return collectCypherStrings(queryCtx, driver, query, params, "id")
		}, nil
	}
	return runtime, graphProviderMetadata{providerVersion: providerVersion, imageReference: imageReference}
}

func newPostgresTraversalRuntime(tb testing.TB) (traversalRuntime, graphProviderMetadata) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), graphProviderStartupLimit)
	defer cancel()
	server := postgrestestcontainer.StartServer(ctx, tb)
	details, err := server.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("PostgreSQL connection details: %v", err)
	}
	dsn, err := details.Require(postgrestestcontainer.ConnectionStringKey)
	if err != nil {
		tb.Fatalf("PostgreSQL connection string: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		tb.Fatalf("open PostgreSQL graph provider: %v", err)
	}
	var schema string
	lifecycle := graphProviderLifecycle{
		cleanupData: func(cleanupCtx context.Context) error {
			if schema == "" {
				return nil
			}
			_, err := db.ExecContext(cleanupCtx, `drop schema if exists `+schema+` cascade`)
			return err
		},
		closeClient: func(context.Context) error { return db.Close() },
	}
	registerGraphProviderLifecycleCleanup(ctx, tb, "PostgreSQLRecursiveCTE", lifecycle)
	runID, err := newGraphProviderRunID()
	if err != nil {
		tb.Fatal(err)
	}
	schema, err = graphProviderSchemaName(runID)
	if err != nil {
		tb.Fatal(err)
	}
	if err := db.PingContext(ctx); err != nil {
		tb.Fatalf("ping PostgreSQL graph provider: %v", err)
	}
	if _, err := db.ExecContext(ctx, `create schema `+schema); err != nil {
		tb.Fatalf("create PostgreSQL graph schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
create table %s.vertices (
    id text primary key
);
create table %s.edges (
    from_id text not null references %s.vertices(id),
    to_id text not null references %s.vertices(id),
    primary key (from_id, to_id)
);
create index edges_from_id_idx on %s.edges(from_id);
`, schema, schema, schema, schema, schema)); err != nil {
		tb.Fatalf("create PostgreSQL graph tables: %v", err)
	}
	var providerVersion string
	if err := db.QueryRowContext(ctx, `show server_version`).Scan(&providerVersion); err != nil {
		tb.Fatalf("query PostgreSQL provider version: %v", err)
	}
	providerVersion = strings.TrimSpace(providerVersion)
	if !graphProviderVersionMatchesAuthority(providerVersion, graphProviderPostgresVersion) {
		tb.Fatalf("PostgreSQL provider version %q does not match pinned image authority %q", sanitizeGraphProviderMetadata(providerVersion), graphProviderPostgresVersion)
	}

	runtime := traversalRuntime{name: "PostgreSQLRecursiveCTE", cleanup: lifecycle.cleanupData, close: lifecycle.closeClient}
	runtime.seed = func(seedCtx context.Context, shape traversalShape) (resultErr error) {
		tx, err := db.BeginTx(seedCtx, nil)
		if err != nil {
			return fmt.Errorf("begin seed transaction: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				if rollbackErr := tx.Rollback(); rollbackErr != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("rollback seed transaction: %w", rollbackErr))
				}
			}
		}()
		if _, err := tx.ExecContext(seedCtx, `delete from `+schema+`.edges`); err != nil {
			return fmt.Errorf("clear edges: %w", err)
		}
		if _, err := tx.ExecContext(seedCtx, `delete from `+schema+`.vertices`); err != nil {
			return fmt.Errorf("clear vertices: %w", err)
		}
		for _, id := range shape.vertices {
			if _, err := tx.ExecContext(seedCtx, `insert into `+schema+`.vertices(id) values ($1)`, id); err != nil {
				return fmt.Errorf("seed vertex: %w", err)
			}
		}
		for _, edge := range shape.edges {
			if _, err := tx.ExecContext(seedCtx, `insert into `+schema+`.edges(from_id, to_id) values ($1, $2)`, edge.from, edge.to); err != nil {
				return fmt.Errorf("seed edge: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit seed transaction: %w", err)
		}
		committed = true
		return nil
	}
	runtime.prepare = func(shape traversalShape) (preparedTraversal, error) {
		if err := validateGraphProviderTraversalDepth(shape.maxDepth); err != nil {
			return nil, err
		}
		query := fmt.Sprintf(`
with recursive reachable(id, depth, path) as (
    select id, 0, array[id]
    from %s.vertices
    where id = $1
    union all
    select edge.to_id, reachable.depth + 1, reachable.path || edge.to_id
    from reachable
    join %s.edges edge on edge.from_id = reachable.id
    where reachable.depth < $2
      and not edge.to_id = any(reachable.path)
)
select distinct id
from reachable
order by id
		`, schema, schema)
		rootID := shape.rootID
		maxDepth := shape.maxDepth
		resultCapacity := len(shape.expectedIDs)
		return func(queryCtx context.Context) (_ []string, resultErr error) {
			rows, err := db.QueryContext(queryCtx, query, rootID, maxDepth)
			if err != nil {
				return nil, fmt.Errorf("query recursive traversal: %w", err)
			}
			defer func() {
				if closeErr := rows.Close(); closeErr != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("close recursive traversal rows: %w", closeErr))
				}
			}()
			ids := make([]string, 0, resultCapacity)
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return nil, fmt.Errorf("scan recursive traversal: %w", err)
				}
				ids = append(ids, id)
			}
			if err := rows.Err(); err != nil {
				return nil, fmt.Errorf("iterate recursive traversal: %w", err)
			}
			return ids, nil
		}, nil
	}
	return runtime, graphProviderMetadata{providerVersion: providerVersion, imageReference: graphProviderPostgresImage}
}

func graphProviderSchemaName(runID string) (string, error) {
	if !regexp.MustCompile(`^[0-9a-f]{1,32}$`).MatchString(runID) {
		return "", errors.New("graph provider run ID must be lowercase hexadecimal")
	}
	return `"btbench_` + runID + `"`, nil
}

func registerGraphProviderLifecycleCleanup(parent context.Context, tb testing.TB, name string, lifecycle graphProviderLifecycle) {
	tb.Helper()
	tb.Cleanup(func() {
		if err := runGraphProviderLifecycleCleanup(parent, graphProviderCleanupLimit, lifecycle); err != nil {
			tb.Errorf("cleanup %s graph provider: %v", name, err)
		}
	})
}

func graphProviderOperationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), graphProviderOperationLimit)
}

func executeCypherWrite(ctx context.Context, driver neo4jdriver.Driver, query string, params map[string]any) (resultErr error) {
	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite})
	defer func() {
		resultErr = errors.Join(resultErr, closeGraphProviderSession(ctx, session))
	}()
	_, err := session.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		_, err = result.Consume(ctx)
		return nil, err
	})
	return err
}

func collectCypherStrings(ctx context.Context, driver neo4jdriver.Driver, query string, params map[string]any, column string) (_ []string, resultErr error) {
	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead})
	defer func() {
		resultErr = errors.Join(resultErr, closeGraphProviderSession(ctx, session))
	}()
	value, err := session.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		records, err := result.Collect(ctx)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(records))
		for _, record := range records {
			raw, ok := record.Get(column)
			if !ok {
				return nil, fmt.Errorf("Cypher result has no %q column", column)
			}
			id, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("Cypher result %q column has type %T", column, raw)
			}
			ids = append(ids, id)
		}
		return ids, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]string), nil
}

func closeGraphProviderSession(parent context.Context, session neo4jdriver.Session) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), graphProviderOperationLimit)
	defer cancel()
	return session.Close(ctx)
}

func waitForGraphProviderConnectivity(ctx context.Context, driver neo4jdriver.Driver) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, graphProviderOperationLimit)
		lastErr = driver.VerifyConnectivity(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func cypherProviderVersion(ctx context.Context, driver neo4jdriver.Driver, memgraph bool) (string, error) {
	if !memgraph {
		info, err := driver.GetServerInfo(ctx)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(info.Agent()), nil
	}
	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead})
	result, err := session.Run(ctx, "SHOW VERSION", nil)
	if err != nil {
		return "", errors.Join(err, closeGraphProviderSession(ctx, session))
	}
	records, collectErr := result.Collect(ctx)
	closeErr := closeGraphProviderSession(ctx, session)
	if err := errors.Join(collectErr, closeErr); err != nil {
		return "", err
	}
	if len(records) != 1 || len(records[0].Values) != 1 {
		return "", fmt.Errorf("SHOW VERSION returned %d rows with unexpected columns", len(records))
	}
	version, ok := records[0].Values[0].(string)
	if !ok {
		return "", fmt.Errorf("SHOW VERSION returned type %T, want string", records[0].Values[0])
	}
	return strings.TrimSpace(version), nil
}

func graphProviderVersionMatchesAuthority(reported, authority string) bool {
	reported = strings.TrimSpace(reported)
	authority = strings.TrimSpace(authority)
	if reported == "" || authority == "" {
		return false
	}
	versionPattern := regexp.MustCompile(`(?:^|[^0-9])` + regexp.QuoteMeta(authority) + `(?:$|[^0-9])`)
	return versionPattern.MatchString(reported)
}

func sanitizeGraphProviderMetadata(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

func TestTraversalShapesHaveExpectedReachability(t *testing.T) {
	tests := []struct {
		shape        traversalShape
		wantIDs      int
		wantVertices int
	}{
		{shape: longChainShape(16, 64), wantIDs: 17, wantVertices: 64},
		{shape: longChainShape(64, 128), wantIDs: 65, wantVertices: 128},
		{shape: deepWideShape(4, 4), wantIDs: 341, wantVertices: 341},
	}
	for _, tt := range tests {
		t.Run(tt.shape.name, func(t *testing.T) {
			if got := len(tt.shape.expectedIDs); got != tt.wantIDs {
				t.Fatalf("expected IDs = %d, want %d", got, tt.wantIDs)
			}
			if got := len(tt.shape.vertices); got != tt.wantVertices {
				t.Fatalf("vertices = %d, want %d", got, tt.wantVertices)
			}
			if got := len(tt.shape.edges); got != tt.wantVertices-1 {
				t.Fatalf("edges = %d, want %d", got, tt.wantVertices-1)
			}
			if len(tt.shape.vertices) == 0 || tt.shape.rootID != tt.shape.vertices[0] {
				t.Fatalf("root = %q, vertices = %#v", tt.shape.rootID, tt.shape.vertices)
			}
			actual := traversalIDsFromSeed(tt.shape)
			if _, err := normalizeTraversalIDs(actual, tt.shape.expectedIDs); err != nil {
				t.Fatalf("seed reachability: %v", err)
			}
		})
	}
}

func TestGraphProviderImagesAreImmutable(t *testing.T) {
	immutable := regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`)
	for name, image := range map[string]string{
		"neo4j":      graphProviderNeo4jImage,
		"memgraph":   graphProviderMemgraphImage,
		"postgresql": graphProviderPostgresImage,
	} {
		if !immutable.MatchString(image) {
			t.Fatalf("%s image = %q, want immutable digest", name, image)
		}
	}
}

func traversalIDsFromSeed(shape traversalShape) []string {
	adjacent := make(map[string][]string, len(shape.vertices))
	for _, edge := range shape.edges {
		adjacent[edge.from] = append(adjacent[edge.from], edge.to)
	}
	type visit struct {
		id    string
		depth int
	}
	queue := []visit{{id: shape.rootID}}
	seen := make(map[string]struct{}, len(shape.expectedIDs))
	ids := make([]string, 0, len(shape.expectedIDs))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := seen[current.id]; exists {
			continue
		}
		seen[current.id] = struct{}{}
		ids = append(ids, current.id)
		if current.depth == shape.maxDepth {
			continue
		}
		for _, next := range adjacent[current.id] {
			queue = append(queue, visit{id: next, depth: current.depth + 1})
		}
	}
	return ids
}

func TestNormalizeTraversalIDsRejectsInvalidRowsAndOrdersExpectedIDs(t *testing.T) {
	expected := []string{"v000", "v001", "v002"}
	got, err := normalizeTraversalIDs([]string{"v002", "v000", "v001"}, expected)
	if err != nil {
		t.Fatalf("normalize valid IDs: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("normalized IDs = %#v, want %#v", got, expected)
	}

	tests := []struct {
		name string
		got  []string
	}{
		{name: "duplicate", got: []string{"v000", "v000", "v001", "v002"}},
		{name: "missing", got: []string{"v000", "v002"}},
		{name: "extra", got: []string{"v000", "v001", "v002", "v003"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := normalizeTraversalIDs(tt.got, expected); err == nil {
				t.Fatal("expected normalization error")
			}
		})
	}
}

func TestGraphProviderRunIDIsLowercaseHex(t *testing.T) {
	runID, err := newGraphProviderRunID()
	if err != nil {
		t.Fatalf("new run ID: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(runID) {
		t.Fatalf("run ID = %q, want 32 lowercase hexadecimal characters", runID)
	}
}

func TestGraphProviderTraversalCypherUsesReviewedLiteralDepths(t *testing.T) {
	for _, depth := range []int{4, 16, 64} {
		query, err := graphProviderTraversalCypher(depth)
		if err != nil {
			t.Fatalf("depth %d: %v", depth, err)
		}
		if !strings.Contains(query, fmt.Sprintf("*0..%d", depth)) {
			t.Fatalf("depth %d query = %q", depth, query)
		}
	}
	if _, err := graphProviderTraversalCypher(17); err == nil {
		t.Fatal("expected unreviewed literal depth error")
	}
}

func TestGraphProviderVersionAuthorityRequiresNumericBoundary(t *testing.T) {
	tests := []struct {
		reported  string
		authority string
		want      bool
	}{
		{reported: "Neo4j/5.26.0", authority: "5.26.0", want: true},
		{reported: "Memgraph/3.5.0", authority: "3.5.0", want: true},
		{reported: "16.10 (Debian)", authority: "16", want: true},
		{reported: "Neo4j/5.26.01", authority: "5.26.0", want: false},
		{reported: "Neo4j/25.26.0", authority: "5.26.0", want: false},
	}
	for _, tt := range tests {
		if got := graphProviderVersionMatchesAuthority(tt.reported, tt.authority); got != tt.want {
			t.Fatalf("version match (%q, %q) = %v, want %v", tt.reported, tt.authority, got, tt.want)
		}
	}
}

func TestGraphProviderSchemaNameRejectsUntrustedIdentifiers(t *testing.T) {
	got, err := graphProviderSchemaName("0123456789abcdef")
	if err != nil {
		t.Fatalf("valid schema: %v", err)
	}
	if got != `"btbench_0123456789abcdef"` {
		t.Fatalf("schema = %q", got)
	}
	for _, invalid := range []string{"", "ABC", "../schema", "abcd;drop schema public", strings.Repeat("a", 33)} {
		if _, err := graphProviderSchemaName(invalid); err == nil {
			t.Fatalf("expected invalid schema error for %q", invalid)
		}
	}
}

func TestBenchmarkTraversalRuntimePreparesOnceBeforePreflightAndTimedQueries(t *testing.T) {
	result := testing.Benchmark(func(b *testing.B) {
		shape := longChainShape(4, 8)
		prepareCalls := 0
		queryCalls := 0
		runtime := traversalRuntime{
			name: "fake",
			seed: func(context.Context, traversalShape) error { return nil },
			prepare: func(got traversalShape) (preparedTraversal, error) {
				prepareCalls++
				if got.name != shape.name {
					return nil, fmt.Errorf("prepared shape = %q, want %q", got.name, shape.name)
				}
				return func(context.Context) ([]string, error) {
					queryCalls++
					return append([]string(nil), shape.expectedIDs...), nil
				}, nil
			},
		}
		benchmarkTraversalRuntime(b, runtime, shape)
		if prepareCalls != 1 {
			b.Fatalf("prepare calls = %d, want 1", prepareCalls)
		}
		if queryCalls != b.N+1 {
			b.Fatalf("prepared query calls = %d, want preflight + b.N = %d", queryCalls, b.N+1)
		}
	})
	if result.N < 1 {
		t.Fatalf("benchmark iterations = %d, want at least 1", result.N)
	}
}

func TestGraphProviderLifecycleCleanupRunsEveryStageInOrderAndJoinsErrors(t *testing.T) {
	wantData := errors.New("data cleanup failed")
	wantClose := errors.New("client close failed")
	wantTerminate := errors.New("container termination failed")
	var calls []string
	lifecycle := graphProviderLifecycle{
		cleanupData: func(context.Context) error {
			calls = append(calls, "data")
			return wantData
		},
		closeClient: func(context.Context) error {
			calls = append(calls, "client")
			return wantClose
		},
		terminateContainer: func(context.Context) error {
			calls = append(calls, "container")
			return wantTerminate
		},
	}

	err := runGraphProviderLifecycleCleanup(context.Background(), time.Second, lifecycle)
	if !reflect.DeepEqual(calls, []string{"data", "client", "container"}) {
		t.Fatalf("cleanup calls = %#v", calls)
	}
	for _, want := range []error{wantData, wantClose, wantTerminate} {
		if !errors.Is(err, want) {
			t.Fatalf("cleanup error = %v, want joined %v", err, want)
		}
	}
}

func TestGraphProviderLifecycleCleanupBoundsEveryStageIndependently(t *testing.T) {
	var calls []string
	block := func(name string) func(context.Context) error {
		return func(ctx context.Context) error {
			calls = append(calls, name)
			<-ctx.Done()
			return ctx.Err()
		}
	}

	started := time.Now()
	err := runGraphProviderLifecycleCleanup(context.Background(), 10*time.Millisecond, graphProviderLifecycle{
		cleanupData:        block("data"),
		closeClient:        block("client"),
		terminateContainer: block("container"),
	})
	if err == nil {
		t.Fatal("expected bounded cleanup errors")
	}
	if !reflect.DeepEqual(calls, []string{"data", "client", "container"}) {
		t.Fatalf("cleanup calls = %#v", calls)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded cleanup took %s", elapsed)
	}
}

func TestGraphProviderLifecycleCleanupIgnoresCanceledParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := runGraphProviderLifecycleCleanup(parent, time.Second, graphProviderLifecycle{
		cleanupData: func(ctx context.Context) error {
			called = true
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("cleanup with canceled parent: %v", err)
	}
	if !called {
		t.Fatal("cleanup stage was not called")
	}
}
