package natstestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
)

const (
	defaultImage = "nats:2.10-alpine"

	// URLKey is the documented key for the NATS connection URL.
	URLKey = "nats.url"
)

// Start launches a NATS test container and returns its connection URL.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	container, err := tcnats.Run(ctx, defaultImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("nats", defaultImage, err))
	}

	testcleanup.Register(ctx, tb, "nats", container)

	url, err := container.ConnectionString(ctx)
	if err != nil {
		tb.Fatalf("%s: %v", URLKey, err)
	}
	return url
}
