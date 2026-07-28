# PostgreSQL Rate Limiter 교훈 (2026-07-13)

**Related issue:** #529
**Affected modules:** `ratelimit`, `ratelimit/ratelimittest`, `ratelimit/redis`, `ratelimit/sql`

## L1: 풀 획득도 commit-unknown 경계에 포함된다

### 문제

`DB.QueryRowContext`를 바로 호출하면 provider가 pooled connection 대기 중 취소와
PostgreSQL dispatch 이후 취소를 구분할 수 없다. 둘 다 commit-unknown으로 처리하면
안전하지만, dispatch가 없었다는 강한 결과를 잃고 caller 진단도 약해진다.

### 결정

먼저 caller-owned `*sql.Conn`을 확보한다. 획득 중 취소되면 원래 context error를
반환하고, 다른 획득 실패는 typed determinate provider error로 낸다. query/scan
실패만 dispatch 가능성이 있는 결과로 분류한다.

### 향후 가드

commit-unknown 의미를 공개하는 distributed `database/sql` provider는 pool-wait
cancellation을 row-lock 또는 response-loss cancellation과 별도로 테스트해야 한다.

## L2: `IF NOT EXISTS`에는 hostile catalog preflight가 필요하다

### 문제

`CREATE TABLE/INDEX IF NOT EXISTS`는 기존 객체가 필요한 owner, relation kind,
columns, constraints, index target, RLS policy, trigger state를 가진다는 증거가
아니다. 같은 이름의 hostile object는 migration이 끝난 것처럼 보이게 하고 runtime
traffic이 시작된 뒤에만 실패를 드러낼 수 있다.

### 결정

migration은 caller-owned로 유지하되, 지원 relation 전체에 대해 traffic 전 catalog
proof를 요구한다. 정확한 schema와 hostile mutation을 모두 테스트하고, 다른 relation에
있는 같은 이름 expiry index도 포함한다.

### 향후 가드

모든 fixed-schema SQL provider는 migration constant와 함께 operator-visible catalog
checklist 및 hostile-object integration case를 가져야 한다.

## L3: conformance는 scheduling latency와 refill 의미를 분리해야 한다

### 문제

초당 100 token이면 빠진 token 하나가 10 ms 안에 보충된다. race instrumentation과
database adapter latency가 이 창을 넘으면 debit-preservation case가 정상적으로 refill된
bucket을 보고 contract failure로 오판할 수 있다. 고정 refill sleep은 반대로 scheduler와
server clock이 충분히 전진했다고 가정하는 문제가 있다.

### 결정

refill이 없어야 하는 assertion에는 전체 case timeout보다 긴 refill interval을 쓴다.
refill contract는 bounded deadline 안에서 reject 결과만 재시도하고 provider의 양수
`RetryAfter`를 존중한다.

### 향후 가드

timing test는 negative assertion에서 시간을 무관하게 만들고, positive eventual behavior에는
condition-based bounded wait를 써야 한다. 의심스러운 flaky test는 baseline noise로
분류하기 전에 반복 재현한다.

## L4: cleanup bound와 scan bound는 서로 다른 주장이다

### 문제

expiry ordering에 primary-key tie breaker를 추가하자 expiry index가 있어도 PostgreSQL이
큰 expired backlog를 sort했다. Sort를 제거해도 `SKIP LOCKED`가 최대 `limit` row만
scan한다는 뜻은 아니다. locked row는 여전히 방문되고 skip될 수 있다.

### 결정

indexed expiry column만으로 정렬한다. large-backlog plan이 expiry-index scan을 쓰고 Sort가
없음을 증명한다. `limit`은 lock/delete 수를 제한하고, scan은 caller timeout과 pressure
budget이 제한한다는 점을 문서화한다.

### 향후 가드

performance documentation은 어떤 resource가 제한되는지 정확히 적고, 의존하는 access path는
execution-plan regression으로 고정해야 한다.

## L5: distributed limiter 문서에는 실행 경계 diagram이 필요하다

### 문제

pre-PR review에서는 prose와 table에 contract가 있으므로 새 diagram을 N/A로 표시했다.
증거는 충분했지만 pre-dispatch, atomic row-serialization, outcome, retry 경계를
다시 구성하기가 비쌌다.

### 결정

English/Korean provider README가 공유하는 source-backed sequence asset 하나를 추가한다.
이 asset은 connection acquisition, 단일 UPSERT/RETURNING dispatch, same-key row
serialization, allow/reject/configuration-mismatch outcome, replay 없는 commit-unknown을
보여 준다. cleanup은 별도의 bounded operation으로 유지하고 unsupported capacity claim은
피한다.

### 향후 가드

distributed provider review에서 diagram을 N/A로 둘 수 있는 경우는 atomic linearization
point, failure boundary, retry rule, cleanup ownership이 한눈에 명확할 때뿐이다. 그렇지
않으면 sequence diagram을 기본으로 두고 source geometry와 full-size rendered PNG를 모두
검증한다.
