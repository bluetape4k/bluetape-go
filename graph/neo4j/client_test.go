package neo4j_test

import (
	"context"
	"errors"
	"testing"

	neo4jadapter "github.com/bluetape4k/bluetape-go/graph/neo4j"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	tcneo4j "github.com/testcontainers/testcontainers-go/modules/neo4j"
)

func TestClientRejectsInvalidInputs(t *testing.T) {
	if _, err := neo4jadapter.NewClient(nil); !errors.Is(err, neo4jadapter.ErrInvalidOptions) {
		t.Fatalf("NewClient(nil) error = %v, want ErrInvalidOptions", err)
	}
}

func TestClientReadWriteFailureCancellationAndCleanupWithTestcontainersNeo4j(t *testing.T) {
	ctx := context.Background()
	driver := startNeo4jDriver(ctx, t)
	if _, err := neo4jadapter.NewClient(driver, nil); !errors.Is(err, neo4jadapter.ErrInvalidOptions) {
		t.Fatalf("NewClient(nil option) error = %v, want ErrInvalidOptions", err)
	}
	if _, err := neo4jadapter.NewClient(driver, neo4jadapter.WithDatabase(" ")); !errors.Is(err, neo4jadapter.ErrInvalidOptions) {
		t.Fatalf("NewClient(blank database) error = %v, want ErrInvalidOptions", err)
	}

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
		"source": "checkout",
		"target": "payments",
		"weight": int64(7),
	}); err != nil {
		t.Fatalf("ExecuteWrite() error = %v", err)
	}

	vertices, err := client.ReadVertices(ctx, `
MATCH (n:Service {name: $name})
RETURN n
`, map[string]any{"name": "checkout"}, "n")
	if err != nil {
		t.Fatalf("ReadVertices() error = %v", err)
	}
	if len(vertices) != 1 {
		t.Fatalf("ReadVertices() len = %d, want 1", len(vertices))
	}
	if vertices[0].Label().String() != "Service" || vertices[0].Properties()["name"] != "checkout" {
		t.Fatalf("vertex = %s props=%#v", vertices[0].Label(), vertices[0].Properties())
	}

	edges, err := client.ReadEdges(ctx, `
MATCH (:Service {name: $source})-[r:CALLS]->(:Service {name: $target})
RETURN r
`, map[string]any{"source": "checkout", "target": "payments"}, "r")
	if err != nil {
		t.Fatalf("ReadEdges() error = %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("ReadEdges() len = %d, want 1", len(edges))
	}
	if edges[0].Label().String() != "CALLS" || edges[0].Properties()["weight"] != int64(7) {
		t.Fatalf("edge = %s props=%#v", edges[0].Label(), edges[0].Properties())
	}
	if edges[0].StartID().String() == "" || edges[0].EndID().String() == "" {
		t.Fatalf("edge endpoints should be populated")
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
	if err := client.VerifyConnectivity(ctx); !errors.Is(err, neo4jadapter.ErrDriver) {
		t.Fatalf("VerifyConnectivity(closed) error = %v, want ErrDriver", err)
	}
}

func startNeo4jDriver(ctx context.Context, t *testing.T) neo4jdriver.Driver {
	t.Helper()
	container, err := tcneo4j.Run(ctx, "neo4j:5.26.0")
	if err != nil {
		t.Fatalf("start neo4j container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Fatalf("terminate neo4j container: %v", err)
		}
	})

	boltURL, err := container.BoltUrl(ctx)
	if err != nil {
		t.Fatalf("neo4j bolt URL: %v", err)
	}
	driver, err := neo4jdriver.NewDriver(boltURL, neo4jdriver.NoAuth())
	if err != nil {
		t.Fatalf("new neo4j driver: %v", err)
	}
	return driver
}
