package redisbloom_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	redisbloom "github.com/bluetape4k/bluetape-go/probabilistic/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

type cuckooExampleClient struct{ lost bool }

func (c *cuckooExampleClient) Do(ctx context.Context, args ...any) *redis.Cmd {
	cmd := redis.NewCmd(ctx, args...)
	if c.lost {
		cmd.SetErr(io.EOF)
		return cmd
	}
	switch args[0] {
	case "CF.RESERVE":
		cmd.SetVal("OK")
	case "CF.INSERT":
		cmd.SetVal([]any{true})
	case "CF.EXISTS":
		cmd.SetVal(true)
	}
	return cmd
}

func ExampleNewCuckoo() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// 실제 client는 MaxRetries:-1과 IO timeout을 설정하고 호출자가 Close한다.
	client := &cuckooExampleClient{}
	filter, err := redisbloom.NewCuckoo(redisbloom.CuckooOptions{Client: client, Namespace: "example"})
	if err != nil {
		panic(err)
	}
	if err = filter.Reserve(ctx, redisbloom.CuckooReserveOptions{Capacity: 64, Expansion: 0}); err != nil {
		panic(err)
	}
	known := 0
	if err = filter.Add(ctx, "item"); err == nil {
		known++
	}
	fmt.Println("known successful insertions:", known)
	exists, err := filter.Exists(ctx, "item")
	if err != nil {
		panic(err)
	}
	fmt.Println("may exist:", exists)
	client.lost = true
	err = filter.Add(ctx, "item")
	if err == nil {
		known++
	}
	// 응답 유실은 성공 장부에 넣지 않으며 같은 mutation이나 Delete를 자동 실행하지 않는다.
	fmt.Println("commit unknown:", errors.Is(err, btredis.ErrCommitUnknown))
	fmt.Println("known successful insertions:", known)
	// Output:
	// known successful insertions: 1
	// may exist: true
	// commit unknown: true
	// known successful insertions: 1
}
