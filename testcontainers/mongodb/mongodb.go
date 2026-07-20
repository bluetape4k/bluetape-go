package mongodbtestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

const (
	defaultImage = "mongo:7.0@sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f"

	// URIKey is the documented key for a MongoDB connection URI.
	URIKey = "mongodb.uri"
)

// Start launches a MongoDB test container and returns its connection URI.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	return mustDetail(ctx, tb, StartServer(ctx, tb), URIKey)
}

// StartServer launches a MongoDB test container and returns the shared server view.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tcmongodb.Run(ctx, defaultImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("mongodb", defaultImage, err))
	}

	srv, err := tcserver.New("mongodb", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		uri, err := container.ConnectionString(ctx)
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{URIKey: uri}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("mongodb server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("mongodb server: %v", err)
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
