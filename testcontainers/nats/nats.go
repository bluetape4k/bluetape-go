package natstestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
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

	return mustDetail(ctx, tb, StartServer(ctx, tb), URLKey)
}

// StartServer launches a NATS test container and returns the shared server view.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tcnats.Run(ctx, defaultImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("nats", defaultImage, err))
	}

	srv, err := tcserver.New("nats", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		url, err := container.ConnectionString(ctx)
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{URLKey: url}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("nats server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("nats server: %v", err)
	}

	srv.RegisterCleanup(ctx, tb)
	return srv
}

func mustDetail(ctx context.Context, tb testing.TB, srv *tcserver.Started, key string) string {
	tb.Helper()

	details, err := srv.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("%s: %v", key, err)
	}
	value, err := details.Require(key)
	if err != nil {
		tb.Fatalf("%s: %v", key, err)
	}
	return value
}
