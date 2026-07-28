# Redis NearCache Lessons (2026-06-04)

**Related issue**: #23
**Impact modules**: `cache/redisnear`, `cache`, `testcontainers/redis`

## L1: Testcontainers log readiness가 항상 connection readiness는 아니다

### 문제

첫 Redis NearCache package test는 Redis container helper가 address를 반환한 직후
`connect: connection refused`로 한 번 실패했다.

### 교훈

Redis Pub/Sub integration test는 subscriber를 만들기 전에 package-level test helper
안에서 가벼운 `PING` readiness check를 추가한다. shared container fixture는 그대로
두면서 timing-sensitive subscriber startup test를 보호한다.

### Evidence

- initial `go test -count=1 ./cache ./cache/redisnear`는
  `TestNearCacheInvalidatesPeerEntries`에서 실패했다.
- `waitForRedis` 추가 후 `go test -count=1 ./cache ./cache/redisnear`가 통과했다.

## L2: failure-mode behavior는 design note가 아니라 direct test가 필요하다

### 문제

spec은 receive error에서 local cache를 clear하고 `OnError`를 호출해야 한다고
요구했지만, 첫 test pass는 malformed message와 close semantics만 다뤘다.

### 교훈

Near-cache invalidation test는 data path와 failure path를 모두 포함해야 한다. peer
invalidation, malformed payload reporting, receive-error local clear, close
idempotency, stress, cancellation을 함께 고정한다.

### Evidence

- `TestNearCacheClearsLocalOnReceiveError` 추가.
- `go test -count=1 ./cache/redisnear` 통과.
- `go test -race -count=1 ./cache/redisnear` 통과.

## L3: example도 production code와 같은 errcheck gate를 만족해야 한다

### 문제

compile-only `ExampleNewPubSub`는 처음에 bare `defer client.Close()`와
`defer near.Close()`를 사용했다. `make ci`는 lint 단계에서 errcheck finding 2개로
실패했다.

### 교훈

example이 의도적으로 작더라도 close call은 deferred function 안에서 처리하고,
cleanup이 example result에 영향을 줄 수 없을 때는 error를 명시적으로 discard한다.

### Evidence

- initial `make ci`는 `cache/redisnear/example_test.go`에서 실패했다.
- deferred close를 감싼 뒤 `make ci`가 `0 issues`로 통과했다.

## L4: NearCache review는 local method뿐 아니라 peer behavior를 stress해야 한다

### 문제

첫 stress test는 `NearCache` instance 하나만 exercise했다. race와 CI는 통과했지만
실제 near-cache risk인 두 peer의 invalidation exchange와 concurrent `GetOrLoad`
상황을 압박하지 못했다.

### 교훈

Redis near-cache stress coverage는 적어도 두 Redis-backed peer, 양쪽의 concurrent
mutating operation, invalidation pressure 아래의 peer read/loader를 포함해야 한다.

### Evidence

- hard PR review가 이를 P1로 발견했다.
- `TestNearCacheConcurrentStress`는 이제 두 peer `NearCache` instance를 사용한다.

## L5: background loop의 observer hook은 isolation이 필요하다

### 문제

`OnError`는 원래 subscriber loop에서 inline으로 실행됐다. blocking handler는
invalidation processing을 지연시킬 수 있고 panic은 goroutine을 종료할 수 있었다.

### 교훈

background lifecycle loop는 diagnostic hook을 protocol processing과 격리해야 한다.
loss가 허용되는 bounded queue를 사용하고, handler panic을 recover하며,
best-effort contract를 문서화한다.

### Evidence

- `TestNearCacheOnErrorDoesNotBlockSubscriber`.
- `TestNearCacheOnErrorPanicIsRecovered`.
