package falkordb_test

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph/falkordb"
	falkordbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/falkordb"
	"github.com/redis/go-redis/v9"
)

func TestFalkorDBCreateQueryDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	redisClient := redis.NewClient(&redis.Options{Addr: falkordbtestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = redisClient.Close() })
	client, err := falkordb.NewClient(redisClient, "btgc_integration")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyConnectivity(ctx); err != nil {
		t.Fatalf("connectivity: %v", err)
	}
	if _, err := client.Query(ctx, "CREATE (:BTGC {name: $name})", map[string]any{"name": "falkor"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	result, err := client.Query(ctx, "RETURN 1", nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("rows=%d want=1", len(result.Rows))
	}
	if err := client.DeleteGraph(ctx); err != nil {
		t.Fatalf("delete graph: %v", err)
	}
}
