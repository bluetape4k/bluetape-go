package cache_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
)

func ExampleNewMemory_getOrLoad() {
	localCache := cache.NewMemory[string, string]()
	ctx := context.Background()

	value, err := localCache.GetOrLoad(ctx, "catalog", time.Minute, func(context.Context, string) (string, error) {
		return "warm value", nil
	})
	if err != nil {
		return
	}
	fmt.Println(value)

	_, err = localCache.Get(ctx, "missing")
	fmt.Println(errors.Is(err, cache.ErrCacheMiss))

	// Output:
	// warm value
	// true
}
