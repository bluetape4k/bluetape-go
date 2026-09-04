package postgrestestcontainer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	"github.com/jackc/pgx/v5"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	defaultImage    = "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"
	defaultDatabase = "bluetape"
	defaultUsername = "bluetape"
	defaultPassword = "bluetape"

	postgresReadinessTimeout = 60 * time.Second
	readinessAttemptTimeout  = time.Second
	readinessPollInterval    = 100 * time.Millisecond

	// ConnectionStringKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	ConnectionStringKey = "postgres.connection-string"
)

// Start Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	return mustDetail(ctx, tb, StartServer(ctx, tb), ConnectionStringKey)
}

// StartServer Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tcpostgres.Run(
		ctx,
		defaultImage,
		tcpostgres.WithDatabase(defaultDatabase),
		tcpostgres.WithUsername(defaultUsername),
		tcpostgres.WithPassword(defaultPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("postgres", defaultImage, err))
	}
	if err := waitForPostgres(ctx, container); err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("postgres readiness: %v; terminate after readiness failure: %v", err, terminateErr)
		}
		tb.Fatalf("postgres readiness: %v", err)
	}

	srv, err := tcserver.New("postgres", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		connString, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{ConnectionStringKey: connString}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("postgres server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("postgres server: %v", err)
	}

	srv.RegisterCleanup(ctx, tb)
	return srv
}

func waitForPostgres(ctx context.Context, container *tcpostgres.PostgresContainer) error {
	if container == nil {
		return errors.New("postgres container must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	readinessCtx, cancel := context.WithTimeout(ctx, postgresReadinessTimeout)
	defer cancel()
	dsn, err := container.ConnectionString(readinessCtx, "sslmode=disable")
	if err != nil {
		return err
	}

	return waitForReady(readinessCtx, func(attemptCtx context.Context) error {
		conn, err := pgx.Connect(attemptCtx, dsn)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close(context.Background()) }()
		return conn.Ping(attemptCtx)
	})
}

func waitForReady(ctx context.Context, attempt func(context.Context) error) error {
	if attempt == nil {
		return errors.New("readiness attempt must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(readinessPollInterval)
	defer ticker.Stop()
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, readinessAttemptTimeout)
		lastErr = attempt(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("postgres readiness: %w", errors.Join(ctx.Err(), lastErr))
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
