package gremlin_test

import (
	"context"

	gremlin "github.com/bluetape4k/bluetape-go/graph/gremlin"
)

func ExampleNewRemoteClient() {
	client, err := gremlin.NewRemoteClient("ws://localhost:8182/gremlin")
	if err != nil {
		return
	}
	defer client.Close(context.Background())
	_, _ = client.Query(context.Background(), "g.V().limit(1)")
}
