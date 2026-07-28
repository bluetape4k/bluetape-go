# Issue 19 Circuit Breaker and Bulkhead Inventory

## 범위

Issue #19는 기존 `github.com/bluetape4k/bluetape-go/resilience` package에 circuit breaker와
bulkhead policy를 추가한다.

package에는 이미 #18 composition core가 있다.

- `Operation[T] func(context.Context) (T, error)`
- `Policy[T]`와 `PolicyFunc[T]`
- 첫 policy가 outermost wrapper가 되는 `Compose[T]`
- `Run[T]`
- retry, timeout, errors, 작은 synchronous `EventHandler`

## GitHub Evidence

- Epic #2: Go service를 위한 0.2.0 resilience primitive.
- Issue #19: circuit breaker와 bulkhead primitive 구현.
- Milestone: 0.2.0.
- Assignee: `debop`.

## Local Graph Evidence

- 이 worktree에서 CodeGraph 초기화: 102 files, 843 nodes, 1,471 edges.
- 이 worktree에서 code-review-graph 초기화: 99 files, 448 nodes, 3,057 edges.
- CodeGraph context는 integration point가 `Run`, `Compose`, `Policy`, `PolicyFunc`,
  `Operation`, `EventHandler`, `emitEvent`임을 확인했다.

## Reference-Only Inputs

- `sony/gobreaker`: state name, counter, transition callback, half-open admission concept.
- `golang.org/x/sync/semaphore`: bulkhead admission을 위한 weighted semaphore behavior reference.
  이 package의 runtime dependency로 쓰지는 않는다.
- resilience4j: service-level circuit breaker와 bulkhead concept. JVM API shape는 복사하지 않는다.

## 기존 Package Constraints

- circuit breaker 또는 bulkhead를 위해 새 runtime dependency를 추가하지 않는다.
- generic typed operation을 유지한다.
- #21이 API reshaping 없이 observability를 추가할 수 있도록 synchronous event hook shape를 보존한다.
- context cancellation은 permit이나 half-open slot을 leak하면 안 된다.
- test는 deterministic해야 한다. fake clock 또는 explicit function으로 제어할 수 있는 behavior에
  sleep-dependent state transition을 피한다.

## Deferred Scope

- HTTP middleware는 #20.
- Full metrics/OpenTelemetry는 #21.
- rate limiting은 다른 0.2.0 issue가 추가하지 않는 한 later milestone.
