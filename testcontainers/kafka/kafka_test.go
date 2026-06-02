package kafkatestcontainer_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	kafkatestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/kafka"
	"github.com/segmentio/kafka-go"
)

func TestStartKafka(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	brokers := kafkatestcontainer.Start(ctx, t)
	topic := fmt.Sprintf("bluetape-test-%d", time.Now().UnixNano())
	createTopic(ctx, t, brokers[0], topic)

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		AllowAutoTopicCreation: true,
		RequiredAcks:           kafka.RequireOne,
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           10 * time.Second,
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Fatalf("close kafka writer: %v", err)
		}
	})

	value := []byte("ping")
	if err := writer.WriteMessages(ctx, kafka.Message{Key: []byte("bluetape"), Value: value}); err != nil {
		t.Fatalf("write kafka message: %v", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		Partition:   0,
		MinBytes:    1,
		MaxBytes:    1024,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Fatalf("close kafka reader: %v", err)
		}
	})

	message, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("read kafka message: %v", err)
	}
	if !bytes.Equal(message.Value, value) {
		t.Fatalf("expected kafka message %q, got %q", value, message.Value)
	}
}

func createTopic(ctx context.Context, t *testing.T, broker string, topic string) {
	t.Helper()

	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		t.Fatalf("connect kafka broker: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close kafka broker connection: %v", err)
		}
	})

	controller, err := conn.Controller()
	if err != nil {
		t.Fatalf("find kafka controller: %v", err)
	}
	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	controllerConn, err := kafka.DialContext(ctx, "tcp", controllerAddr)
	if err != nil {
		t.Fatalf("connect kafka controller: %v", err)
	}
	t.Cleanup(func() {
		if err := controllerConn.Close(); err != nil {
			t.Fatalf("close kafka controller connection: %v", err)
		}
	})

	if err := controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}); err != nil {
		t.Fatalf("create kafka topic: %v", err)
	}
}
