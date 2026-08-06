# Issue 24 Redis Distributed Lock Lessons

Date: 2026-06-04 KST
Related issue: #24
Affected package: `lock/redis`

## L1: caller-owned Redis key를 보존한다

### 문제

첫 구현은 validation과 storage 모두에 `strings.TrimSpace`를 사용했다. spec은
trim-based blank validation만 의도했으므로 `" locks:billing-rollup "` 같은 caller
key가 조용히 바뀔 수 있었다.

### 교훈

Redis key는 validation에서 normalized value를 검사할 수 있지만, API가
canonicalization을 명시하지 않는 한 package는 caller가 제공한 exact key를 저장하고
사용해야 한다.

### Evidence

- `TestNewPreservesRedisKeyVerbatim` 추가.
- `go test -count=1 ./lock/redis`: PASS, 15 tests.
- `make ci`: PASS.

## L2: stress probe가 자체 race window를 만들면 안 된다

### 문제

same-key contention stress test는 Redis key release 뒤 active-owner counter를
decrement했다. 그 작은 gap에서 새 owner가 key를 acquire하면 Redis ownership이
안전해도 false positive가 발생할 수 있었다.

### 교훈

stress test가 critical-section overlap을 측정할 때는 external lock을 release하기
전에 test-level critical section을 끝낸다. 그렇지 않으면 probe가 product race가
아닌 test instrumentation race를 보고할 수 있다.

### Evidence

- `go test -count=5 ./lock/redis -run 'TestMutexSameKeyContentionStress|TestMutexAsyncCancellationDoesNotLeakKey'`: PASS, 10 runs.
- `make ci`: PASS.
