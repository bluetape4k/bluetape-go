package redistestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	defaultImage = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"

	// AddressKey Redis host:port 주소를 저장하는 documented key다.
	AddressKey = "redis.address"
)

// Start Start 공개 API의 동작을 수행하며 Redis test fixture의 image, address, cleanup ownership 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tb: 실패를 보고할 testing 객체다.
func Start(ctx context.Context, tb testing.TB) string {
	tb.Helper()

	return mustDetail(ctx, tb, StartServer(ctx, tb), AddressKey)
}

// StartServer StartServer 공개 API의 동작을 수행하며 Redis test fixture의 image, address, cleanup ownership 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tb: 실패를 보고할 testing 객체다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tcredis.Run(ctx, defaultImage)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("redis", defaultImage, err))
	}

	srv, err := tcserver.New("redis", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
		if err != nil {
			return nil, err
		}
		return tcserver.ConnectionDetails{AddressKey: addr}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("redis server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("redis server: %v", err)
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
