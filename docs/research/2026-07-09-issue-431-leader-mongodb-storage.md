# Issue #431 MongoDB Leader Storage Evaluation

Issue: [#431](https://github.com/bluetape4k/bluetape-go/issues/431)  
Milestone: Backlog  
Date: 2026-07-09  
Decision: **implement `leader/mongo` in a single-elector first slice**

## Decision

Add a future MongoDB-backed `leader.Elector` package only after this research
branch lands. The first implementation issue should support single-leader
campaign, renewal, observation, and owner-token release for `leader.Elector`.

Do not ship `GroupElector` or `StrategicElector` in the first MongoDB slice.
Both are valid follow-ups, but they need different document shapes and
contention proofs:

- `GroupElector` needs a slot model that preserves exact `MaxLeaders` under
  concurrent acquisition.
- `StrategicElector` needs a candidate registry, pruning policy, deterministic
  read model, and strategy-specific failover tests.

## Current Repo Evidence

| Evidence | Result |
|---|---|
| `leader/elector.go` | The shared `Elector` contract already separates `Campaign`, `Resign`, `IsLeader`, and `Leader`. |
| `leader/group.go` | `GroupElector` adds `MaxLeaders`, active counts, and slot availability, so it is not just a single-document variant. |
| `leader/options.go` | Lease, renew interval, group, member ID, and key prefix validation are already backend-neutral. |
| `leader/README.md` / `.ko.md` | Backend renewal failure must make `IsLeader` false. |
| `leader/redis/README.md` | Redis already provides single, group, and strategic implementations, but each uses a different storage pattern. |
| `testcontainers/mongodb` | #430 provides a shared MongoDB integration-test fixture while keeping client, database, collection, indexes, and test data caller-owned. |
| `docs/lessons/2026-07-07-mongodb-testcontainer-fixture.md` | MongoDB helpers should not hide client lifecycle or caller-owned storage decisions. |

## MongoDB Semantics That Matter

| Semantics | Design implication |
|---|---|
| `findOneAndUpdate` atomically filters and updates one document | Use one lease document per normalized leader key for single-elector acquisition and renewal. |
| Single-document writes are atomic | Store the active owner token and `lease_until` in the same document; do not split ownership across collections for the first slice. |
| TTL indexes remove expired documents asynchronously | TTL is cleanup only. Lease validity must come from query predicates such as `lease_until <= now` or `lease_until > now`. |
| `$currentDate` can set server-side timestamps | Prefer server-side timestamps for `updated_at`; evaluate aggregation-pipeline updates for server-side `lease_until` calculation before accepting client clock skew. |
| Write concern is caller configuration | Document `majority` write concern as the production recommendation; do not silently mutate a caller-provided collection. |

Sources:

- MongoDB TTL indexes: <https://www.mongodb.com/docs/manual/core/index-ttl/>
- MongoDB single-document atomicity: <https://www.mongodb.com/docs/manual/core/write-operations-atomicity/>
- MongoDB `$currentDate`: <https://www.mongodb.com/docs/manual/reference/operator/update/currentDate/>
- MongoDB `findOneAndUpdate`: <https://www.mongodb.com/docs/manual/reference/method/db.collection.findOneAndUpdate/>
- MongoDB Go driver `FindOneAndUpdate`: <https://www.mongodb.com/docs/drivers/go/current/usage-examples/updateOne/#find-and-update>

## Proposed Single-Elector Shape

| Field | Purpose |
|---|---|
| `_id` or `key` | Unique normalized leader key, such as `<keyPrefix>:<group>`. |
| `group` | Human-readable group name for diagnostics. |
| `member_id` | Caller member ID from `leader.Options`. |
| `token` | Opaque owner token, preferably `memberID:random`, returned by `Leader`. |
| `lease_until` | Authoritative lease-expiry instant used by acquire/read predicates. |
| `created_at` / `updated_at` | Diagnostics and cleanup support. |

Indexes for the first slice:

- unique `_id` or unique `{key: 1}`;
- optional TTL index on `lease_until` with `expireAfterSeconds: 0` for cleanup;
- no group/strategy index until group or strategic elector work begins.

## Operation Design

| Operation | Required behavior |
|---|---|
| `Campaign(ctx)` | Loop until context cancellation or successful ownership. Try an atomic conditional update for expired ownership; handle duplicate-key races from upsert as a lost acquisition and retry after the existing renewal interval/backoff. |
| `Renew` | Update only when the stored token matches this elector's token. A zero-match renewal means leadership was lost; stop the renewal loop and make `IsLeader` false. |
| `Resign(ctx)` | Delete or clear only when token matches. If another owner already replaced the document, return success after clearing local leadership because `Resign` is idempotent for non-leaders. |
| `Leader(ctx)` | Read the document only when `lease_until > now` and return the stored token; expired documents are treated as no leader even if TTL cleanup has not run. |
| `IsLeader()` | Return local state only. It becomes true after successful acquisition and false after resign, failed renewal, context-driven shutdown, or observed owner loss. |

## Time And Clock Policy

The implementation should prefer MongoDB server time for write timestamps and,
if practical with the Go driver, server-side calculation of `lease_until`.
If the first slice computes `lease_until` in Go, the package must document that
callers need bounded clock skew between contenders and should configure lease
durations larger than expected skew plus operation latency.

TTL monitor timing must not participate in correctness. It may delete old
lease documents later than their expiry; acquisition and observation remain
correct only because every active query compares `lease_until`.

## Cancellation And Lifecycle

- Caller owns the MongoDB client, database, collection, indexes, and write
  concern configuration.
- `Campaign` and `Resign` honor caller contexts.
- The renewal loop must stop on `Resign`, lost ownership, context cancellation,
  or backend errors that prevent proving ownership.
- Cleanup contexts may use bounded `context.WithoutCancel` only for local
  teardown after a request context is canceled; no hidden global goroutines or
  clients.

## Follow-Up Scope

Follow-up implementation issue:

- [#485](https://github.com/bluetape4k/bluetape-go/issues/485)
  `feat: Add MongoDB single leader elector backend`

Scope for #485:

- `leader/mongo` single `leader.Elector`;
- options that accept a caller-owned `*mongo.Collection`;
- owner-token acquire, renew, release, and read predicates;
- TTL cleanup index documentation or helper;
- integration tests with `testcontainers/mongodb`;
- contention/race tests proving only one local leader at a time.

Defer these to later issues:

- MongoDB `GroupElector`;
- MongoDB `StrategicElector`;
- transaction-backed or multi-document designs;
- JVM wire/document compatibility;
- a generic distributed-lock package.

## Verification Plan For Implementation

Future implementation PRs must include:

- `go test -count=1 ./leader ./leader/mongo`
- `go test -race -count=1 ./leader ./leader/mongo`
- `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb`
- contention tests with multiple contenders and short leases;
- failed-renewal tests that prove `IsLeader` flips false;
- expired-document tests that prove TTL cleanup is not required for takeover.

## Outcome

#431 should close after this research note, README pointer, review artifact, and
durable wiki preservation land. Follow-up issue #485 implements only the
single-elector MongoDB backend and uses this document as the acceptance boundary.
