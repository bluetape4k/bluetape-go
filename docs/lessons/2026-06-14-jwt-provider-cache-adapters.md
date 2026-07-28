# JWT Provider Cache Adapter 교훈 (2026-06-14)

**Related issue**: #175
**Related PR**: #230
**Affected modules**: `jwt`, `jwt/redis`, `testing/concurrency`

## L1: Cached JWT reader에는 stale-hit revalidation test가 필요하다

### 문제

cached `jwt.Reader`는 key rotation, algorithm 변경, key id 제거 뒤 invalid해질 수
있다. warm-hit test만으로는 stale cached reader가 삭제되고 live provider path로 다시
parse된다는 것을 증명하지 못한다.

### 교훈

JWT cache adapter에서는 local 및 distributed stale-hit branch를 모두 test한다.
invalid reader state로 cache를 seed하고 delete/reparse behavior를 검증하며, nil
reader, wrong algorithm, unknown key id, expired-key 또는 expired-token no-recache
case를 포함한다.

### 증거

- `jwt/cached_provider_test.go`
- `jwt/cached_distributed_provider_test.go`
- stale-hit 및 TTL proof test를 추가한 뒤 Step 6-R Security lane은 `P0=0 P1=0`에
  도달했다.

## L2: Cancellation test는 cache write가 완료되지 않았음을 증명해야 한다

### 문제

async cancellation test는 caller가 canceled된 것만 관찰하고도 통과할 수 있다. 그동안
in-flight cache owner가 여전히 `Set`을 완료해 stale entry를 남길 수 있다. 이는
context-aware cache API의 correctness gap이다.

### 교훈

cache adapter의 cancellation test는 caller-visible error와 storage side effect를 모두
assert해야 한다. 맞는 곳에는 repository concurrency helper를 사용하고, helper가 반환된
뒤 completed `Set`과 retained entry가 없음을 assert한다.

### 증거

- `jwt/cache_failure_test.go` uses `AsyncJobTester`.
- Security rerun attempt 2가 P2 proof gap을 찾았고, main integration이 `sets == 0`와
  `entries == 0` assertion을 추가했다.
- assertion fix 뒤 fresh targeted test, race test, `make ci`가 통과했다.

## L3: Review lane timeout은 즉시 실패가 아니라 recoverable work다

### 문제

review perspective가 여전히 유용해도 native subagent lane은 10-minute SLA를 넘을 수
있다. lane을 너무 빨리 final timeout으로 남기면 runtime delay가 약한 review
evidence로 바뀐다.

### 교훈

Step 2-R, Step 3-R, Step 6-R, Step 7-R에서는 timed-out lane을 닫고 final
main-session fallback 전에 같은 perspective를 fresh gate-scoped agent로 최대 3회
다시 실행한다. subagent가 실행되는 동안 main session은 local verification을 계속해야
한다.

### 증거

- Step 6-R Security attempt 1은 10-minute SLA 뒤 timeout됐다.
- Security attempt 2는 `P0=0 P1=0 P2=1 P3=0`로 완료됐다.
- `bluetape4k-workflow`와 `bluetape4k-full-feature`는 live skill file 및 chezmoi
  source에서 같은 retry-then-fallback rule로 갱신됐다.
