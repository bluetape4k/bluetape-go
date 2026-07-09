package neo4j_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	neo4jadapter "github.com/bluetape4k/bluetape-go/graph/neo4j"
	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j/dbtype"
	"github.com/testcontainers/testcontainers-go"
	tcneo4j "github.com/testcontainers/testcontainers-go/modules/neo4j"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	graphNeo4jBenchEnv           = "BLUETAPE_GRAPH_NEO4J_BENCH"
	graphBenchmarkOperationLimit = 10 * time.Second
)

func BenchmarkVertexFromNode(b *testing.B) {
	node := dbtype.Node{
		ElementId: "node-1",
		Labels:    []string{"Service", " API ", "Service", "Checkout"},
		Props: map[string]any{
			"name":    "checkout",
			"version": int64(7),
			"active":  true,
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		vertex, err := neo4jadapter.VertexFromNode(node)
		if err != nil {
			b.Fatal(err)
		}
		if vertex.ID().String() == "" {
			b.Fatal("empty vertex id")
		}
	}
}

func BenchmarkEdgeFromRelationship(b *testing.B) {
	relationship := dbtype.Relationship{
		ElementId:      "rel-1",
		StartElementId: "node-1",
		EndElementId:   "node-2",
		Type:           "CALLS",
		Props: map[string]any{
			"weight": int64(7),
			"kind":   "sync",
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		edge, err := neo4jadapter.EdgeFromRelationship(relationship)
		if err != nil {
			b.Fatal(err)
		}
		if edge.StartID().String() == "" || edge.EndID().String() == "" {
			b.Fatal("empty edge endpoint")
		}
	}
}

func BenchmarkVerticesFromRecords(b *testing.B) {
	records := make([]*neo4jdriver.Record, 100)
	for i := range records {
		records[i] = &neo4jdriver.Record{
			Keys: []string{"n"},
			Values: []any{
				dbtype.Node{
					ElementId: fmt.Sprintf("node-%d", i),
					Labels:    []string{"Service", "Benchmark"},
					Props:     map[string]any{"seq": int64(i), "name": fmt.Sprintf("service-%d", i)},
				},
			},
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		vertices, err := neo4jadapter.VerticesFromRecords(records, "n")
		if err != nil {
			b.Fatal(err)
		}
		if len(vertices) != len(records) {
			b.Fatalf("vertices len = %d, want %d", len(vertices), len(records))
		}
	}
}

func BenchmarkEdgesFromRecords(b *testing.B) {
	records := make([]*neo4jdriver.Record, 100)
	for i := range records {
		records[i] = &neo4jdriver.Record{
			Keys: []string{"r"},
			Values: []any{
				dbtype.Relationship{
					ElementId:      fmt.Sprintf("rel-%d", i),
					StartElementId: fmt.Sprintf("node-%d", i),
					EndElementId:   fmt.Sprintf("node-%d", i+1),
					Type:           "CALLS",
					Props:          map[string]any{"weight": int64(i)},
				},
			},
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		edges, err := neo4jadapter.EdgesFromRecords(records, "r")
		if err != nil {
			b.Fatal(err)
		}
		if len(edges) != len(records) {
			b.Fatalf("edges len = %d, want %d", len(edges), len(records))
		}
	}
}

func BenchmarkGraphNeo4jContainers(b *testing.B) {
	if os.Getenv(graphNeo4jBenchEnv) != "1" {
		b.Skipf("set %s=1 to run serial Testcontainers-backed Neo4j/Memgraph benchmarks", graphNeo4jBenchEnv)
	}

	b.Run("Neo4j/neo4j:5.26.0", func(b *testing.B) {
		ctx := context.Background()
		driver := startNeo4jBenchmarkDriver(ctx, b)
		client := newBenchmarkClient(ctx, b, driver)
		runGraphClientBenchmarks(ctx, b, client, "neo4j")
	})
	b.Run("Memgraph/memgraph:3.5.0", func(b *testing.B) {
		ctx := context.Background()
		driver := startMemgraphBenchmarkDriver(ctx, b)
		client := newBenchmarkClient(ctx, b, driver)
		runGraphClientBenchmarks(ctx, b, client, "memgraph")
	})
}

func runGraphClientBenchmarks(ctx context.Context, b *testing.B, client *neo4jadapter.Client, runtime string) {
	b.Run("WriteNode", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "write-node")
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		for i := 0; b.Loop(); i++ {
			opCtx, cancel := graphBenchmarkOperationContext(ctx)
			err := client.ExecuteWrite(opCtx, `
CREATE (:BTBenchNode {run: $run, seq: $seq, name: $name})
`, map[string]any{"run": runID, "seq": int64(i), "name": fmt.Sprintf("write-node-%d", i)})
			cancel()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("WriteRelationship", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "write-relationship")
		seedGraphBenchmarkData(ctx, b, client, runID, 2)
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		for i := 0; b.Loop(); i++ {
			opCtx, cancel := graphBenchmarkOperationContext(ctx)
			err := client.ExecuteWrite(opCtx, `
MATCH (a:BTBenchNode {run: $run, seq: 0})
MATCH (b:BTBenchNode {run: $run, seq: 1})
CREATE (a)-[:BTBENCH_REL {run: $run, seq: $seq, weight: $weight}]->(b)
`, map[string]any{"run": runID, "seq": int64(i), "weight": int64(i % 17)})
			cancel()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ReadVertices/Small10", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "read-vertices-small")
		seedGraphBenchmarkData(ctx, b, client, runID, 120)
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		benchmarkReadVertices(ctx, b, client, runID, `
MATCH (n:BTBenchNode {run: $run})
RETURN n
ORDER BY n.seq
LIMIT 10
`)
	})
	b.Run("ReadVertices/Medium100", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "read-vertices-medium")
		seedGraphBenchmarkData(ctx, b, client, runID, 120)
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		benchmarkReadVertices(ctx, b, client, runID, `
MATCH (n:BTBenchNode {run: $run})
RETURN n
ORDER BY n.seq
LIMIT 100
`)
	})
	b.Run("ReadEdges/Small10", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "read-edges-small")
		seedGraphBenchmarkData(ctx, b, client, runID, 120)
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		benchmarkReadEdges(ctx, b, client, runID, `
MATCH (:BTBenchNode {run: $run})-[r:BTBENCH_REL]->(:BTBenchNode {run: $run})
RETURN r
ORDER BY r.seq
LIMIT 10
`)
	})
	b.Run("ReadEdges/Medium100", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "read-edges-medium")
		seedGraphBenchmarkData(ctx, b, client, runID, 120)
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		benchmarkReadEdges(ctx, b, client, runID, `
MATCH (:BTBenchNode {run: $run})-[r:BTBENCH_REL]->(:BTBenchNode {run: $run})
RETURN r
ORDER BY r.seq
LIMIT 100
`)
	})
	b.Run("ReadEmptyResult", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "read-empty")
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		for b.Loop() {
			opCtx, cancel := graphBenchmarkOperationContext(ctx)
			vertices, err := client.ReadVertices(opCtx, `
MATCH (n:BTBenchMissing {run: $run})
RETURN n
`, map[string]any{"run": runID}, "n")
			cancel()
			if err != nil {
				b.Fatal(err)
			}
			if len(vertices) != 0 {
				b.Fatalf("vertices len = %d, want 0", len(vertices))
			}
		}
	})
	b.Run("WriteSyntaxError", func(b *testing.B) {
		runID := graphBenchmarkRunID(runtime, "write-syntax-error")
		cleanupGraphBenchmarkData(ctx, b, client, runID)
		for b.Loop() {
			opCtx, cancel := graphBenchmarkOperationContext(ctx)
			err := client.ExecuteWrite(opCtx, "RETURN $missing +", nil)
			cancel()
			if !errors.Is(err, neo4jadapter.ErrDriver) {
				b.Fatalf("ExecuteWrite(invalid) error = %v, want ErrDriver", err)
			}
		}
	})
}

func graphBenchmarkRunID(runtime string, name string) string {
	return fmt.Sprintf("%s-%s-%d", runtime, name, time.Now().UnixNano())
}

func cleanupGraphBenchmarkData(ctx context.Context, b *testing.B, client *neo4jadapter.Client, runID string) {
	b.Helper()
	b.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = client.ExecuteWrite(cleanupCtx, `
MATCH (n:BTBenchNode {run: $run})
DETACH DELETE n
`, map[string]any{"run": runID})
	})
}

func graphBenchmarkOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, graphBenchmarkOperationLimit)
}

func benchmarkReadVertices(ctx context.Context, b *testing.B, client *neo4jadapter.Client, runID string, query string) {
	for b.Loop() {
		params := map[string]any{"run": runID}
		opCtx, cancel := graphBenchmarkOperationContext(ctx)
		vertices, err := client.ReadVertices(opCtx, query, params, "n")
		cancel()
		if err != nil {
			b.Fatal(err)
		}
		if len(vertices) == 0 {
			b.Fatal("expected vertices")
		}
	}
}

func benchmarkReadEdges(ctx context.Context, b *testing.B, client *neo4jadapter.Client, runID string, query string) {
	for b.Loop() {
		params := map[string]any{"run": runID}
		opCtx, cancel := graphBenchmarkOperationContext(ctx)
		edges, err := client.ReadEdges(opCtx, query, params, "r")
		cancel()
		if err != nil {
			b.Fatal(err)
		}
		if len(edges) == 0 {
			b.Fatal("expected edges")
		}
	}
}

func seedGraphBenchmarkData(ctx context.Context, b *testing.B, client *neo4jadapter.Client, runID string, size int) {
	b.Helper()
	if size < 2 {
		b.Fatal("seed size must be at least 2")
	}
	opCtx, cancel := graphBenchmarkOperationContext(ctx)
	err := client.ExecuteWrite(opCtx, `
UNWIND range(0, $size - 1) AS seq
CREATE (:BTBenchNode {run: $run, seq: seq, name: 'node-' + toString(seq)})
`, map[string]any{"run": runID, "size": int64(size)})
	cancel()
	if err != nil {
		b.Fatalf("seed nodes: %v", err)
	}
	opCtx, cancel = graphBenchmarkOperationContext(ctx)
	err = client.ExecuteWrite(opCtx, `
MATCH (a:BTBenchNode {run: $run})
MATCH (b:BTBenchNode {run: $run})
WHERE b.seq = a.seq + 1
CREATE (a)-[:BTBENCH_REL {run: $run, seq: a.seq, weight: a.seq % 17}]->(b)
`, map[string]any{"run": runID})
	cancel()
	if err != nil {
		b.Fatalf("seed relationships: %v", err)
	}
}

func newBenchmarkClient(ctx context.Context, b *testing.B, driver neo4jdriver.Driver) *neo4jadapter.Client {
	b.Helper()
	client, err := neo4jadapter.NewClient(driver)
	if err != nil {
		b.Fatalf("NewClient() error = %v", err)
	}
	b.Cleanup(func() {
		if err := client.Close(ctx); err != nil {
			b.Fatalf("Close() error = %v", err)
		}
	})
	if err := client.VerifyConnectivity(ctx); err != nil {
		b.Fatalf("VerifyConnectivity() error = %v", err)
	}
	return client
}

func startNeo4jBenchmarkDriver(ctx context.Context, b *testing.B) neo4jdriver.Driver {
	b.Helper()
	container, err := tcneo4j.Run(ctx, "neo4j:5.26.0")
	if err != nil {
		b.Fatal(testcleanup.FormatStartError("neo4j", "neo4j:5.26.0", err))
	}
	testcleanup.Register(ctx, b, "neo4j", container)

	boltURL, err := container.BoltUrl(ctx)
	if err != nil {
		b.Fatalf("neo4j bolt URL: %v", err)
	}
	driver, err := neo4jdriver.NewDriver(boltURL, neo4jdriver.NoAuth())
	if err != nil {
		b.Fatalf("new neo4j driver: %v", err)
	}
	return driver
}

func startMemgraphBenchmarkDriver(ctx context.Context, b *testing.B) neo4jdriver.Driver {
	b.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        memgraphImage,
			ExposedPorts: []string{memgraphBoltPort},
			WaitingFor:   wait.ForListeningPort(memgraphBoltPort),
		},
		Started: true,
	})
	if err != nil {
		b.Fatal(testcleanup.FormatStartError("memgraph", memgraphImage, err))
	}
	testcleanup.Register(ctx, b, "memgraph", container)

	boltURL, err := container.PortEndpoint(ctx, memgraphBoltPort, "bolt")
	if err != nil {
		b.Fatalf("memgraph bolt URL: %v", err)
	}
	driver, err := neo4jdriver.NewDriver(boltURL, neo4jdriver.NoAuth())
	if err != nil {
		b.Fatalf("new memgraph driver: %v", err)
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := waitForMemgraphConnectivity(verifyCtx, driver); err != nil {
		_ = driver.Close(ctx)
		b.Fatalf("memgraph verify connectivity: %v", err)
	}
	return driver
}
