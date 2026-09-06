package falkordbtestcontainer

import (
	"context"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage = "falkordb/falkordb:latest@sha256:adbddd418916c25618564ff8597a919b08bc76452ebeb74eb985c38d7281df62"

	// AddressKey FalkorDB fixture의 host:port detail key다.
	AddressKey = "falkordb.address"
)

// Start는 digest-pinned FalkorDB fixture의 address를 반환한다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()
	return mustDetail(ctx, tb, StartServer(ctx, tb), AddressKey)
}

// StartServer는 FalkorDB container와 bounded cleanup을 준비한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        defaultImage,
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("falkordb", defaultImage, err))
	}
	srv, err := tcserver.New("falkordb", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		address, err := container.PortEndpoint(ctx, "6379/tcp", "")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{AddressKey: address}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("falkordb server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("falkordb server: %v", err)
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
