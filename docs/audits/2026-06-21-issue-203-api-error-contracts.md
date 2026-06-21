# Issue #203 Public API and Error Contract Audit

Issue: [#203](https://github.com/bluetape4k/bluetape-go/issues/203)  
Parent: [#199](https://github.com/bluetape4k/bluetape-go/issues/199)  
Milestone: `0.6.2`  
Date: 2026-06-21

## Verdict

P0 = 0, P1 = 0, P2 = 5, P3 = 2

No public API or error-contract blocker needs to stop the `0.6.3` through
`0.6.6` corrective series. The high-risk packages already expose typed or
sentinel errors where callers need `errors.Is` / `errors.As`; lower-level helper
packages mostly return plain validation errors where callers do not yet have a
stable semantic matching need.

Breaking API proposals are not bundled into this issue. They should be filed
under the receiving corrective epics only when the next implementation slice
proves the migration value.

## Evidence

- `go list ./...` enumerated 36 packages, including 35 public packages and one
  internal cleanup helper package.
- `go doc -short` was used for public exported-surface inventory.
- `rg` checked sentinel errors, typed errors, wrapping, and `errors.Is` /
  `errors.As` tests.
- CodeGraph index was available with 300 Go files, 4,295 symbols, and 10,286
  edges for structural lookup.

## Package Contract Matrix

| Package | Exported surface | Error contract | Zero/nil/context contract | Verdict |
|---|---:|---|---|---|
| `batch` | 25 | `ErrUnsafeWriterSkipCheckpoint`; policy predicates preserve caller errors. | Constructors reject nil steps and invalid policies; checkpoint safety documented. | P2: consider typed constructor errors only if callers need matching. |
| `cache` | 6 | `ErrCacheMiss` is stable and tested with `errors.Is`. | Zero TTL semantics and cancellation behavior are tested. | OK |
| `cache/rediscoord` | 5 | Wraps Redis/cache/lock causes; no exported sentinel. | Context and lease cleanup behavior covered in tests. | P2: revisit if shared stampede errors become operator-facing. |
| `cache/redisnear` | 6 | `ErrClosed` is stable and tested with `errors.Is`; cache miss comes from `cache`. | Closed/cancelled behavior covered. | OK |
| `codec` | 20 | Decode errors are plain format errors. | Stateless functions, no zero-value object. | P3: optional future sentinel for invalid alphabet/input if callers need matching. |
| `collections` | 9 | Mapper/predicate errors are propagated and tested with `errors.Is`; validation errors are plain. | Nil callback inputs are rejected; nil slice behavior is ordinary Go. | P2: future #204 should decide whether validation sentinels add value. |
| `compression` | 13 | Compressor errors wrap codec/writer/reader failures with `%w`. | Nil reader/writer rejected; compressor values are immutable factories. | OK |
| `concurrency` | 9 | `PanicError` supports `errors.As`; task errors and context errors propagate. | Nil tasks and worker inputs rejected; context cancellation covered. | OK |
| `core` | 21 | Validation helpers return plain errors. | Zero/default helpers are value-only and deterministic. | P2: #204 should decide whether shared validation sentinels are worth exporting. |
| `id` | 46 | Mature sentinel + typed errors: options, parse, entropy, clock rollback, sequence exhaustion. | Injected clocks/readers validated; concurrency and monotonic behavior tested. | OK |
| `jwt` | 57 | Mature sentinel + typed errors for options, token, key, expiry, and not-yet-valid cases. | Nil/zero provider and context behavior tested. | OK |
| `jwt/redis` | 3 | Alias facade inherits `jwt.RedisRepository` contracts. | Narrow constructor surface. | OK |
| `leader` | 20 | `ErrAlreadyLeader` / `ErrNotLeader` are public sentinels. | Strategy constructors reject invalid scorer sets. | OK |
| `leader/redis` | 6 | Wraps Redis/context causes and leader sentinels. | Campaign/resign/context behavior tested with Redis. | OK |
| `lock/redis` | 5 | `ErrNotAcquired` sentinel plus Redis/context wrapping. | Lease unlock and cancellation behavior tested. | OK |
| `measure` | 206 | Mature sentinels for unit, measure, parse, compatibility, divide-by-zero; `ParseError` supports matching. | Value types validate finite amounts; `Must*` variants are explicit. | P3: large surface should be rechecked before #221 math/time additions. |
| `money` | 30 | Mature sentinels for currency, amount, mismatch, overflow, exchange-rate/provider states. | Zero values rejected where unsafe; provider cancellation tested. | OK |
| `probabilistic` | 11 | Sentinels plus `ConfigError` for invalid config; compatibility errors tested. | Config and hasher zero-value behavior guarded. | OK |
| `probabilistic/redis` | 7 | Redis Bloom sentinels for options/config mismatch/corrupt; `RedisError` supports `errors.As`. | Config immutability and context behavior tested. | OK |
| `ratelimit` | 11 | Context errors propagate; validation errors are plain. | Token bucket and HTTP handler contracts are tested. | P2: future public matching need may justify package sentinels. |
| `ratelimit/redis` | 3 | Redis/context causes wrap; validation errors are plain. | Redis server-time refill and cancellation behavior covered. | P2: keep cleanup/timing behavior under #215/#209 watch. |
| `resilience` | 51 | Mature typed errors and sentinels for retry, timeout, circuit, and bulkhead. | Policies preserve operation/context causes and event categories. | OK |
| `serialization` | 12 | Envelope sentinels cover invalid envelope, version, and format mismatch. | Raw serializers reject nil input where ambiguous. | OK |
| `state` | 9 | Transition sentinels and generic `TransitionError` support `errors.Is`. | Final-state, guard, and concurrent transition behavior tested. | OK |
| `testcontainers/kafka` | 1 | Testing helper fails through `testing.T`; no caller error contract. | Starts fixture and returns broker addresses. | OK |
| `testcontainers/mysql` | 1 | Testing helper fails through `testing.T`; no caller error contract. | Starts fixture and returns DSN. | OK |
| `testcontainers/nats` | 1 | Testing helper fails through `testing.T`; no caller error contract. | Starts fixture and returns URL. | OK |
| `testcontainers/postgres` | 1 | Testing helper fails through `testing.T`; no caller error contract. | Starts fixture and returns DSN. | OK |
| `testcontainers/redis` | 1 | Testing helper fails through `testing.T`; no caller error contract. | Starts fixture and returns address. | OK |
| `testing` | 4 | Helpers fail the test directly, matching Go `testing` style. | Polling/timeout behavior is explicit. | OK |
| `testing/concurrency` | 8 | `RunError` supports `errors.Is` / `errors.As`, including panic causes. | Options validate workers, rounds, and timeout. | OK |
| `workflow` | 7 | Runner sentinels cover nil work/predicate and invalid report status. | Context cancellation maps into work reports. | OK |
| `workreport` | 13 | `ErrUnknownFailurePolicy` plus typed `FailurePolicyError`; child errors preserve causes. | Reports are immutable enough for caller consumption; children are copied. | OK |

## Follow-up Routing

| Severity | Candidate | Route | Rationale |
|---|---|---|---|
| P2 | Shared validation sentinels for `core`, `collections`, `ratelimit`, and small constructor helpers | #204 | Only add if the core parity work proves callers need stable semantic matching. |
| P2 | `cache/rediscoord` operator-facing error categories | #215 or later cache issue | Current wrapping is sufficient; categorization may help only if GenericServer/property-export work reuses it. |
| P2 | Redis timing/cancellation helper contracts | #209 / #215 | Existing tests passed, but Testcontainers cleanup and time-bound gates remain cross-cutting risks. |
| P2 | Public API shape before adding range/collection primitives | #204 / #206 | Avoid importing Kotlin extension/DSL shapes into Go. |
| P2 | Public testing helper shape before adding temp/output/env/faker/reporting helpers | #209 / #214 | Keep helpers as `testing.TB` utilities, not a framework. |
| P3 | Codec invalid-input sentinel | Later maintenance issue only if callers ask for matching | Current callers generally treat decode errors as invalid input without branching. |
| P3 | Measure surface size | #221 / #223 | Large exported unit catalog is intentional, but future math/time additions should avoid uncontrolled expansion. |

## Non-goals

- Do not add broad sentinels to every validation error just for symmetry.
- Do not convert small helper packages into Kotlin-style extension surfaces.
- Do not make breaking public API changes inside #203 without a package-specific
  migration issue.
- Do not replace `testing.T` fixture helpers with error-returning APIs unless
  #215 selects a reusable generic server contract.

## Acceptance Check

- Exported symbols were grouped by package.
- Error contracts were checked for sentinel/typed matching where callers need
  stable behavior.
- README/root documentation was already corrected by #202/#245 before this
  audit; no new P0/P1 doc mismatch remained in this pass.
- No P0/P1 contract issue is hidden in this tracking issue; follow-up candidates
  are routed above.
