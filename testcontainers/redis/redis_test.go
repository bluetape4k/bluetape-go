package redistestcontainer_test

import (
	"context"
	"testing"
	"time"

	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

func TestStartRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close redis client: %v", err)
		}
	})

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	const key = "bluetape:testcontainers:redis"
	const value = "ok"
	if err := client.Set(ctx, key, value, 0).Err(); err != nil {
		t.Fatalf("set redis key: %v", err)
	}

	got, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("get redis key: %v", err)
	}
	if got != value {
		t.Fatalf("redis value = %q, want %q", got, value)
	}
}

func TestConnectionDetailKey(t *testing.T) {
	if redistestcontainer.AddressKey != "redis.address" {
		t.Fatalf("AddressKey = %q", redistestcontainer.AddressKey)
	}
}
