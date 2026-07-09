# MongoDB Single Leader Elector Lessons

## L1: Keep TTL as cleanup even in tests

The MongoDB elector can take over expired documents without waiting for the TTL
monitor. Tests should advance or write `lease_until` directly and prove normal
query predicates handle expiry.

Prevention:

- Test expired takeover with an existing document still present.
- Keep `EnsureIndexes` optional and document it as cleanup support only.
- Do not add sleeps that wait for MongoDB TTL deletion.

## L2: Campaign semantics differ from Redis by design

Redis `Campaign` returns `ErrNotLeader` immediately when another member owns the
key. Issue #485 requires MongoDB `Campaign(ctx)` to wait until acquisition or
context cancellation.

Prevention:

- Document MongoDB `Campaign(ctx)` as wait-until-acquired.
- Test cancellation explicitly so callers can bound waits.
- Keep duplicate local campaign protection separate from remote ownership waits.

## L3: Testcontainers contention tests need one container per suite

Starting a MongoDB container per subtest would make the package slow and noisy.
One container with per-subtest collections keeps tests isolated while preserving
serial Testcontainers execution.

Prevention:

- Create one MongoDB container in the integration suite.
- Use unique collection names for subtests.
- Drop collections with bounded cleanup contexts.
