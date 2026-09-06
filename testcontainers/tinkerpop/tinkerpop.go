package tinkerpoptestcontainer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage = "tinkerpop/gremlin-server:3.8.1@sha256:d7b23b4b6773a521cb70cf82c68584a6c68e35019c1357ab4c9371c4e843d651"
	gremlinPort  = "8182/tcp"

	// EndpointKey TinkerPop WebSocket endpoint를 저장하는 documented key다.
	EndpointKey = "gremlin.endpoint"
)

// Start는 local TinkerPop endpoint를 시작하고 test cleanup에 등록한다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()
	return mustDetail(ctx, tb, StartServer(ctx, tb), EndpointKey)
}

// StartServer는 digest-pinned Gremlin Server를 시작해 connection details를 반환한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        defaultImage,
			ExposedPorts: []string{gremlinPort},
			WaitingFor: wait.ForListeningPort(gremlinPort).
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("tinkerpop", defaultImage, err))
	}

	srv, err := tcserver.New("tinkerpop", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		endpoint, err := container.PortEndpoint(ctx, gremlinPort, "ws")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{EndpointKey: strings.TrimRight(endpoint, "/") + "/gremlin"}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("tinkerpop server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("tinkerpop server: %v", err)
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
