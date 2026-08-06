package mariadbtestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	tcmariadb "github.com/testcontainers/testcontainers-go/modules/mariadb"
)

const (
	defaultImage    = "mariadb:11.0.3"
	defaultDatabase = "bluetape"
	defaultUsername = "bluetape"
	defaultPassword = "bluetape"

	// DSNKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	DSNKey = "mariadb.dsn"
)

// Start Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	return mustDetail(ctx, tb, StartServer(ctx, tb), DSNKey)
}

// StartServer Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tcmariadb.Run(
		ctx,
		defaultImage,
		tcmariadb.WithDatabase(defaultDatabase),
		tcmariadb.WithUsername(defaultUsername),
		tcmariadb.WithPassword(defaultPassword),
	)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("mariadb", defaultImage, err))
	}

	srv, err := tcserver.New("mariadb", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		dsn, err := container.ConnectionString(ctx, "parseTime=true")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{DSNKey: dsn}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("mariadb server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("mariadb server: %v", err)
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
