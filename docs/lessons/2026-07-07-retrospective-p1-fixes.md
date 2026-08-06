# Retrospective P1 fix 교훈

일자: 2026-07-07 KST
관련 이슈: #425
영향 package: `cache`, `ratelimit/redis`

## L1: Same-key load collapse가 한 caller의 cancellation을 빌리면 안 된다

### 문제

`cache.Memory.GetOrLoad`는 같은 key caller들의 shared `singleflight` load에 첫 caller의
context를 사용했다. Owner caller가 cancel되고 다른 waiter는 아직 live context를 갖고 있을 때,
waiter는 owner의 `context.Canceled`를 받았다.

### 교훈

Same-key stampede protection에서 cancellation은 caller-owned다. Live waiter는 shared owner
call이 실패했다는 사실을 관찰할 수는 있지만, 다른 caller의 cancellation을 상속하지 않고 자기
context로 retry할 수 있어야 한다.

### 증거

- `TestMemorySameKeyCanceledOwnerDoesNotCancelLiveWaiter`를 추가했다.
- Fix 전 test는 `live waiter should retry after owner cancellation, got context canceled`로
  실패했다.
- `go test -count=1 ./cache ./ratelimit/redis`: PASS.
- `go test -race -count=1 ./cache ./ratelimit/redis`: PASS.

## L2: Redis logical key validation은 caller-owned storage byte를 보존해야 한다

### 문제

`ratelimit/redis`는 Redis bucket key를 만들기 전에 logical key를 trim했다. README가
`<key>`를 caller-provided logical key로 문서화했는데도 `"tenant:blue"`와
`" tenant:blue "`가 같은 bucket으로 합쳐졌다.

### 교훈

Redis package는 blank input을 거부하기 위해 `strings.TrimSpace(key)`를 검사할 수 있다.
하지만 public API가 canonicalization을 명시하고 test하지 않는 한 storage와 collision
behavior는 정확히 caller-provided key를 사용해야 한다.

### 증거

- `TestLimiterPreservesCallerOwnedKeys`를 추가했다.
- Fix 전 test는 spaced key가 trimmed bucket을 재사용해 rejected되면서 실패했다.
- `go test -count=1 ./cache ./ratelimit/redis`: PASS.
- `go test -race -count=1 ./cache ./ratelimit/redis`: PASS.
