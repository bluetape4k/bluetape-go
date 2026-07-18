# Issue #536 RESP3 CLIENT TRACKING Spike Step 2-R Review

Date: 2026-07-18 KST
Issue: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)
Reviewed spec: `docs/superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-design.md`
Original converged commit: `38364100af92d6da616ff89101109fc768e639a4`
Final reviewed commit: `3d7567b13ebc3a427771734e42aa9b980a7d8388`
Baseline: `f4acaab1676ca4a989051a28f60f37ab147d87f9`

## Integrated Verdict

`PASS — P0=0 P1=0 P2=0`

Six independent lanes reviewed the same final spec commit after a delta review
from the original convergence point. The main session integrated their
findings, checked the repaired design against the current
`cache/redisnear`, `cache/redisvalue`, `redis.KeyBuilder`, Testcontainers, and
go-redis/v9 v9.20.0 surfaces, and found no remaining blocker.

The `P1-1` through `P1-8` entries retained in the design are production RESP3
capability blockers that the spike is intended to prove or reject. They are not
unresolved defects in the spike design.

## Final Exact-Commit Results

| Lane | P0 | P1 | P2 | Verdict |
|---|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | PASS |
| Stability/concurrency | 0 | 0 | 0 | PASS |
| Security | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 0 | PASS |
| User/caller | 0 | 0 | 0 | PASS |
| Main-session integration | 0 | 0 | 0 | PASS |

## Findings Resolved During Review

### Performance

- Separated tracking, L2, writer, and admin client ownership so a sticky
  `PoolSize: 1` connection cannot starve TieredCache commands.
- Changed the conclusion from “heartbeat proven” to one explicit
  command-coupled drain proven. Cadence, latency, QPS, socket/memory cost, and
  provider comparison remain in #560 or a separately approved Type A design.
- Bounded synchronous handler work with short TieredCache cleanup limits,
  independent handler context, non-blocking observation, and overflow failure.
- Replaced timing-based absence assertions with a completed external `SET`
  barrier and immediate non-blocking check.
- Fixed the exact serial repetition command, count, timeout, and exclusion of
  container startup from performance evidence.

### Stability/concurrency

- Defined unregister as non-quiescent and added explicit proof that it does not
  wait for an already selected callback.
- Added a synchronized callback gate that closes admission before waiting,
  rejects later callbacks with one redacted observation, waits for held work,
  unregisters, then closes through idempotent resource owners.
- Disabled go-redis command retries for loss proof, required exact kill count,
  first transport failure, different replacement client ID, tracking-off
  evidence, L1 clear, and resumed delivery.
- Gave handler cleanup an independent deadline and synchronized success/failure
  observation so an expired drain context cannot silently leave stale L1 state.
- Required distinct client IDs for affinity and replaced a goroutine-leak claim
  with observable bounded operations.
- Described context-free `Conn.Close` and `Client.Close` with watchdog semantics
  that observe but cannot cancel a stuck close.
- Registered client/connection cleanup after container cleanup so LIFO closes
  Redis resources first, with per-resource `sync.Once` for explicit-close
  re-entry.

### Security

- Required a low-cardinality handler sentinel because go-redis logs handler
  errors through its global logger.
- Added direct negative tests for malformed, foreign, duplicate, oversized,
  and failing-local payload paths with sensitive-marker non-disclosure checks.
- Bounded frame arity, key count, per-key bytes, aggregate bytes, and allowed
  physical keys; reverse prefix trimming is forbidden.
- Lowered the aggregate limit to 64 KiB so 33 valid 2 KiB allowlisted keys can
  independently exercise it before the 64-key and per-key limits fire.
- Separated runtime, L2, writer, and disposable-test admin identities and
  documented production ACL denial for destructive commands.
- Constrained `FLUSHDB` and `CLIENT KILL ID` to an admin client constructed
  internally from the fresh Testcontainers endpoint with no caller-supplied
  client, options, dialer, or endpoint.

### Operator/Ops

- Required `INFO server` version/build data plus configured image tag or digest
  in the result ledger.
- Replaced a duplicated image constant with actual container inspection of the
  configured image and engine image identity.
- Classified provider and proxy claims as proved, documented, unsupported, or
  unknown instead of implying universal compatibility.
- Kept AUTH/TLS/certificate ownership and ACL expectations explicit while
  avoiding a false claim that the unauthenticated container proves them.

### Developer/API

- Confirmed the spike compiles against public-only seams:
  `TieredCache.InvalidateLocal`, `TieredCache.ClearLocal`,
  `redis.KeyBuilder`, go-redis sticky connections, handler registration, and a
  retained notification processor.
- Preserved the Type B boundary: no production file, exported tracking API,
  dependency change, connection owner, pump, reconnect subsystem, or exported
  physical-key mapper.

### User/caller

- Kept RESP3 frame delivery separate from a coherent near-cache claim.
- Separated null-payload handler clearing from transport-failure detection and
  harness-owned clear-before-replacement behavior in the architecture diagram.
- Kept `redisnear.NewPubSub` as the production strategy unless a future Type A
  implementation closes autonomous drain, affinity, disconnect, reconnect,
  failure propagation, and provider-topology gaps.
- Made the final result actionable even when the correct issue outcome is
  rejection of a production `NewTracking` API.

## Main-Session Integration Check

- The approved issue remains a Type B test/research spike.
- The test callback can only invalidate or clear L1; it cannot mutate Redis L2.
- The four client owners remove pool starvation and administrative authority
  ambiguity.
- The negative command-coupled proof is deterministic and is not a benchmark.
- Reconnect and shutdown claims match the public go-redis API rather than
  assuming callbacks or context-aware close APIs that do not exist.
- Handler failure paths cannot disclose raw Redis payloads through the
  go-redis logger.
- Every issue acceptance criterion maps to an executable test or result-ledger
  entry.
- No implementation mutation is authorized by this review artifact itself;
  Step 3 plan review remains the next gate.

## Verification

```bash
git diff --check f4acaab1676ca4a989051a28f60f37ab147d87f9..3d7567b13ebc3a427771734e42aa9b980a7d8388
```

Result: PASS.

Runtime Redis behavior was intentionally not executed during Step 2-R. The
approved plan and test spec own RED/GREEN execution and evidence capture.
