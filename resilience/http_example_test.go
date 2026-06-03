package resilience_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func ExampleNewRoundTripper() {
	var events []string
	onEvent := func(_ context.Context, event resilience.Event) {
		events = append(events, string(event.Category))
	}

	retry, _ := resilience.NewRetry[*http.Response](resilience.RetryOptions{ //nolint:bodyclose
		Name:        "catalog-http",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		OnEvent:     onEvent,
	})
	timeout, _ := resilience.NewTimeout[*http.Response](resilience.TimeoutOptions{ //nolint:bodyclose
		Name:    "catalog-http",
		Timeout: time.Second,
		OnEvent: onEvent,
	})
	breaker, _ := resilience.NewCircuitBreaker[*http.Response](resilience.CircuitBreakerOptions{ //nolint:bodyclose
		Name:             "catalog-http",
		FailureThreshold: 2,
		OpenTimeout:      time.Second,
		OnEvent:          onEvent,
	})

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := http.Client{
		Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
			Transport:       http.DefaultTransport,
			Policies:        []resilience.Policy[*http.Response]{retry, timeout, breaker},
			RetryableStatus: resilience.RetryableServerError,
		}),
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		return
	}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, _ := io.ReadAll(response.Body)

	fmt.Println(string(body))
	fmt.Println(strings.Join(events, ","))

	// Output:
	// ok
	// retry,success,success
}

func ExampleNewHandler() {
	bulkhead, _ := resilience.NewBulkhead[struct{}](resilience.BulkheadOptions{
		Name:          "workers",
		MaxConcurrent: 1,
	})

	handler := resilience.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("handled"))
	}), resilience.HandlerOptions{
		Policies: []resilience.Policy[struct{}]{bulkhead},
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	fmt.Println(recorder.Body.String())

	// Output:
	// handled
}
