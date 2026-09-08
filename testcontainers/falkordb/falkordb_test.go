package falkordbtestcontainer_test

import (
	"context"
	"testing"
	"time"

	falkordbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/falkordb"
	"github.com/redis/go-redis/v9"
)

func TestStartFalkorDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: falkordbtestcontainer.Start(ctx, t)})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping falkordb: %v", err)
	}
}

func TestAddressKey(t *testing.T) {
	if falkordbtestcontainer.AddressKey != "falkordb.address" {
		t.Fatalf("AddressKey=%q", falkordbtestcontainer.AddressKey)
	}
}
