# Issue #490 Mongo Strategic Leader Elector 교훈

MongoDB strategic election은 Redis public contract를 맞추되 key model을 그대로
복사해서는 안 된다. Redis는 candidate payload와 sorted index key가 필요하지만,
MongoDB는 각 candidate를 leased document 하나로 저장하고 `group_key,
lease_until`로 scan할 수 있다.

Lesson: TTL은 cleanup으로만 취급한다. MongoDB TTL monitor는 비동기라 correctness의
일부가 될 수 없으므로, `ListCandidates`는 strategy evaluation 전에 expired
candidate document를 명시적으로 prune하거나 filter해야 한다.

Lesson: result counter에는 live lease predicate로 guard되는 atomic backend update가
필요하다. read-modify-write candidate refresh는 stress 상황에서 concurrent outcome
increment를 잃는다.

Prevention: 다른 strategic backend를 추가할 때는 최소한 candidate
registration/listing, TTL deletion 전 stale cleanup, FIFO/scored/random strategy
compatibility, missing/expired result rejection, failure recording,
contention-safe result update를 증명한다.
