# Redis Streams Primitive: Delivery Policy 없이 Command만 공유한다

## Context

Issue #571은 `redis/stream`을 추가하고 `audit/sqloutbox/redisstreams` 안의 `XADD`
dispatch만 migration했다. provider는 이미 안정적인 audit envelope와
duplicate-attempt behavior를 소유하고 있었다. shared package는 반복되던 command
safety behavior만 제거하면 됐다.

## Lesson

provider-backed Redis Streams feature에서는 command mechanics만 공유하고 delivery
policy와 domain data는 provider 또는 application boundary에 둔다.

- primitive는 좁은 command interface, argument validation, nil/typed nil
  detection, context preflight, redacted typed error를 소유한다.
- provider는 record field, payload encoding, default stream selection,
  idempotency identity, relay retry/dead-letter policy, public API를 소유한다.
- application은 consumer group, idle threshold, `XAUTOCLAIM` cursor persistence,
  replay, retention, consumer shutdown을 소유한다.

이렇게 하면 generic message-bus abstraction을 피하면서도 모든 caller에게 일관된
cancellation/error contract를 줄 수 있다.

## Cancellation Rule

Redis blocking read는 caller deadline이 이미 끝난 뒤 Redis nil-style result로 완료될
수 있다. provider cause를 context cause로 대체하지 않는다. redacted operational
error로 감싸기 전에 둘을 join해서 caller가 provider result 또는
`context.DeadlineExceeded`/`context.Canceled` 어느 쪽에도 `errors.Is`를 사용할 수
있게 한다.

## Test Isolation Rule

Consumer group은 stream key 아래 저장된다. cleanup이 있어도 static test stream name은
normal test process와 race test process를 교차 오염시킬 수 있다. Testcontainers
fixture invocation마다 unique suffix를 사용하고 test-owned stream key만 정리한다.

## Benchmark Boundary

이 issue는 throughput claim이 아니라 command behavior와 provider reuse를 추가한다.
여기에는 benchmark가 적절하지 않다. provider comparison은 issue #560이 소유하며,
측정을 실행할 때 필요한 result table, chart, written analysis를 함께 publish해야
한다.
