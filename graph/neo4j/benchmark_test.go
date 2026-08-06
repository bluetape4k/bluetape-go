package neo4j_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
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
	graphBenchmarkCleanupLimit   = 30 * time.Second
	neo4jBenchmarkImage          = "neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5"
	memgraphBenchmarkImage       = "memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1"
)

type graphBenchmarkCleanupStage struct {
	name string
	run  func(context.Context) error
}

func runGraphBenchmarkCleanupStages(parent context.Context, timeout time.Duration, stages []graphBenchmarkCleanupStage) error {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = graphBenchmarkCleanupLimit
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

func registerGraphBenchmarkCleanup(parent context.Context, b *testing.B, timeout time.Duration, stage graphBenchmarkCleanupStage) {
	b.Helper()
	b.Cleanup(func() {
		if err := runGraphBenchmarkCleanupStages(parent, timeout, []graphBenchmarkCleanupStage{stage}); err != nil {
			b.Errorf("graph benchmark cleanup: %v", err)
		}
	})
}

func TestGraphBenchmarkImagesAreImmutable(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{
			name:  "neo4j",
			image: neo4jBenchmarkImage,
			want:  "neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5",
		},
		{
			name:  "memgraph",
			image: memgraphBenchmarkImage,
			want:  "memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1",
		},
	}
	immutableImage := regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.image != tt.want {
				t.Fatalf("image = %q, want %q", tt.image, tt.want)
			}
			if !immutableImage.MatchString(tt.image) {
				t.Fatalf("image = %q, want immutable image reference", tt.image)
			}
		})
	}
}

func TestRunGraphBenchmarkCleanupStagesContinuesAndJoinsErrors(t *testing.T) {
	wantDelete := errors.New("delete failed")
	wantClose := errors.New("close failed")
	wantTerminate := errors.New("terminate failed")
	var calls []string
	err := runGraphBenchmarkCleanupStages(context.Background(), 100*time.Millisecond, []graphBenchmarkCleanupStage{
		{name: "delete data", run: func(context.Context) error { calls = append(calls, "delete"); return wantDelete }},
		{name: "close driver", run: func(context.Context) error { calls = append(calls, "close"); return wantClose }},
		{name: "terminate container", run: func(context.Context) error { calls = append(calls, "terminate"); return wantTerminate }},
	})
	if !reflect.DeepEqual(calls, []string{"delete", "close", "terminate"}) {
		t.Fatalf("cleanup calls = %#v", calls)
	}
	for _, want := range []error{wantDelete, wantClose, wantTerminate} {
		if !errors.Is(err, want) {
			t.Fatalf("cleanup error = %v, want joined %v", err, want)
		}
	}
}

func TestRunGraphBenchmarkCleanupStagesBoundsEveryStage(t *testing.T) {
	var calls []string
	block := func(name string) func(context.Context) error {
		return func(ctx context.Context) error {
			calls = append(calls, name)
			<-ctx.Done()
			return ctx.Err()
		}
	}
	started := time.Now()
	err := runGraphBenchmarkCleanupStages(context.Background(), 10*time.Millisecond, []graphBenchmarkCleanupStage{
		{name: "delete data", run: block("delete")},
		{name: "close driver", run: block("close")},
		{name: "terminate container", run: block("terminate")},
	})
	if err == nil {
		t.Fatal("expected bounded cleanup error")
	}
	if !reflect.DeepEqual(calls, []string{"delete", "close", "terminate"}) {
		t.Fatalf("cleanup calls = %#v", calls)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded cleanup took %s", elapsed)
	}
}

func TestRunGraphBenchmarkCleanupStagesIgnoresCanceledParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := runGraphBenchmarkCleanupStages(parent, time.Second, []graphBenchmarkCleanupStage{
		{name: "delete data", run: func(ctx context.Context) error {
			called = true
			return ctx.Err()
		}},
	})
	if err != nil {
		t.Fatalf("cleanup with canceled parent: %v", err)
	}
	if !called {
		t.Fatal("cleanup stage was not called")
	}
}

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
	registerGraphBenchmarkCleanup(ctx, b, graphBenchmarkOperationLimit, graphBenchmarkCleanupStage{
		name: "delete benchmark data " + runID,
		run: func(cleanupCtx context.Context) error {
			return client.ExecuteWrite(cleanupCtx, `
MATCH (n:BTBenchNode {run: $run})
DETACH DELETE n
`, map[string]any{"run": runID})
		},
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
	registerGraphBenchmarkCleanup(ctx, b, graphBenchmarkOperationLimit, graphBenchmarkCleanupStage{
		name: "close graph adapter and driver",
		run:  client.Close,
	})
	if err := client.VerifyConnectivity(ctx); err != nil {
		b.Fatalf("VerifyConnectivity() error = %v", err)
	}
	return client
}

func startNeo4jBenchmarkDriver(ctx context.Context, b *testing.B) neo4jdriver.Driver {
	b.Helper()
	container, err := tcneo4j.Run(ctx, neo4jBenchmarkImage)
	if err != nil {
		b.Fatal(testcleanup.FormatStartError("neo4j", neo4jBenchmarkImage, err))
	}
	registerGraphBenchmarkCleanup(ctx, b, graphBenchmarkCleanupLimit, graphBenchmarkCleanupStage{
		name: "terminate neo4j container",
		run: func(cleanupCtx context.Context) error {
			return container.Terminate(cleanupCtx)
		},
	})

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
			Image:        memgraphBenchmarkImage,
			ExposedPorts: []string{memgraphBoltPort},
			WaitingFor:   wait.ForListeningPort(memgraphBoltPort),
		},
		Started: true,
	})
	if err != nil {
		b.Fatal(testcleanup.FormatStartError("memgraph", memgraphBenchmarkImage, err))
	}
	registerGraphBenchmarkCleanup(ctx, b, graphBenchmarkCleanupLimit, graphBenchmarkCleanupStage{
		name: "terminate memgraph container",
		run: func(cleanupCtx context.Context) error {
			return container.Terminate(cleanupCtx)
		},
	})

	boltURL, err := container.PortEndpoint(ctx, memgraphBoltPort, "bolt")
	if err != nil {
		b.Fatalf("memgraph bolt URL: %v", err)
	}
	driver, err := neo4jdriver.NewDriver(boltURL, neo4jdriver.NoAuth())
	if err != nil {
		b.Fatalf("new memgraph driver: %v", err)
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	verifyErr := waitForMemgraphConnectivity(verifyCtx, driver)
	cancel()
	if verifyErr != nil {
		closeErr := runGraphBenchmarkCleanupStages(ctx, graphBenchmarkOperationLimit, []graphBenchmarkCleanupStage{
			{name: "close memgraph driver after readiness failure", run: driver.Close},
		})
		b.Fatalf("memgraph verify connectivity: %v", errors.Join(verifyErr, closeErr))
	}
	return driver
}
