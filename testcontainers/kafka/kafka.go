package kafkatestcontainer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
)

const (
	defaultImage     = "confluentinc/confluent-local:7.5.0"
	defaultClusterID = "bluetape-test-cluster"

	// BrokersKey는 Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	BrokersKey = "kafka.brokers"
)

var errEmptyBrokers = errors.New("empty broker list")

// Start는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func Start(ctx context.Context, tb testing.TB) []string {
	tb.Helper()

	brokers := strings.Split(mustDetail(ctx, tb, StartServer(ctx, tb), BrokersKey), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		tb.Fatalf("%s: empty broker list", BrokersKey)
	}
	return brokers
}

// StartServer는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartServer(ctx context.Context, tb testing.TB) *tcserver.Started {
	tb.Helper()

	container, err := tckafka.Run(
		ctx,
		defaultImage,
		tckafka.WithClusterID(defaultClusterID),
	)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("kafka", defaultImage, err))
	}

	srv, err := tcserver.New("kafka", container, tcserver.WithConnectionDetails(func(ctx context.Context, _ *tcserver.Started) (tcserver.ConnectionDetails, error) {
		brokers, err := container.Brokers(ctx)
		if err != nil {
			return nil, err
		}
		if len(brokers) == 0 {
			return nil, errEmptyBrokers
		}
		return tcserver.ConnectionDetails{BrokersKey: strings.Join(brokers, ",")}, nil
	}))
	if err != nil {
		if terminateErr := testcleanup.Terminate(ctx, testcleanup.DefaultTerminateTimeout, container); terminateErr != nil {
			tb.Fatalf("kafka server: %v; terminate after construction failure: %v", err, terminateErr)
		}
		tb.Fatalf("kafka server: %v", err)
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
