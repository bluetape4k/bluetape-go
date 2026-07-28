package toxiproxytestcontainer

import (
	"context"
	"fmt"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	"github.com/testcontainers/testcontainers-go"
	tctoxiproxy "github.com/testcontainers/testcontainers-go/modules/toxiproxy"
)

const (
	defaultImage = "ghcr.io/shopify/toxiproxy:2.12.0"

	// ControlURIKey는 Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	ControlURIKey = "toxiproxy.control_uri"
)

// Start는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func Start(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) string {
	tb.Helper()

	return mustDetail(ctx, tb, StartServer(ctx, tb, opts...), ControlURIKey)
}

// StartServer는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartServer(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) *tcserver.Started {
	tb.Helper()

	container, err := tctoxiproxy.Run(ctx, defaultImage, opts...)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("toxiproxy", defaultImage, err))
	}

	srv, err := tcserver.New("toxiproxy", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		uri, err := container.URI(ctx)
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{ControlURIKey: uri}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("toxiproxy server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("toxiproxy server: %v", err)
	}

	srv.RegisterCleanup(ctx, tb)
	return srv
}

// StartContainer는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartContainer(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) *tctoxiproxy.Container {
	tb.Helper()

	container, err := tctoxiproxy.Run(ctx, defaultImage, opts...)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("toxiproxy", defaultImage, err))
	}
	testcleanup.Register(ctx, tb, "toxiproxy", container)
	return container
}

// ProxiedEndpoint는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func ProxiedEndpoint(ctx context.Context, tb testing.TB, container *tctoxiproxy.Container, port int) string {
	tb.Helper()
	if container == nil {
		tb.Fatal("toxiproxy container must not be nil")
	}
	if err := ctx.Err(); err != nil {
		tb.Fatalf("toxiproxy proxied endpoint %d: %v", port, err)
	}
	host, mappedPort, err := container.ProxiedEndpoint(port)
	if err != nil {
		tb.Fatalf("toxiproxy proxied endpoint %d: %v", port, err)
	}
	return fmt.Sprintf("%s:%s", host, mappedPort)
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
