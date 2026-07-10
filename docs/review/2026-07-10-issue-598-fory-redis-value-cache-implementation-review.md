# Issue #598 Fory Redis Value Cache Implementation Review

Date: 2026-07-10 KST
Scope: `origin/develop...HEAD`
Baseline: `origin/develop` at `24ab73aba12168d29dc8ef8a12ba04d48eb3edfe`

## Evidence

- `cache/redisfory` provides opt-in Go-native fast and compatible Fory value
  caches over a caller-owned `redis.Cmdable`.
- Physical keys include the caller namespace and schema generation. Values use
  the fail-closed `BTFV v1` binary envelope with exact magic, profile, schema,
  declared-length, and total-length checks before Fory decode.
- Fory runtime construction, registration, panic recovery, root validation,
  synchronization, and resource limits are shared with `cache/rediscoord/fory`
  through `cache/internal/forynative`; the two public packages retain independent
  `BTFV` and `BTFY` storage contracts.
- `Set` rechecks caller cancellation after serialization and before Redis
  dispatch. A Redis 7.4 integration test proves that this path leaves no key.
- `Get` uses `GETRANGE` to materialize at most the configured payload, the
  14-byte header, and one overflow-detection byte. `EXISTS` distinguishes a
  missing key from an existing empty corrupt value.
- A Redis 7.4 ACL integration test proves that the documented minimum command
  set, `GETRANGE`, `EXISTS`, `SET`, and `DEL`, supports Set/Get/miss/Delete.
- Redis and Fory provider details are replaced by sanitized package causes.
  Caller cancellation and deadlines remain inspectable with `errors.Is`, and
  `*btredis.OpError.KeyID()` supplies the redacted Redis key identifier.
- English and Korean package READMEs document bounded contexts, finite Redis
  timeouts, caller-owned hooks, rollout/rollback, cluster cleanup, the exact
  envelope, and the no-cross-language contract.
- The architecture diagram is retained as SVG and PNG. XML parsing, 3000x1640
  rendering, endpoint, sequence-style, geometry, connector, crossing, and
  visual inspection checks passed.

## Verification

| Command or check | Result |
| --- | --- |
| `go test -p 1 -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory` | PASS |
| `go test -race -p 1 -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory` | PASS |
| Redis 7.4 cancellation, bounded oversized read, empty corruption, ACL lifecycle, TTL, schema isolation, redaction, and 16x100 concurrent round trips | PASS |
| `go vet ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory` | PASS |
| Scoped `golangci-lint` | PASS, `0 issues` |
| `gopls check` on changed Go files | PASS |
| `make fmt-check` and `make tidy-check` | PASS |
| `git diff --check origin/develop...HEAD` | PASS |
| `make ci` | INCOMPLETE: unrelated `ratelimit/redis` Redis readiness tests timed out; the changed packages passed and `go test -p 1 -count=1 ./ratelimit/redis` passed immediately afterward. Remote CI remains required. |

## Seven-Tier Review

| Lane | Initial findings | Corrections and final decision |
| --- | --- | --- |
| Performance | P2 constructor-invariant reflection; P3 copy and key-redaction hot-path questions | Root shape is cached, magic comparison avoids string conversion, and #599 owns copy/contention/key-cost measurement. P0=0 P1=0. |
| Stability | P2 missing Redis-backed post-serialization cancellation proof | Added no-write proof, bounded/empty/miss coverage, readiness polling, and serial normal/race evidence. P0=0 P1=0. |
| Security | No findings | Bounds, trusted-input boundary, error redaction, and schema isolation remain intact. P0=0 P1=0. |
| Operator/Ops | P2/P3 telemetry and timeout guidance; later P1 full Redis read and P1 stale ACL list | Added bounded reads, Redis oversized-value proof, caller hook/timeouts guidance, exact ACL docs, and minimum-ACL lifecycle proof. P0=0 P1=0. |
| Developer/API | P3 direct nil-context diagnostics, duplicate `format` state, and dead helper | Removed diagnostics/state/helper and retained the shared panic boundary. P0=0 P1=0. |
| User/Caller | P3 `Key.RedactedID()` wording; P2 wrong `redis.OpError` name | Corrected field wording and documented `*btredis.OpError.KeyID()` in both READMEs. P0=0 P1=0. |
| Main integration | Reviewed complete diff, tests, docs, diagram, and #597/#599 boundaries | Scope remains direct Go-only Redis values; no loading fallback, cross-language claim, or benchmark claim was introduced. P0=0 P1=0. |

## Deferred Work

- Issue #599 owns comparative performance work. It must retain exact commands,
  raw output, environment and revision metadata, a result table, a Chart, and
  written analysis covering Fory profiles, alternatives, mutex/pool contention,
  safe copies, and key-redaction cost.
- The per-operation `btredis.Key` redacted-ID calculation remains unchanged
  until #599 shows that it is material under realistic Redis-backed workloads.
- Remote CI must be green before merge because the local full-repository gate
  encountered an unrelated transient Testcontainers readiness failure.

P0=0 P1=0
