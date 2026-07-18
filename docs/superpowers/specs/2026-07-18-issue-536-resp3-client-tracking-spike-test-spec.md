# Issue #536 RESP3 CLIENT TRACKING Spike Test Specification

Date: 2026-07-18 KST
Issue: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)
Companion design: `2026-07-18-issue-536-resp3-client-tracking-spike-design.md`
Step 2-R evidence: `../reviews/2026-07-18-issue-536-resp3-client-tracking-spike-step-2r-spec-review.md`

## Test Boundary

The spike is an external-package test under `cache/redisnear`. Its cache,
serialization, key-building, and RESP3 behavior use only public bluetape-go and
go-redis APIs; repository-internal test cleanup may own only disposable
container termination. No production `.go` file, exported symbol, dependency,
background pump, or reconnect component is added.

| Layer | Location | Purpose |
|---|---|---|
| Handler unit | `cache/redisnear/resp3_tracking_spike_test.go` | Prove exact frame validation, allowlist mapping, redacted failures, bounded cleanup, non-blocking observation, overflow, and unregister semantics without Redis. |
| Redis integration | same file | Prove RESP3 negotiation, command-coupled push delivery, connection affinity, global invalidation, reconnect loss, and safe shutdown against a disposable Redis 7.4 container. |
| Regression | `cache/redisvalue`, `cache/redisnear` | Confirm the spike does not change existing TieredCache or Pub/Sub behavior. |
| Research ledger | `docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md` | Record exact environment, result matrix, Pub/Sub comparison, provider assumptions, and adopt/reject conclusion. |

## Test-Only Contracts

The test file defines a narrow invalidation seam:

```go
type localInvalidator interface {
	InvalidateLocal(context.Context, string) error
	ClearLocal(context.Context) error
}
```

`*redisvalue.TieredCache[string]` satisfies this interface. Direct handler tests
use a concurrency-safe fake that records logical keys and injected failures.

The handler accepts only:

```text
["invalidate", nil]
["invalidate", ["exact-physical-key-1", "exact-physical-key-2"]]
```

Validation is fixed:

- frame length exactly 2;
- first element exactly string `invalidate`;
- second element either nil or `[]interface{}`;
- no more than 64 keys;
- every key is a non-empty string of at most 2 KiB;
- aggregate key bytes at most 64 KiB;
- no duplicate physical keys;
- every key exists in a precomputed exact allowlist;
- physical keys map by exact map lookup, never prefix trimming.

Every failure returns only `errRESP3InvalidationRejected`. Observations contain a
typed low-cardinality reason and success bit, not raw frames, keys, namespaces,
provider errors, endpoints, or credentials. Observation uses a non-blocking send
and atomic overflow flag. Key-list cleanup uses a 1-second default context;
timeout tests inject 100 ms and observe completion with a 1-second watchdog.
If a per-key invalidation fails, later keys are not attempted and one
`ClearLocal` repair runs with a separate 250 ms context. The observation records
only a `repaired` boolean in addition to the redacted reason.

Callback admission uses a mutex-protected gate. `begin` checks the closed flag
and increments the in-flight `WaitGroup` while holding the same mutex that
`close` uses to reject later dispatch. Shutdown calls `close` before `wait`, so
no `WaitGroup.Add` can race a wait.

## Handler Unit Cases

### `TestRESP3TrackingSpikeHandlerRejectsUnsafePayloadsWithoutDisclosure`

Run these table rows directly against the handler:

| Row | Input | Expected reason |
|---|---|---|
| wrong arity 0/1/3 | missing or extra frame elements | `shape` |
| wrong notification | first element not string `invalidate` | `type` |
| wrong key collection | scalar/map second element | `type` |
| non-string key | nested integer/bool/nil | `type` |
| empty key | `""` | `key-size` |
| too many keys | 65 entries | `key-count` |
| oversized key | 2049 bytes | `key-size` |
| oversized aggregate | 33 allowlisted 2 KiB entries totaling more than 64 KiB | `aggregate-size` |
| duplicate key | same allowlisted physical key twice | `duplicate` |
| foreign key | sensitive-marker key absent from allowlist | `unknown-key` |

For every row:

- `errors.Is(err, errRESP3InvalidationRejected)` is true;
- the invalidator receives zero calls;
- exactly one failure observation is available;
- the overflow flag is false;
- `err.Error()` and the observation contain none of the sensitive markers.

### `TestRESP3TrackingSpikeHandlerProcessesBoundedMultiKeyPayload`

Submit two exact allowlisted physical keys. Assert both logical keys are
invalidated in payload order, no full clear occurs, one success observation has
`count=2`, and overflow is false.

### `TestRESP3TrackingSpikeHandlerReportsLocalCleanupFailure`

Run these subtests:

- three-key middle failure with successful repair: the first and second keys
  are attempted, the third is not, exactly one full clear occurs, and the
  failure observation has `repaired=true`;
- the same middle failure with injected full-clear failure: exactly one repair
  is attempted and the observation has `repaired=false`;
- global clear failure;
- expired 100 ms key-cleanup deadline, bounded by a 1-second watchdog.

Inject raw errors containing sensitive markers in every failure path.

Expected:

- no success observation;
- one redacted `local-cleanup`, `repair-failed`, or `cleanup-timeout`
  observation;
- returned error is only `errRESP3InvalidationRejected`;
- raw provider/local error text is absent;
- `go test -race` finds no observation/in-flight data race.

### `TestRESP3TrackingSpikeHandlerOverflowDoesNotBlock`

Use an unconsumed event channel of capacity one, invoke the handler twice, and
assert the second invocation returns within 1 second, sets `overflow=true`, and
returns the low-cardinality rejection sentinel. The test does not infer a
production latency budget from this watchdog.

### `TestRESP3TrackingSpikeUnregisterIsNotAQuiescenceBarrier`

Register an unprotected handler in a retained processor, obtain the registered
handler, and hold a direct callback after lookup on a latch. Unregister and
assert it returns before releasing the in-flight callback. Release and wait for
the callback with a 1-second bound. This proves unregister removes future lookups
but does not wait for callbacks already selected by the processor.

### `TestRESP3TrackingSpikeShutdownOrdersQuiescenceBeforeUnregister`

Hold one callback inside the invalidator, close the dispatch gate, and prove a
later direct dispatch is rejected without reaching the invalidator. Start the
gate wait and prove it is still blocked before releasing the held callback;
after release it must complete within 1 second. Then unregister, assert
`GetHandler("invalidate") == nil`, and close any owned connection/client through
1-second watchdog helpers. `Close()` timeouts fail the test but cannot cancel
the underlying context-free call. The focused handler/shutdown suite must pass
under `go test -race`.

## Redis Fixture

Each integration test starts a fresh Redis server with the already-declared
upstream `github.com/testcontainers/testcontainers-go/modules/redis` module and
the repository-pinned `redis:7.4-alpine` image. It registers the returned
container with `internal/testcleanup.Register`, obtains its `6379/tcp`
endpoint, and creates and owns:

1. tracking client: RESP3, retained processor, `PoolSize: 1`,
   `MaxRetries: -1`;
2. L2 client: separate ordinary client for `redisvalue.ValueCache`;
3. writer client: separate ordinary client for external `SET`;
4. admin client: constructed internally from the returned fixture address and
   never accepted from caller input.

The affinity test alone uses a tracking client with `PoolSize: 2` and obtains
two simultaneous sticky connections.

Before behavior assertions, call `container.Inspect(ctx)` and record and verify:

- non-empty configured image `inspect.Config.Image`, equal to
  `redis:7.4-alpine`;
- non-empty engine image identity `inspect.Image`;
- `INFO server` fields `redis_version`, `redis_build_id`, `os`, and
  `arch_bits` when present;
- `HELLO 3` map field `proto == 3`;
- tracking connection `CLIENT ID`;
- successful `CLIENT TRACKING ON NOLOOP`.

Test-admin methods are unexported fixture methods. They expose only `flushDB`
and `killID`, and they cannot be constructed with a custom client, dialer, or
endpoint.

## Redis Integration Cases

### `TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads`

Setup:

- namespace `issue-536-command-drain`;
- logical key `item`;
- exact physical key built by checking each public API result and returning the
  final `redis.Key.Value`:

  ```go
  builder, err := btredis.NewKeyBuilder("bluetape:cache:value")
  if err != nil {
      t.Fatal(err)
  }
  builder, err = builder.Structural(namespace)
  if err != nil {
      t.Fatal(err)
  }
  key, err := builder.LogicalKey("item")
  if err != nil {
      t.Fatal(err)
  }
  physical := key.Value
  ```
- L2 value `old` and a TieredCache L1 hit;
- tracked connection performs `GET` for the physical key.

Proof:

1. Writer sets the physical key to `new` and returns successfully.
2. An immediate non-blocking event read observes nothing.
3. TieredCache returns stale L1 value `old`; this is the expected negative
   capability evidence.
4. Tracked connection runs `PING` with a 2-second context.
5. Exactly one successful key-invalidation observation arrives and overflow is
   false.
6. TieredCache returns `new` from L2 after `InvalidateLocal`.

The test publishes no latency claim. It proves only that an ordinary command on
the tracked socket drains the pending push.

### `TestRESP3TrackingSpikeRequiresReadAndTrackingOnSameConnection`

Setup two sticky connections A/B with distinct `CLIENT ID` values.

Proof:

1. Enable tracking on A.
2. Read the physical key on B, mutate through writer, and drain A then B.
3. Observe no invalidation for B's untracked read.
4. Read the key on A, mutate again, drain A.
5. Observe exactly one invalidation.

No assertion depends on repeated `Client.Do` pool scheduling.

### `TestRESP3TrackingSpikeMapsGlobalInvalidationToClearLocal`

Track and cache logical keys `first` and `second`. Run fixture-owned `FLUSHDB`,
drain the tracked connection, and assert:

- exactly one global observation;
- `ClearLocal` succeeds once;
- overflow is false;
- both subsequent TieredCache reads return `cache.ErrCacheMiss`;
- handler performs no Redis mutation.

### `TestRESP3TrackingSpikeReconnectRequiresReenableAndLocalFlush`

Proof sequence:

1. Track/read `item`, cache `old`, and record client ID A.
2. Admin `CLIENT KILL ID A` returns exactly 1.
3. Writer changes the physical key to `new` during the disconnected window.
4. Retry-disabled tracked `PING` returns a transport error within 2 seconds.
5. No invalidation is replayed and direct L1 inspection still sees `old`.
6. Test harness blocks tracked-L1 use, calls `ClearLocal`, and closes the dead
   sticky connection with a watchdog.
7. Obtain replacement connection B and assert B != A.
8. `HELLO 3` reports protocol 3; `CLIENT TRACKINGINFO` reports flag `off`.
9. Enable tracking, read/cache `new`, writer changes to `newer`, and `PING`
   drains exactly one invalidation.
10. TieredCache returns `newer` from L2.

This case proves the safe order; it does not implement a production reconnect
state machine.

## ACL And Provider Evidence

The unauthenticated Redis container proves protocol/lifecycle behavior only.
The research ledger must separately record:

| Identity | Required commands | Explicitly unnecessary or denied |
|---|---|---|
| tracked runtime | `HELLO`, `CLIENT TRACKING`, `CLIENT TRACKINGINFO`, `CLIENT ID`, `GET`, `PING` | `FLUSHDB`, `FLUSHALL`, `CLIENT KILL` |
| Tiered L2 runtime | existing redisvalue bounded read/write commands | admin commands |
| writer | ordinary mutation commands | admin commands |
| disposable-test admin | `FLUSHDB`, `CLIENT KILL ID` | no production identity equivalent |

AUTH, TLS, certificate validation, Sentinel, Cluster, managed provider, and
proxy behavior remain documented or unknown unless separately proven. Redis
Cloud/Software `REDIRECT` is recorded as unsupported, not tested.

## Research Result Matrix

The note contains one row per proof:

| Proof | Expected result before execution | Decision effect |
|---|---|---|
| RESP3 negotiation | PASS | Push API prerequisite only |
| exact key payload | PASS after explicit drain | Parsing technically available |
| idle L1 | stale before drain | Reject autonomous invalidation claim |
| affinity | only A read is invalidated | Reject ordinary pooled-client affinity claim |
| global flush | null payload clears L1 | Global repair technically available after drain |
| connection loss | missed invalidation not replayed | Require block and full L1 clear |
| replacement connection | tracking initially off | Require explicit re-enable |
| unregister | not quiescent | Require separate in-flight ownership |
| ordered shutdown | bounded after quiescence | Test-owned lifecycle proof only |

If observed behavior differs, record the actual result and source-pinned
go-redis/Redis versions. Do not force the expected rejection conclusion by
weakening assertions.

## Verification Commands

```bash
go test -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpikeHandler'
go test -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpike(Unregister|Shutdown)'
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike(Delivers|Requires|Maps|Reconnect)'
go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads|RequiresReadAndTrackingOnSameConnection|MapsGlobalInvalidationToClearLocal|ReconnectRequiresReenableAndLocalFlush|UnregisterIsNotAQuiescenceBarrier|ShutdownOrdersQuiescenceBeforeUnregister)$'
go test -race -count=1 ./cache/redisnear
go test -count=1 ./cache/redisvalue ./cache/redisnear
make fmt-check
make tidy-check
make vet
make lint
make test
```

## Explicit Non-Applicable Checks

- No benchmark table, chart, latency, throughput, CPU, memory, or heartbeat
  interval claim. Issue #560 owns measured comparison.
- No production goroutine leak test. The spike adds no production goroutine.
- No managed-provider integration claim. The disposable OSS container is the
  only live provider proof.
- No public API compatibility test beyond compilation because the spike adds no
  public symbol.
- No root/package README or CHANGELOG change unless implementation unexpectedly
  crosses a Type A escalation trigger, in which case work stops first.
