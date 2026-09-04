package redisbucket_test

import (
	"fmt"
	"io"
	"log/slog"

	redisbucket "github.com/bluetape4k/bluetape-go/redis/bucket"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

func ExampleNew() {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() { _ = client.Close() }()
	bucket, err := redisbucket.New(client, redisbucket.Options[string]{
		Namespace:  "example",
		Serializer: serialization.StringSerializer{},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		panic(err)
	}
	_ = bucket
	fmt.Println("durable Redis primitive; caller owns near-cache and stampede policy")
	// Output:
	// durable Redis primitive; caller owns near-cache and stampede policy
}
