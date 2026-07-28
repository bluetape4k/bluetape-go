# Issue 18 Resilience Core Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 목표

Provide the first usable slice of `github.com/bluetape4k/bluetape-go/resilience`:
composable policy core plus retry and timeout policies.

## Constraints

- No runtime dependency on `failsafe-go`, `cenkalti/backoff`, resilience4j, or
  another resilience package.
- Public API must stay Go-native and context-aware.
- Policy composition must support future circuit breaker, bulkhead,
  observability, and HTTP integration without reshaping #18 APIs.
- Tests must avoid flaky timing. Use fake sleepers where possible and bounded
  deadlines where real context timeouts are the behavior under test.

## API Contract

- `Operation[T]` is `func(context.Context) (T, error)`.
- `Policy[T]` wraps an operation and returns another operation.
- `Compose[T]` applies policies in listed order, with the first policy
  outermost.
- `Run[T]` composes policies and executes the protected operation.
- `NewRetry[T]` returns a retry policy configured by `RetryOptions`.
- `NewTimeout[T]` returns a timeout policy configured by `TimeoutOptions`.

## Retry Contract

- `MaxAttempts` must be positive.
- A nil backoff defaults to no delay.
- A nil predicate retries every non-nil error except `context.Canceled`.
- Bare `context.DeadlineExceeded` is not retried by default.
- Timeout policy errors are retryable by default.
- Retry exhaustion returns `RetryError`.
- `errors.Is(err, ErrRetryExhausted)` must work.
- The final operation error remains reachable through `errors.Is` /
  `errors.As`.
- Backoff sleep must respect context cancellation.

## Timeout Contract

- Timeout duration must be positive.
- Timeout composes by deriving a child context with `context.WithTimeout`.
- Timeout is cooperative: operations must observe the provided context.
- If the timeout policy's own child deadline expires, return `TimeoutError`.
- `errors.Is(err, ErrTimeout)` must work.
- `context.DeadlineExceeded` remains reachable through error unwrapping.
- Parent cancellation or parent deadline must not be mislabeled as this
  policy's timeout.

## Event Skeleton

#18 introduces stable hook shapes for #21:

- event kind
- policy name
- policy type
- attempt number
- retry delay
- associated error

#18 only requires success, retry, and timeout events.

## Test Contract

- Retry succeeds after transient failure.
- Retry exhaustion preserves the final cause.
- Retry predicate can reject an error without another attempt.
- Retry backoff sleep returns context cancellation.
- Exponential backoff supports cap and deterministic jitter.
- Timeout wraps its own deadline.
- Parent cancellation is not reported as timeout.
- Retry + timeout composition retries timeout failures when retry is outermost.
- Example demonstrates retry + timeout composition.
