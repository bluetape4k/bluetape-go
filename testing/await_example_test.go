package bttesting_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func ExampleCheckAwaitValue_cacheInvalidation() {
	var staleReads atomic.Int32

	result, err := bttesting.CheckAwaitValue(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (string, error) {
			if staleReads.Add(1) < 3 {
				return "old", nil
			}
			return "new", nil
		},
		"new",
	)

	fmt.Println(err == nil, result.Value)

	// Output:
	// true new
}

func ExampleCheckAwait_lockAcquisition() {
	var locked atomic.Bool
	locked.Store(true)

	result, err := bttesting.CheckAwait(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (bool, error) {
			if result := locked.Swap(false); result {
				return false, nil
			}
			return true, nil
		},
		func(acquired bool, err error) bttesting.AwaitStatus {
			if err != nil {
				return bttesting.AwaitFailure
			}
			if acquired {
				return bttesting.AwaitSuccess
			}
			return bttesting.AwaitContinue
		},
	)

	fmt.Println(err == nil, result.Value)

	// Output:
	// true true
}

func ExampleCheckAwait_containerReadiness() {
	var probes atomic.Int32

	result, err := bttesting.CheckAwait(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (string, error) {
			if probes.Add(1) < 2 {
				return "starting", errors.New("connection refused")
			}
			return "ready", nil
		},
		func(state string, err error) bttesting.AwaitStatus {
			if err == nil && state == "ready" {
				return bttesting.AwaitSuccess
			}
			return bttesting.AwaitContinue
		},
	)

	fmt.Println(err == nil, result.Value)

	// Output:
	// true ready
}

func ExampleCheckAwaitValue_workflowStatus() {
	statuses := []string{"queued", "running", "completed"}
	var index atomic.Int32

	result, err := bttesting.CheckAwaitValue(
		context.Background(),
		50*time.Millisecond,
		time.Millisecond,
		func(context.Context) (string, error) {
			next := int(index.Add(1)) - 1
			if next >= len(statuses) {
				next = len(statuses) - 1
			}
			return statuses[next], nil
		},
		"completed",
	)

	fmt.Println(err == nil, result.Value)

	// Output:
	// true completed
}
