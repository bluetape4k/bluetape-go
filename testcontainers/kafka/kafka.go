package kafkatestcontainer

import (
	"context"
	"testing"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
)

const (
	defaultImage     = "confluentinc/confluent-local:7.5.0"
	defaultClusterID = "bluetape-test-cluster"

	// BrokersKey is the documented key for the Kafka broker address list.
	BrokersKey = "kafka.brokers"
)

// Start launches a Kafka test container and returns broker addresses.
func Start(ctx context.Context, tb testing.TB) []string {
	tb.Helper()

	container, err := tckafka.Run(
		ctx,
		defaultImage,
		tckafka.WithClusterID(defaultClusterID),
	)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("kafka", defaultImage, err))
	}

	testcleanup.Register(ctx, tb, "kafka", container)

	brokers, err := container.Brokers(ctx)
	if err != nil {
		tb.Fatalf("%s: %v", BrokersKey, err)
	}
	if len(brokers) == 0 {
		tb.Fatalf("%s: empty broker list", BrokersKey)
	}
	return brokers
}
