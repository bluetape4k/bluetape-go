# Redis fenced lock/semaphore 교훈 (#572)

## L1: fencing token은 lease보다 외부 resource boundary에 속한다

Lock TTL이 만료되면 이전 holder의 작업은 자동으로 멈추지 않는다. 따라서 새
holder의 fencing token을 발급하는 것만으로 stale 작업을 차단할 수 없다. 외부
resource가 마지막으로 허용한 token을 저장하고 낮은 token을 거부해야 한다. 이
경계를 `doc.go`, README, example에 반복해 적으면 lock을 일반 mutual exclusion
기능으로 오용하는 위험을 줄일 수 있다.

## L2: mutation ambiguity는 context error와 별도로 보존한다

Redis dispatch 뒤 cancellation/deadline이 오면 command가 commit되었는지 caller가
알 수 없다. Lock은 짧은 background probe로 owner/counter를 확인해 확정 가능한
lease를 복원하고, 그렇지 않으면 원인과 `ErrCommitUnknown`을 함께 반환한다.
Semaphore는 exact owner member를 직접 재확인하도록 `ErrCommitUnknown`을 보존한다.
단순히 context error만 반환하면 permit/lease 누수를 숨길 수 있다.

## L3: same-slot key 설계는 primitive 계약의 일부다

Lock의 owner key와 persistent counter key는 동일한 digest hash tag를 공유해야
cluster Lua script가 원자적으로 실행된다. Caller key를 raw Redis key로 남기지
않으면서 같은 slot을 보장하려면 shared `KeyBuilder`와 redacted digest를 같이
사용해야 한다. Key layout 테스트에서 두 key의 hash tag equality를 직접 고정한다.

## L4: TTL expiry와 wait cancellation을 함께 검증한다

Semaphore는 acquire마다 Redis server time으로 expired sorted-set member를 먼저
정리한다. Local timer나 wall-clock으로 capacity를 추정하지 않는다. Blocking
`Acquire`는 `ErrNotAcquired`만 bounded backoff하고 context deadline에서 즉시
돌아와야 하며, canceled waiter가 permit member를 남기지 않는 실제 Redis 테스트와
race 테스트를 함께 둔다.

## L5: 작은 공개 surface와 bilingual 운영 문서가 안전성을 높인다

`TryAcquire`/`Acquire`, owner-safe `Release`, nil-safe accessors만 공개하고
watchdog, Redlock, FIFO fairness 같은 확장 기능은 추가하지 않았다. English와
Korean README 모두 cleanup context, commit-unknown 재확인, over-TTL overlap,
semaphore의 no-fencing 경계를 동일하게 설명해야 소비자가 언어별 문서에서 서로
다른 보장으로 오해하지 않는다.
