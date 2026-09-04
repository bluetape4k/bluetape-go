package redissem_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	redissem "github.com/bluetape4k/bluetape-go/redis/semaphore"
)

func Example_cleanupContext() {
	var lease *redissem.Lease
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	released, err := lease.Release(cleanupCtx)
	fmt.Println(released, err == nil)
	_ = errors.Is(err, btredis.ErrCommitUnknown)
	// Output:
	// false true
}

func Example_overTTLOverlap() {
	fmt.Println("semaphore expiry is not external-resource fencing")
	// Output:
	// semaphore expiry is not external-resource fencing
}
