# Issue #411 Redis HyperLogLog Lesson

## Lesson

Redis HyperLogLog is safest as a narrow core-Redis wrapper, not as a broad
probabilistic command facade. Keeping HLL on `PFADD`, `PFCOUNT`, and `PFMERGE`
lets the existing Redis Testcontainers fixture prove behavior without requiring
RedisBloom module assumptions.

## Evidence

- #410 selected HLL first and deferred Cuckoo until `CF*` runtime support is
  explicit.
- The implementation stores SHA-256 hex digests of `probabilistic.Hasher`
  output, avoiding raw caller values in Redis command payloads.
- `GoroutineStressTester` and `AsyncJobTester` cover concurrent HLL calls and
  cancellation behavior.
- The first full test/race gate was accidentally started in parallel; the
  touched Testcontainers package was rerun sequentially to preserve reliable
  evidence.

## Future Rule

For Redis probabilistic additions, keep each structure as its own contract:
Bloom for membership with false positives, HLL for approximate cardinality, and
Cuckoo only after RedisBloom `CF*` fixture/runtime assumptions are documented.
