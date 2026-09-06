package gremlin

import (
	"context"
	"testing"

	tinkerpoptestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/tinkerpop"
)

func TestRemoteResultMapping(t *testing.T) {
	endpoint := tinkerpoptestcontainer.Start(context.Background(), t)
	client, err := NewRemoteClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	result, err := client.Query(context.Background(), "g.inject(1)")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 1 || result.Values[0] != int32(1) && result.Values[0] != int64(1) && result.Values[0] != 1 {
		t.Fatalf("unexpected remote result: %#v", result.Values)
	}
}
