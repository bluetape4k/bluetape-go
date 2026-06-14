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
)

// Start 는 MySQL test container를 시작하고 DSN을 반환한다.
func Start(ctx context.Context, t *testing.T) string {
	t.Helper()

	container, err := tcmysql.Run(
		ctx,
		defaultImage,
		tcmysql.WithDatabase(defaultDatabase),
		tcmysql.WithUsername(defaultUsername),
		tcmysql.WithPassword(defaultPassword),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}

	testcleanup.Register(ctx, t, "mysql", container)

	dsn, err := container.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		t.Fatalf("mysql connection string: %v", err)
	}
	return dsn
}
