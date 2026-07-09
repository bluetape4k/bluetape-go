# MongoDB Leader Storage Research Lessons

## L1: TTL cleanup is not lease correctness

MongoDB TTL indexes delete documents asynchronously, so they are useful only for
garbage collection. A leader backend must decide acquisition and observation
from `lease_until` predicates in normal reads and writes.

Prevention:

- Treat `lease_until <= now` as the takeover condition.
- Treat `lease_until > now` as the only active-leader read condition.
- Keep TTL index creation optional or documented as cleanup support, not as the
  coordination mechanism.

## L2: Ship one MongoDB elector shape before group or strategy variants

Redis supports single, group, and strategic electors, but MongoDB should not
inherit those shapes mechanically. Single-elector ownership fits one lease
document. Group and strategic electors need different concurrency proofs.

Prevention:

- First MongoDB implementation issue covers only `leader.Elector`.
- Defer `GroupElector` until an exact `MaxLeaders` slot design is written.
- Defer `StrategicElector` until candidate registry and pruning semantics are
  designed independently.

## L3: Caller-owned MongoDB resources remain the package boundary

The existing MongoDB Testcontainers fixture returns connection information and
leaves clients, collections, indexes, and data to callers. `leader/mongo` should
preserve that shape instead of hiding lifecycle or write concern decisions.

Prevention:

- Accept a caller-owned `*mongo.Collection`.
- Document production write concern recommendations instead of mutating caller
  collection configuration silently.
- Keep renewal and cleanup goroutines bounded by elector lifecycle.
