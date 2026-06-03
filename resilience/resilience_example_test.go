package resilience_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func ExampleRun() {
	retry, err := resilience.NewRetry[string](resilience.RetryOptions{
		MaxAttempts: 3,
		Backoff:     resilience.NoBackoff(),
	})
	if err != nil {
		return
	}
	timeout, err := resilience.NewTimeout[string](resilience.TimeoutOptions{
		Timeout: time.Second,
	})
	if err != nil {
		return
	}

	var attempts int
	result, err := resilience.Run(context.Background(), func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("temporary")
		}
		return "ok", nil
	}, retry, timeout)
	if err != nil {
		return
	}

	fmt.Println(result, attempts)

	// Output:
	// ok 2
}
