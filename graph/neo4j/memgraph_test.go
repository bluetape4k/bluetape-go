package neo4j_test

import (
	"context"
	"errors"
	"testing"
	"time"

	neo4jadapter "github.com/bluetape4k/bluetape-go/graph/neo4j"
	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	memgraphImage    = "memgraph/memgraph:3.5.0"
	memgraphBoltPort = "7687/tcp"
)

func TestClientMemgraphCompatibilityWithGenericContainer(t *testing.T) {
	ctx := context.Background()
	driver := startMemgraphDriver(ctx, t)
	client, err := neo4jadapter.NewClient(driver)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.VerifyConnectivity(ctx); err != nil {
		t.Fatalf("VerifyConnectivity() error = %v", err)
	}
	if err := client.ExecuteWrite(ctx, `
CREATE (a:Service {name: $source})-[r:CALLS {weight: $weight}]->(b:Service {name: $target})
`, map[string]any{
		"source": "memgraph-checkout",
		"target": "memgraph-payments",
		"weight": int64(11),
	}); err != nil {
		t.Fatalf("ExecuteWrite() error = %v", err)
	}

	vertices, err := client.ReadVertices(ctx, `
MATCH (n:Service {name: $name})
RETURN n
`, map[string]any{"name": "memgraph-checkout"}, "n")
	if err != nil {
		t.Fatalf("ReadVertices() error = %v", err)
	}
	if len(vertices) != 1 {
		t.Fatalf("ReadVertices() len = %d, want 1", len(vertices))
	}
	if vertices[0].Label().String() != "Service" || vertices[0].Properties()["name"] != "memgraph-checkout" {
		t.Fatalf("vertex = %s props=%#v", vertices[0].Label(), vertices[0].Properties())
	}

	edges, err := client.ReadEdges(ctx, `
MATCH (:Service {name: $source})-[r:CALLS]->(:Service {name: $target})
RETURN r
`, map[string]any{"source": "memgraph-checkout", "target": "memgraph-payments"}, "r")
	if err != nil {
		t.Fatalf("ReadEdges() error = %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("ReadEdges() len = %d, want 1", len(edges))
	}
	if edges[0].Label().String() != "CALLS" || edges[0].Properties()["weight"] != int64(11) {
		t.Fatalf("edge = %s props=%#v", edges[0].Label(), edges[0].Properties())
	}

	if err := client.ExecuteWrite(ctx, "RETURN $missing +", nil); !errors.Is(err, neo4jadapter.ErrDriver) {
		t.Fatalf("ExecuteWrite(bad query) error = %v, want ErrDriver", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = client.ReadVertices(canceled, "RETURN 1 AS n", nil, "n")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadVertices(canceled) error = %v, want context.Canceled", err)
	}

	if err := client.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func startMemgraphDriver(ctx context.Context, t *testing.T) neo4jdriver.Driver {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        memgraphImage,
			ExposedPorts: []string{memgraphBoltPort},
			WaitingFor:   wait.ForListeningPort(memgraphBoltPort),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(testcleanup.FormatStartError("memgraph", memgraphImage, err))
	}
	testcleanup.Register(ctx, t, "memgraph", container)

	boltURL, err := container.PortEndpoint(ctx, memgraphBoltPort, "bolt")
	if err != nil {
		t.Fatalf("memgraph bolt URL: %v", err)
	}
	driver, err := neo4jdriver.NewDriver(boltURL, neo4jdriver.NoAuth())
	if err != nil {
		t.Fatalf("new memgraph driver: %v", err)
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := waitForMemgraphConnectivity(verifyCtx, driver); err != nil {
		_ = driver.Close(ctx)
		t.Fatalf("memgraph verify connectivity: %v", err)
	}
	return driver
}

func waitForMemgraphConnectivity(ctx context.Context, driver neo4jdriver.Driver) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := driver.VerifyConnectivity(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}
