# MongoDB Single Leader Elector 교훈

## L1: 테스트에서도 TTL은 cleanup으로만 둔다

MongoDB elector는 TTL monitor를 기다리지 않고 expired document를 takeover할 수 있다.
테스트는 `lease_until`을 직접 전진시키거나 기록해 normal query predicate가 expiry를
처리함을 증명해야 한다.

Prevention:

- 기존 document가 남아 있는 상태에서 expired takeover를 테스트한다.
- `EnsureIndexes`는 optional로 유지하고 cleanup support로만 문서화한다.
- MongoDB TTL deletion을 기다리는 sleep을 추가하지 않는다.

## L2: Campaign semantics는 의도적으로 Redis와 다르다

Redis `Campaign`은 다른 member가 key를 소유하면 즉시 `ErrNotLeader`를 반환한다.
Issue #485는 MongoDB `Campaign(ctx)`가 acquisition 또는 context cancellation까지
기다리도록 요구한다.

Prevention:

- MongoDB `Campaign(ctx)`를 wait-until-acquired로 문서화한다.
- caller가 wait를 bound할 수 있도록 cancellation을 명시적으로 테스트한다.
- duplicate local campaign protection과 remote ownership wait를 분리한다.

## L3: Testcontainers contention test에는 suite당 container 하나가 필요하다

subtest마다 MongoDB container를 시작하면 package가 느리고 noisy해진다. 하나의
container와 subtest별 collection을 사용하면 serial Testcontainers execution을
유지하면서 test isolation을 보존할 수 있다.

Prevention:

- integration suite에서 MongoDB container 하나를 만든다.
- subtest에는 unique collection name을 사용한다.
- bounded cleanup context로 collection을 drop한다.
