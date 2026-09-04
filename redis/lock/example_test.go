package redislock_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	redislock "github.com/bluetape4k/bluetape-go/redis/lock"
)

type fencedResource struct {
	last uint64
}

func (r *fencedResource) Write(token uint64) error {
	if token <= r.last {
		return errors.New("stale fencing token")
	}
	r.last = token
	return nil
}

func Example_fencingToken() {
	resource := &fencedResource{}
	_ = resource.Write(12)
	freshErr := resource.Write(13)
	staleErr := resource.Write(12)
	fmt.Println(freshErr == nil, staleErr)
	// Output:
	// true stale fencing token
}

func Example_cleanupContext() {
	var lease *redislock.Lease
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	released, err := lease.Release(cleanupCtx)
	fmt.Println(released, err == nil)
	_ = errors.Is(err, btredis.ErrCommitUnknown)
	// Output:
	// false true
}

func Example_overTTLOverlap() {
	fmt.Println("lease expiry is not external-resource fencing")
	// Output:
	// lease expiry is not external-resource fencing
}
