package resilience_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

func ExampleRetryOptions_slogOnEvent() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	retry, err := resilience.NewRetry[string](resilience.RetryOptions{
		Name:        "catalog",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		OnEvent: func(ctx context.Context, event resilience.Event) {
			logger.LogAttrs(ctx, slog.LevelInfo, "resilience event",
				slog.String("policy", event.PolicyName),
				slog.String("policy_type", event.PolicyType),
				slog.String("kind", string(event.Kind)),
				slog.String("category", string(event.Category)),
				slog.Int("attempt", event.Attempt),
				slog.Duration("delay", event.Delay),
				slog.String("error_category", string(event.ErrorCategory)),
			)
		},
	})
	if err != nil {
		return
	}

	var attempts int
	result, err := resilience.Run(ctx, func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("temporary")
		}
		return "ok", nil
	}, retry)
	if err != nil {
		return
	}

	fmt.Println(result, attempts)

	// Output:
	// ok 2
}
