# Issue 32 ID Generator Preimplementation Risk

Plan task: T0
Issue: #32
Milestone: 0.6.0
Baseline: `64fbb11`

## Evidence

Repository checks:

- `go.mod` currently lists `github.com/google/uuid v1.6.0` as indirect.
- `go.sum` already contains `github.com/google/uuid v1.6.0`.
- `go list -m -versions github.com/google/uuid github.com/oklog/ulid/v2 github.com/gofrs/uuid/v5`
  shows latest candidate versions:
  - `github.com/google/uuid v1.6.0`
  - `github.com/oklog/ulid/v2 v2.1.1`
  - `github.com/gofrs/uuid/v5 v5.4.0`
- `go list -m -json github.com/oklog/ulid/v2@v2.1.1` reports tag
  time `2024-04-13T18:09:41Z`, module path
  `github.com/oklog/ulid/v2`, and Go version `1.15`.
- `go list -m -json github.com/gofrs/uuid/v5@v5.4.0` reports tag time
  `2025-10-30T03:30:36Z`, module path `github.com/gofrs/uuid/v5`, and Go
  version `1.19`.
- Context7 resolves `github.com/google/uuid` as `/google/uuid` with high source
  reputation. The docs confirm `NewRandom`, `NewRandomFromReader`, and `NewV7`.
  Context7 did not return an exact `github.com/oklog/ulid/v2` match, so ULID
  evidence uses local downloaded module source as primary evidence.

Local source checks:

- `github.com/google/uuid@v1.6.0` provides:
  - `type UUID [16]byte`
  - `Parse(string) (UUID, error)`
  - `NewRandom() (UUID, error)`
  - `NewRandomFromReader(io.Reader) (UUID, error)`
  - `NewV7() (UUID, error)`
  - `NewV7FromReader(io.Reader) (UUID, error)`
  - `NewV6() (UUID, error)`, which remains out of scope for 0.6.0.
- `github.com/google/uuid@v1.6.0/version4.go` documents that v4 UUID strength
  is based on `crypto/rand`; `NewRandomFromReader` returns `Nil` plus the read
  error on entropy failure.
- `github.com/google/uuid@v1.6.0/version7.go` uses `NewRandom` or
  `NewRandomFromReader`, then sets UUID v7 timestamp bits. Its internal
  `getV7Time` serializes and advances repeated or backward clock observations.
- `github.com/oklog/ulid/v2@v2.1.1` provides:
  - `type ULID [16]byte`
  - `New(ms uint64, entropy io.Reader) (ULID, error)`
  - `Parse(string) (ULID, error)`
  - `ParseStrict(string) (ULID, error)`
  - `Timestamp(time.Time) uint64`
  - `(ULID).Time() uint64`
  - `Monotonic(io.Reader, uint64) *MonotonicEntropy`
  - `LockedMonotonicReader`
- `github.com/oklog/ulid/v2@v2.1.1` has package default entropy based on
  `math/rand` through `DefaultEntropy`/`Make`; do not use those defaults for
  bluetape-go random IDs.
- `testing/concurrency` provides `NewGoroutineStressTester` and
  `NewAsyncJobTester`. T5 uses `GoroutineStressTester` for ID concurrency. If no
  context-aware batch helper is added, `AsyncJobTester` remains N/A because
  single generation has no caller-observable cancellation boundary.
- `codec/base62.go` already owns Base62 helpers; `id` must not add duplicate
  Base62 encoding in this issue.

## Decisions

| Area | Decision | Rationale |
|---|---|---|
| UUID dependency | Adopt `github.com/google/uuid v1.6.0` and make it a direct dependency when UUID code is added. | Already present indirectly, high-reputation docs, supports v4/v7 plus reader-based deterministic tests, and uses `crypto/rand` for default v4/v7 entropy. |
| UUID fallback | Reject `github.com/gofrs/uuid/v5` for 0.6.0. | It supports v4/v6/v7 and is a viable fallback, but adopting it would add a second UUID dependency while `google/uuid` already satisfies 0.6.0 v4/v7 requirements. |
| UUID public API exposure | Do not expose `google/uuid.UUID` as the stable bluetape-go API in 0.6.0. | Public API remains repo-owned string/byte parse and generation boundaries so the dependency can be replaced without breaking callers. Internal tests may compare dependency values. |
| ULID dependency | Adopt `github.com/oklog/ulid/v2 v2.1.1` when ULID code is added. | It provides canonical parse/strict parse, timestamp extraction, monotonic entropy, and reader injection. |
| ULID entropy default | Do not use `ulid.Make`, `ulid.MustNewDefault`, or `ulid.DefaultEntropy` for bluetape-go defaults. | Those defaults are based on `math/rand`. bluetape-go random and monotonic ULID defaults must pass `crypto/rand.Reader` or a caller-provided entropy source. |
| ULID public API exposure | Do not expose `ulid.ULID` as the stable bluetape-go API in 0.6.0. | Keep repo-owned string/byte parse and generation boundaries; document canonical 26-character string behavior. |
| Monotonic ULID state | Use a concurrency-safe default generator. | Spec prefers library ergonomics; implementation should protect monotonic entropy state with a mutex or `LockedMonotonicReader` rather than requiring caller synchronization. |
| Base62 | Defer compact UUID/Base62 output. | `codec` already owns Base62, and UUID compact rendering is deferred by the spec. |
| Cancellation | Do not add `context.Context` to single local ID generation. | Generation is local CPU/entropy work; cancellation tests are N/A unless a context-aware batch helper is explicitly added. |

## Implementation Constraints

- Constructors/factories must return repo-owned APIs, not dependency concrete
  types.
- No `Must*` helpers in bluetape-go `id`.
- Random UUID/ULID defaults must be crypto-grade.
- Deterministic entropy readers are test hooks only.
- UUID/ULID failing-reader tests must assert error wrapping and no zero ID with
  nil error.
- Parse wrappers must expose typed/sentinel errors compatible with `errors.Is`
  and wrap dependency parse errors with `%w`.
- Snowflake remains first-party and must not depend on UUID/ULID packages.
- Snowflake machine ID allocation is caller-owned; this issue must not add a
  global allocator or host identity discovery.

## Validation

Commands run for this decision:

```bash
go list -m -versions github.com/google/uuid github.com/oklog/ulid/v2 github.com/gofrs/uuid/v5
go list -m -json github.com/google/uuid
go mod download -json github.com/oklog/ulid/v2@v2.1.1 github.com/gofrs/uuid/v5@v5.4.0
go list -m -json github.com/oklog/ulid/v2@v2.1.1 github.com/gofrs/uuid/v5@v5.4.0
rg -n "github.com/google/uuid|github.com/oklog/ulid" go.mod go.sum
rg -n "func NewV7|func NewV7FromReader|func NewRandom|func NewRandomFromReader|func Parse|type UUID" /Users/debop/go/pkg/mod/github.com/google/uuid@v1.6.0
rg -n "func Make|func New|func ParseStrict|func Monotonic|type ULID|DefaultEntropy|math/rand|crypto/rand" /Users/debop/go/pkg/mod/github.com/oklog/ulid/v2@v2.1.1
```

T0 result: PASS. Implementation can proceed to T1 after dependency additions
are made intentionally in the source diff.
