# resilience

`resilience` provides first-party retry, timeout, circuit breaker, and bulkhead
policies for service calls. Policies compose around typed operations and expose
synchronous event hooks for logging, metrics, or tracing bridges.

## Import

```go
import "github.com/bluetape4k/bluetape-go/resilience"
```

## Usage

```go
retry, err := resilience.NewRetry[string](resilience.RetryOptions{
    Name:        "catalog",
    MaxAttempts: 3,
    Backoff:     resilience.ConstantBackoff(50 * time.Millisecond),
})
if err != nil {
    return err
}

timeout, err := resilience.NewTimeout[string](resilience.TimeoutOptions{
    Name:    "catalog",
    Timeout: time.Second,
})
if err != nil {
    return err
}

value, err := resilience.Run(ctx, loadCatalogValue, retry, timeout)
```

## HTTP Adapters

```go
client := http.Client{
    Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
        Transport:       http.DefaultTransport,
        Policies:        []resilience.Policy[*http.Response]{retryPolicy, timeoutPolicy},
        RetryableStatus: resilience.RetryableServerError,
    }),
}
```

Server handlers can be wrapped with `NewHandler` for admission or timeout
policies. Prefer retries on outbound client calls where the request body is
replayable.

## Behavior

- `Compose` applies the first policy as the outermost wrapper.
- Retry predicates can reject errors that should not be retried.
- Timeout distinguishes its own deadline from parent context cancellation.
- Circuit breaker transitions between closed, open, and half-open states.
- Bulkhead limits concurrent admissions and can reject or wait according to
  options.
- `OnEvent` handlers run synchronously on the protected call path.

## Operational Boundaries

- The package does not include an OpenTelemetry exporter.
- Keep event handlers fast and non-blocking.
- HTTP retry requires replayable request bodies.

## Test

```bash
go test -count=1 ./resilience
```
