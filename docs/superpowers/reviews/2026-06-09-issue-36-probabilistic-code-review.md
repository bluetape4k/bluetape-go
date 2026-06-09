# Issue #36 Step 6-R Code Review

Date: 2026-06-09
Scope: current `issue-36-probabilistic` branch diff against `origin/develop`.

## Integrated Gate

| Tier | Scope | P0 | P1 | P2 | P3 | Result |
|---|---|---:|---:|---:|---:|---|
| 1 | Security | 0 | 0 | 0 | 0 | PASS |
| 2 | Ops/SRE reliability | 0 | 0 | 0 | 0 | PASS |
| 3 | Structural impact | 0 | 0 | 0 | 0 | PASS |
| 4 | Go code quality | 0 | 0 | 0 | 0 | PASS |
| 5 | Tests/types/silent failure | 0 | 0 | 0 | 0 | PASS |
| 6 | Performance/stability | 0 | 0 | 0 | 0 | PASS |
| 7 | Documentation/release/evidence | 0 | 0 | 0 | 0 | PASS |

Final gate: `P0=0`, `P1=0`.

Subagent integration:

- API/security/code quality lane: initial result `P0=0`, `P1=2`. The merge API
  P1 was resolved by sealing `BloomFilter[T]` with an unexported method and
  documenting that only package-created filters can merge. The custom hasher P1
  was resolved by documenting deterministic/goroutine-safe callback requirements
  and adding `TestBloomFilterStressCustomHasher`.
- Tests/performance lane: `P0=0`, `P1=0`, one P2 on custom hasher concurrency
  contract. Resolved by documenting that custom hashers must be deterministic
  and goroutine-safe, and by adding `TestBloomFilterStressCustomHasher`.
- Docs/release/evidence lane: `P0=0`, `P1=0`; PR/CI and release readiness remain
  pending by design until Step 7 and Step 9.
- Follow-up API/security/code quality re-review after fixes: `P0=0`, `P1=0`,
  `P2=0`, `P3=0`. Prior P1s on external interface implementation and custom
  hasher concurrency were confirmed resolved.

## Tier Evidence

### Tier 1: Security

No security issue found. The new package is in-memory only, performs no I/O,
does not parse untrusted structured formats, does not touch auth/authz, and does
not add secrets or configuration files. Hashing uses `crypto/sha256` internally
for Bloom offsets.

### Tier 2: Ops/SRE Reliability

No reliability issue found. Constructors return typed errors for invalid config,
nil filters, nil hashers, empty hasher keys, and incompatible merge inputs. The
package allocates only in-memory bitsets and has no cleanup, lifecycle, retry, or
shutdown hooks.

### Tier 3: Structural Impact

No structural blocker found. `BloomFilter[T]` is exported as a sealed,
constructor-owned interface while the concrete implementation remains unexported,
avoiding unsafe zero-value use and unsupported external implementations. `PutAll`
accepts only package-created compatible filters and returns `ErrIncompatibleFilter`
for incompatible package-created filters.

### Tier 4: Go Code Quality

No Go quality blocker found. Exported API comments start with exported
identifiers, constructors avoid panics except for impossible built-in hasher
initialization/default config invariants, and the hasher compatibility contract is
explicit through `Hasher.Key()`. Custom hashers must be deterministic and
goroutine-safe; that caller-owned contract is documented in the API and package
README files.

Quick scan:

```bash
rg "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" .
```

Result: existing repository hits plus new `probabilistic` invariant panics only;
no context, goroutine, HTTP trust-boundary, or background lifecycle surface was
added by #36.

### Tier 5: Tests/Types/Silent Failure

No test blocker found. The tests cover config validation, no false negatives for
inserted values not followed by `Clear`, bounded deterministic FPP, `Put`
bit-change semantics, `Clear`, compatible and incompatible `PutAll`, nil filter
handling, custom hasher key compatibility, examples, and race/stress paths for
`Put`, `MightContain`, metadata, reciprocal `PutAll`, self-merge, and `Clear`.
`TestBloomFilterStressCustomHasher` also exercises the custom hasher callback
path under stress/race validation.

`AsyncJobTester: N/A` because #36 exposes no context-aware background job API.

### Tier 6: Performance/Stability

No performance or stability issue found in the final diff. Hot paths allocate
hash-offset slices per operation and use SHA-256 intentionally for deterministic
first-party hashing. `PutAll` snapshots the source under `RLock` before taking
the target `Lock`, avoiding reciprocal merge deadlock. Stress/race tests cover
shared state and cache-like mutation requirements.

### Tier 7: Documentation/Release/Evidence

No documentation/release blocker found. `README.md`, `README.ko.md`,
`CHANGELOG.md`, `WIP.md`, package README files, spec, plan, review artifacts, and
test log were updated. Redis-backed probabilistic filters are explicitly deferred
to #182 and not bundled into #36.

## Validation

```bash
go test -count=1 ./probabilistic
go test -race -count=1 ./probabilistic
git diff --check
rg -n "AsyncJobTester: N/A|probabilistic|#182|Redis-backed Bloom" docs/superpowers README.md README.ko.md WIP.md
golangci-lint cache clean
make ci
```

Result: PASS. `make ci` initially surfaced stale `golangci-lint` cache output
from removed sibling worktree `.worktrees/issue-35-money`; after
`golangci-lint cache clean`, current branch validation passed with `0 issues.`
and all packages green. Latest full `make ci` after sealed-interface and custom
hasher stress fixes also passed at `2026-06-09 15:25:30 KST`.
