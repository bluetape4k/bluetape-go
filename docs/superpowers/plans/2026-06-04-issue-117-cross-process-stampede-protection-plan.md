# Issue #117 Cross-Process Stampede Protection Plan

Issue: #117
Milestone: 0.3.0
Spec: `docs/superpowers/specs/2026-06-04-issue-117-cross-process-stampede-protection-spec.md`
Research: `docs/research/2026-06-04-issue-117-cross-process-stampede-protection.md`

## Implementation Strategy

Add `cache/rediscoord` as a narrow opt-in package. The package wraps an existing
`cache.LoadingCache[string,V]`, uses Redis owner-token locks for load ownership,
and publishes a short-lived result envelope so waiters can populate their local
cache without invoking their user loader.

`cache/redisnear` remains unchanged except for README examples that show it can
be wrapped when cross-process collapse is needed.

## Tasks

| Task | Scope | Work | Validation |
|---|---|---|---|
| T1 Package scaffold | `cache/rediscoord/doc.go`, API files | Add package docs, `Codec`, `JSONCodec`, `Options`, `StampedeCache`, defaults, validation, interface assertion. | `go test -count=1 ./cache/rediscoord -run 'Test.*Options|Test.*JSONCodec'` |
| T2 Result envelope | `cache/rediscoord/result.go` | Encode/decode versioned token-bound result envelope with codec payload. Reject malformed, wrong-version, and wrong-token results. | envelope unit tests |
| T3 Winner path | `cache/rediscoord/stampede_cache.go` | On miss, acquire #24 Redis lock, run wrapped `GetOrLoad` with a loader that marshals before returning, publish result, unlock through background cleanup context. | winner unit/integration tests |
| T4 Waiter path | `cache/rediscoord/stampede_cache.go` | Observe current lock owner token, poll matching result envelope, fill wrapped cache through local decoded loader, retry after lock expiry, return context errors. | cancellation and collapse tests |
| T5 NearCache integration | `cache/rediscoord/*_test.go` | Use two `redisnear.NewPubSub` instances and two wrappers under one namespace after peer invalidation. | Testcontainers collapse test |
| T6 Stress/cancellation | `cache/rediscoord/*_test.go` | Use `GoroutineStressTester` for cold burst collapse and `AsyncJobTester` for waiter cancellation. | stress and async tests |
| T7 Lease expiry | `cache/rediscoord/*_test.go` | Prove abandoned/over-TTL owner does not deadlock peer progress; document possible over-TTL overlap. | lease-expiry test |
| T8 Docs | `README.md`, `README.ko.md`, `CHANGELOG.md`, example | Add package row, cross-process stampede semantics, failure boundary, and benchmark note. | docs diff and example compile |
| T9 Lessons/verifier | `docs/lessons`, `docs/superpowers/reviews` | Record implementation lessons, verifier matrix, and 7-Tier code review. | review gate `P0 = 0`, `P1 = 0` |

## Detailed Algorithm

### Winner

1. Check the wrapped cache with `Get`.
2. Acquire a Redis lock with key:

   ```text
   bluetape:cache:coord:<namespace>:lock:<key>
   ```

3. Call wrapped `GetOrLoad` with a wrapper loader:
   - calls the user loader;
   - marshals the user value before returning it;
   - returns marshal errors so an unshareable value is not cached by the
     wrapped cache.
4. If wrapped `GetOrLoad` returns a value from an existing in-process fill,
   marshal that value after the call.
5. Store the result envelope at:

   ```text
   bluetape:cache:coord:<namespace>:result:<key>
   ```

   with `ResultTTL`.
6. Unlock using a short background cleanup context.

### Waiter

1. If lock acquisition returns `redislock.ErrNotAcquired`, read the current
   owner token from the lock key.
2. Poll the result key at `PollInterval`.
3. Accept only an envelope whose token matches the observed owner token.
4. Fill the wrapped cache with `GetOrLoad` and a local loader returning the
   decoded value.
5. If the lock disappears without a matching result, retry acquisition.
6. If caller context expires, return the context error.

## Error Handling Decisions

- Nil loader returns a normal validation error before Redis commands.
- Redis lock acquisition errors return immediately.
- Loader errors return immediately and do not write a result envelope.
- Marshal errors return immediately and prevent caching on the winner path.
- Result `SET` failure after a local load returns the Redis error. The local
  cache may already hold the value; the error is still surfaced because
  cross-process collapse failed for this call.
- Unlock failure after successful result publication returns a joined error with
  the loaded value unavailable through the API. The lock TTL remains the recovery
  guard. This avoids silently hiding a coordination cleanup failure during first
  implementation.
- Waiter unmarshal failures on a matching owner result return an error because
  the current shared result is corrupt or incompatible.

## Test Matrix

| Behavior | Unit | Redis/Testcontainers | Stress | Race |
|---|---:|---:|---:|---:|
| Option validation/defaults | Yes | No | No | No |
| JSON codec round-trip | Yes | No | No | No |
| Envelope token/version checks | Yes | No | No | No |
| Winner loads and publishes result | No | Yes | No | Yes |
| Waiter reuses winner result | No | Yes | Yes | Yes |
| NearCache post-invalidation collapse | No | Yes | Yes | Yes |
| Waiter cancellation | No | Yes | Yes via `AsyncJobTester` | Yes |
| Lease expiry recovery | No | Yes | No | Yes |
| Underlying cache interface conformance | Yes | No | No | No |

## Documentation Notes

README wording must distinguish:

- local same-process duplicate suppression: `cache.Memory`;
- peer invalidation: `cache/redisnear`;
- cross-process load-result collapse: `cache/rediscoord`;
- benchmark execution: `make bench-cache` / #107, not `make ci`.

Korean README must preserve the same semantics in concise Korean.

## Validation Sequence

1. `go test -count=1 ./cache/rediscoord`
2. `go test -race -count=1 ./cache/rediscoord`
3. `go test -count=1 ./cache ./cache/redisnear ./cache/rediscoord ./lock/redis`
4. `go test -count=1 ./...`
5. `make ci`
6. `git diff --check`
7. `gno update`
8. Representative `gno search "Issue #117 Cross-Process Stampede Protection" -c bluetape4k-docs -n 10 --files`

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| Result envelope consumed from previous load window | Require token match with observed current owner token. |
| Waiter spins under Redis race | Poll with `PollInterval` and caller context; retry only after lock disappears. |
| Loader runs longer than lease | Document lease boundary; lease expiry test proves no deadlock, not perfect mutual exclusion. |
| Codec turns API into hidden Redis cache | Name package and docs as coordination/result transport; keep `ResultTTL` short. |
| NearCache `Set` publishes invalidation when filling waiter local cache | Fill through wrapped `GetOrLoad` with decoded local loader, not `Set`. |
| Cleanup uses canceled caller context | Use short background cleanup context for unlock. |

## Definition Of Done

- All T1-T9 tasks complete.
- Required validation sequence passes or any blocker is documented with exact
  command output.
- Step 6-R code review reports `P0 = 0` and `P1 = 0`.
- PR is opened with issue link, validation evidence, and explicit request for
  approval before merge.
