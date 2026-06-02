package natstestcontainer

import (
	"context"
	"testing"

	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
)

const defaultImage = "nats:2.10-alpine"

// Start 는 NATS test container를 시작하고 connection URL을 반환한다.
func Start(ctx context.Context, t *testing.T) string {
	t.Helper()

	container, err := tcnats.Run(ctx, defaultImage)
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("terminate nats container: %v", err)
		}
	})

	url, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("nats connection string: %v", err)
	}
	return url
}
