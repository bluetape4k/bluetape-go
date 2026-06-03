package resilience_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func ExampleNewCircuitBreaker() {
	breaker, err := resilience.NewCircuitBreaker[string](resilience.CircuitBreakerOptions{
		FailureThreshold: 2,
		OpenTimeout:      time.Second,
	})
	if err != nil {
		panic(err)
	}

	_, _ = resilience.Run(context.Background(), func(context.Context) (string, error) {
		return "", errors.New("first failure")
	}, breaker)
	_, _ = resilience.Run(context.Background(), func(context.Context) (string, error) {
		return "", errors.New("second failure")
	}, breaker)

	_, err = resilience.Run(context.Background(), func(context.Context) (string, error) {
		return "unreachable", nil
	}, breaker)

	fmt.Println(errors.Is(err, resilience.ErrCircuitOpen))
	// Output: true
}

func ExampleNewBulkhead() {
	bulkhead, err := resilience.NewBulkhead[string](resilience.BulkheadOptions{
		MaxConcurrent: 1,
	})
	if err != nil {
		panic(err)
	}

	result, err := resilience.Run(context.Background(), func(context.Context) (string, error) {
		return "ok", nil
	}, bulkhead)

	fmt.Println(result, err)
	// Output: ok <nil>
}
