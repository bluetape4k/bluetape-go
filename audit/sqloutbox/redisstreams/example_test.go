package redisstreams_test

import (
	"context"
	"fmt"

	redisstreams "github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
	"github.com/redis/go-redis/v9"
)

func ExampleNew() {
	publisher, err := redisstreams.New(redisstreams.Options{
		Client: exampleClient{},
		Stream: "audit:sqloutbox",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(publisher.Stream())
	// Output: audit:sqloutbox
}

type exampleClient struct{}

func (exampleClient) XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd {
	return redis.NewStringResult("1-0", nil)
}
