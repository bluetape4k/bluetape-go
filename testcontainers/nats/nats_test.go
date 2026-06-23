package natstestcontainer_test

import (
	"context"
	"testing"
	"time"

	natstestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/nats"
	"github.com/nats-io/nats.go"
)

func TestStartNATS(t *testing.T) {
	url := natstestcontainer.Start(context.Background(), t)
	client, err := nats.Connect(url, nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(client.Close)

	if err := client.Publish("bluetape.test", []byte("ping")); err != nil {
		t.Fatalf("publish nats message: %v", err)
	}
	if err := client.FlushTimeout(5 * time.Second); err != nil {
		t.Fatalf("flush nats message: %v", err)
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if natstestcontainer.URLKey != "nats.url" {
		t.Fatalf("URLKey = %q", natstestcontainer.URLKey)
	}
}
