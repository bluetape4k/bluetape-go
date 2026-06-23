package mysqltestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
)

const (
	defaultImage    = "mysql:8.4"
	defaultDatabase = "bluetape"
	defaultUsername = "bluetape"
	defaultPassword = "bluetape"

	// DSNKey is the documented key for a MySQL data source name.
	DSNKey = "mysql.dsn"
)

// Start launches a MySQL test container and returns its data source name.
func Start(ctx context.Context, tb testing.TB) string {
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

	testcleanup.Register(ctx, tb, "mysql", container)

	dsn, err := container.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		tb.Fatalf("%s: %v", DSNKey, err)
	}
	return dsn
}
