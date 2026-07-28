# MongoDB Leader Storage Research 교훈

## L1: TTL cleanup은 lease correctness가 아니다

MongoDB TTL index는 document를 비동기로 삭제하므로 garbage collection에만 유용하다.
leader backend는 일반 read/write의 `lease_until` predicate로 acquisition과
observation을 판단해야 한다.

Prevention:

- `lease_until <= now`를 takeover condition으로 취급한다.
- `lease_until > now`만 active-leader read condition으로 취급한다.
- TTL index creation은 optional 또는 cleanup support로 문서화하고 coordination
  mechanism으로 취급하지 않는다.

## L2: group 또는 strategy variant보다 MongoDB elector 한 형태를 먼저 배포한다

Redis는 single, group, strategic elector를 지원하지만 MongoDB가 그 형태를
기계적으로 상속해서는 안 된다. single-elector ownership은 하나의 lease document에
맞는다. group과 strategic elector에는 다른 concurrency proof가 필요하다.

Prevention:

- 첫 MongoDB implementation issue는 `leader.Elector`만 다룬다.
- 정확한 `MaxLeaders` slot design이 작성될 때까지 `GroupElector`를 미룬다.
- candidate registry와 pruning semantics가 독립적으로 설계될 때까지
  `StrategicElector`를 미룬다.

## L3: Caller-owned MongoDB resource가 package boundary다

기존 MongoDB Testcontainers fixture는 connection information을 반환하고 client,
collection, index, data를 caller에게 맡긴다. `leader/mongo`는 lifecycle이나 write
concern 결정을 숨기지 말고 이 형태를 보존해야 한다.

Prevention:

- caller-owned `*mongo.Collection`을 받는다.
- caller collection configuration을 조용히 바꾸지 말고 production write concern
  recommendation을 문서화한다.
- renewal과 cleanup goroutine은 elector lifecycle로 bound한다.
