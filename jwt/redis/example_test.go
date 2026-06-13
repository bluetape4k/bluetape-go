package redis_test

import (
	"context"
	"time"

	"github.com/bluetape4k/bluetape-go/jwt"
	redisjwt "github.com/bluetape4k/bluetape-go/jwt/redis"
	"github.com/redis/go-redis/v9"
)

func ExampleRepository_distributedHMACProvider() {
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelSetup()

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() { _ = client.Close() }()

	repo, err := redisjwt.New(redisjwt.Options{
		Client:    client,
		Namespace: "service-auth",
	})
	if err != nil {
		panic(err)
	}
	provider, err := jwt.NewDistributedHMACProvider(setupCtx, repo, jwt.HS256)
	if err != nil {
		panic(err)
	}

	opCtx, cancelOp := context.WithTimeout(context.Background(), time.Second)
	defer cancelOp()

	token, err := provider.ComposeContext(opCtx, jwt.WithSubject("account-42"), jwt.WithExpiresAfter(time.Hour))
	if err != nil {
		panic(err)
	}
	reader, err := provider.ParseContext(opCtx, token, jwt.WithExpectedSubject("account-42"))
	if err != nil {
		panic(err)
	}
	_ = reader
}

func ExampleRepository_distributedRSAProvider() {
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelSetup()

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() { _ = client.Close() }()

	repo, err := redisjwt.New(redisjwt.Options{
		Client:    client,
		Namespace: "service-auth",
	})
	if err != nil {
		panic(err)
	}
	provider, err := jwt.NewDistributedRSAProvider(setupCtx, repo, jwt.RS256)
	if err != nil {
		panic(err)
	}

	opCtx, cancelOp := context.WithTimeout(context.Background(), time.Second)
	defer cancelOp()

	token, err := provider.ComposeContext(opCtx, jwt.WithAudience("api"), jwt.WithExpiresAfter(time.Hour))
	if err != nil {
		panic(err)
	}
	reader, err := provider.ParseContext(opCtx, token, jwt.WithExpectedAudience("api"))
	if err != nil {
		panic(err)
	}
	_ = reader
}

func ExampleRepository_contextTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() { _ = client.Close() }()

	repo, err := redisjwt.New(redisjwt.Options{
		Client:    client,
		Namespace: "service-auth",
	})
	if err != nil {
		panic(err)
	}
	provider, err := jwt.NewDistributedHMACProvider(ctx, repo, jwt.HS256)
	if err != nil {
		panic(err)
	}
	if _, err := provider.CurrentKeyChainContext(ctx); err != nil {
		panic(err)
	}
}
