# resilience

[English](README.md) | [한국어](README.ko.md)

`resilience`는 service call을 위한 자체 retry, timeout, circuit breaker, bulkhead
policy를 제공합니다. Policy는 typed operation 주변에 compose되며 logging, metrics,
tracing bridge를 위한 synchronous event hook을 노출합니다.

## 다이어그램

![resilience policy chain flow](../docs/images/readme-diagrams/resilience-policy-chain-flow.png)

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/resilience"
```

## 사용 예

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

## HTTP Adapter

```go
client := http.Client{
    Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
        Transport:       http.DefaultTransport,
        Policies:        []resilience.Policy[*http.Response]{retryPolicy, timeoutPolicy},
        RetryableStatus: resilience.RetryableServerError,
    }),
}
```

Server handler는 admission 또는 timeout policy를 위해 `NewHandler`로 감쌀 수
있습니다. Request body를 replay할 수 있는 outbound client call에 retry를 우선
적용하세요.

## 동작

- `Compose`는 첫 policy를 outermost wrapper로 적용합니다.
- Retry predicate는 retry하면 안 되는 error를 거부할 수 있습니다.
- Timeout은 자신의 deadline과 parent context cancellation을 구분합니다.
- Circuit breaker는 closed, open, half-open state 사이를 전환합니다.
- Bulkhead는 concurrent admission을 제한하고 option에 따라 reject 또는 wait할 수
  있습니다.
- `OnEvent` handler는 protected call path에서 synchronous로 실행됩니다.

## slog Bridge

Application이 `log/slog` handler를 설정하고 package-local bridge를 `OnEvent`에
전달합니다. 이 library는 global logging default를 바꾸거나 logger registry를
소유하지 않습니다.

아래 snippet은 `context`, `log/slog`, `time` 같은 standard import를 가정합니다.
Compile-checked 버전은 [`resilience_example_test.go`](resilience_example_test.go)에
있습니다.

```go
logger := slog.Default()
retry, err := resilience.NewRetry[string](resilience.RetryOptions{
    Name:        "catalog",
    MaxAttempts: 3,
    Backoff:     resilience.ConstantBackoff(50 * time.Millisecond),
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
```

## 운영 경계

- 이 패키지는 OpenTelemetry exporter를 포함하지 않습니다.
- Application이 `slog` record를 OpenTelemetry로 내보내야 한다면 application
  setup에서 공식 `go.opentelemetry.io/contrib/bridges/otelslog` handler를
  연결하고, 만들어진 `slog.Logger`를 `OnEvent`로 전달하세요.
- Event handler는 빠르고 non-blocking하게 유지하세요.
- High-volume success event는 synchronous hook에서 log로 내보내기 전에 filter
  또는 sample하세요.
- Application이 `slog` handler 설정을 소유합니다. 이 package는 caller-owned
  synchronous event만 전달합니다.
- HTTP retry에는 replayable request body가 필요합니다.

## 테스트

```bash
go test -count=1 ./resilience
```
