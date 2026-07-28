package mysqltestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
)

const (
	defaultImage    = "mysql:8.4"
	defaultDatabase = "bluetape"
	defaultUsername = "bluetape"
	defaultPassword = "bluetape"

	// DSNKey는 Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	DSNKey = "mysql.dsn"
)

// Start는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	return mustDetail(ctx, tb, StartServer(ctx, tb), DSNKey)
}

// StartServer는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tcmysql.Run(
		ctx,
		defaultImage,
		tcmysql.WithDatabase(defaultDatabase),
		tcmysql.WithUsername(defaultUsername),
		tcmysql.WithPassword(defaultPassword),
	)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("mysql", defaultImage, err))
	}

	srv, err := tcserver.New("mysql", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		dsn, err := container.ConnectionString(ctx, "parseTime=true")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{DSNKey: dsn}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("mysql server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("mysql server: %v", err)
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
