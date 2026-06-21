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

## 운영 경계

- 이 패키지는 OpenTelemetry exporter를 포함하지 않습니다.
- Event handler는 빠르고 non-blocking하게 유지하세요.
- HTTP retry에는 replayable request body가 필요합니다.

## 테스트

```bash
go test -count=1 ./resilience
```
