package redis

import (
	"testing"

	jwt "github.com/bluetape4k/bluetape-go/jwt"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisFacadeNewReturnsRepository(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })

	repo, err := New(Options{Client: client, Namespace: "prod"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := any(repo).(*jwt.RedisRepository); !ok {
		t.Fatalf("New() returned %T, want *jwt.RedisRepository", repo)
	}
}
