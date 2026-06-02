package kafkatestcontainer

import (
	"context"
	"testing"

	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
)

const (
	defaultImage     = "confluentinc/confluent-local:7.5.0"
	defaultClusterID = "bluetape-test-cluster"
)

// Start 는 Kafka test container를 시작하고 broker 주소 목록을 반환한다.
func Start(ctx context.Context, t *testing.T) []string {
	t.Helper()

	container, err := tckafka.Run(
		ctx,
		defaultImage,
		tckafka.WithClusterID(defaultClusterID),
	)
	if err != nil {
		t.Fatalf("start kafka container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Fatalf("terminate kafka container: %v", err)
		}
	})

	brokers, err := container.Brokers(ctx)
	if err != nil {
		t.Fatalf("kafka brokers: %v", err)
	}
	if len(brokers) == 0 {
		t.Fatal("kafka brokers: empty broker list")
	}
	return brokers
}
