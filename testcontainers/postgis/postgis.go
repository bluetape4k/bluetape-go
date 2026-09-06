package postgistestcontainer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage    = "postgis/postgis:16-3.5-alpine@sha256:47e961a569fd52ff31f0fe205ed91eeab17d9f5fff6722e6d7ea6b588748b293"
	defaultDatabase = "bluetape"
	defaultUsername = "bluetape"
	defaultPassword = "bluetape"

	readinessTimeout = 90 * time.Second
	readinessPoll    = 100 * time.Millisecond

	// ConnectionStringKey PostGIS fixture가 caller에게 제공하는 connection string key다.
	ConnectionStringKey = "postgis.connection-string"
)

// Start는 PostGIS fixture의 connection string을 반환한다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()
	return mustDetail(ctx, tb, StartServer(ctx, tb), ConnectionStringKey)
}

// StartServer는 digest-pinned PostGIS container와 bounded readiness/cleanup을 준비한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:         defaultImage,
			ImagePlatform: "linux/amd64",
			Env: map[string]string{
				"POSTGRES_DB":       defaultDatabase,
				"POSTGRES_USER":     defaultUsername,
				"POSTGRES_PASSWORD": defaultPassword,
			},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(readinessTimeout),
		},
		Started: true,
	})
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("postgis", defaultImage, err))
	}
	if err := waitForPostGIS(ctx, container); err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("postgis readiness: %v; terminate after readiness failure: %v", err, terminateErr)
		}
		tb.Fatalf("postgis readiness: %v", err)
	}

	srv, err := tcserver.New("postgis", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		host, err := container.Host(ctx)
		if err != nil {
			return nil, err
		}
		port, err := container.MappedPort(ctx, "5432/tcp")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{ConnectionStringKey: fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", defaultUsername, defaultPassword, net.JoinHostPort(host, port.Port()), defaultDatabase)}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("postgis server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("postgis server: %v", err)
	}
	srv.RegisterCleanup(ctx, tb)
	return srv
}

func waitForPostGIS(ctx context.Context, container testcontainers.Container) error {
	if container == nil {
		return errors.New("postgis container must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	readinessCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	host, err := container.Host(readinessCtx)
	if err != nil {
		return err
	}
	port, err := container.MappedPort(readinessCtx, "5432/tcp")
	if err != nil {
		return err
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", defaultUsername, defaultPassword, net.JoinHostPort(host, port.Port()), defaultDatabase)
	ticker := time.NewTicker(readinessPoll)
	defer ticker.Stop()
	for {
		attemptCtx, attemptCancel := context.WithTimeout(readinessCtx, time.Second)
		conn, connectErr := pgx.Connect(attemptCtx, dsn)
		if connectErr == nil {
			connectErr = conn.Ping(attemptCtx)
			_ = conn.Close(context.Background())
		}
		attemptCancel()
		if connectErr == nil {
			return nil
		}
		select {
		case <-readinessCtx.Done():
			return fmt.Errorf("postgis readiness: %w", errors.Join(readinessCtx.Err(), connectErr))
		case <-ticker.C:
		}
	}
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
