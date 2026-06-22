package bttesting_test

import (
	"context"
	"fmt"
	"time"

	bttesting "github.com/bluetape4k/bluetape-go/testing"
)

func ExampleCheckContextCanceled() {
	err := bttesting.CheckContextCanceled(func(ctx context.Context) error {
		return ctx.Err()
	})

	fmt.Println(err == nil)

	// Output:
	// true
}

func ExampleCheckCleanupOnCancel() {
	err := bttesting.CheckCleanupOnCancel(50*time.Millisecond, func(ctx context.Context, ready func(), cleaned func()) error {
		ready()
		<-ctx.Done()
		cleaned()
		return ctx.Err()
	})

	fmt.Println(err == nil)

	// Output:
	// true
}
