# Issue #354 Core Parity Matrix

Date: 2026-07-03

Issue: [#354](https://github.com/bluetape4k/bluetape-go/issues/354)
Parent: [#353](https://github.com/bluetape4k/bluetape-go/issues/353)
Milestone: `0.12.0`

## Purpose

This matrix maps `bluetape4k-projects/bluetape4k/core` source areas to the
current or proposed bluetape-go package boundaries before 0.12.0 implementation
continues. The goal is selective Go-native parity, not a mechanical Kotlin/JVM
port.

Decision labels:

- `keep`: current Go package already owns the useful subset.
- `adapt`: port the concept with Go idioms, error returns, generics, and
  `context.Context` where appropriate.
- `replace`: use a 0.12.0 implementation issue to replace weak, duplicated, or
  too-small current helpers with source-backed Go behavior.
- `split later`: useful, but belongs outside the current core foundation.
- `non-goal`: JVM/Kotlin-specific or too broad for bluetape-go.

## Source Evidence

Kotlin source tree reviewed:

- `bluetape4k/core/README.md` and `README.ko.md`
- `bluetape4k/core/src/main/kotlin/io/bluetape4k/*`
- Notable groups: `apache`, `codec`, `collections`, `concurrent`,
  `exceptions`, `functional`, `javatimes`, `ranges`, `support`,
  `support/i18n`, and `utils`

Current Go packages reviewed:

- `core`
- `collections`
- `codec`
- `concurrency`

Prior planning notes reviewed:

- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`
- `docs/superpowers/research/2026-06-24-issue-223-utility-parity.md`
- `docs/research/2026-07-02-issue-37-rule-engine-primitives.md`

## Parity Matrix

| Kotlin source package/file group | Existing/proposed Go package | Current Go status | Decision | Replacement candidate | Rationale and compatibility notes | Follow-up |
|---|---|---|---|---|---|---|
| Root value-object helpers: `AbstractValueObject.kt`, `ValueObject.kt`, `DefaultFields.kt`, `GetterSetter.kt`, `ToStringBuilder.kt`, `SortDirection.kt` | Existing domain packages; no shared base package | No abstract value-object framework | non-goal | No | Go value types should expose concrete fields, methods, `String`, `MarshalText`, or package-local comparison. A shared inheritance-like base would be unidiomatic and would not replace current APIs. | None |
| `support/RequireSupport.kt`, `AssertSupport.kt` | `core` | `RequireNotBlank`, `RequireNotEmpty`, range and numeric checks return errors wrapping `core.ErrInvalidArgument` | adapt / replace | Yes | Keep error-return validation. Do not port Kotlin contracts, exception hierarchies, or assertion DSL panics. Replace scattered package validation only when #359 adds source-backed helpers with tests. | [#359](https://github.com/bluetape4k/bluetape-go/issues/359) |
| `support/StringSupport.kt` | `core` | `HasText`, `EmptyToDefault`, `BlankToDefault`, `TruncateUTF8Bytes` | adapt / replace | Yes | Add only repeated caller needs such as blank checks, byte-length validation, safe truncation, or small masking/prefix helpers. Reject broad Apache-style aliases for `strings` and `unicode/utf8`. | [#359](https://github.com/bluetape4k/bluetape-go/issues/359) |
| `support/UuidSupport.kt` | `core` or package-local helper | No UUID helper in `core` | adapt | Yes | UUID support should be narrow: parse/validate, zero UUID handling, text compatibility, and possibly byte conversion if required by existing codec/id packages. Do not add a global UUID abstraction unless implementation evidence shows repeated use. | [#359](https://github.com/bluetape4k/bluetape-go/issues/359) |
| `support/ObjectSupport.kt`, `AnySupport.kt`, primitive/array helpers, boolean/number/byte-array helpers | `core` | `Ptr`, `ValueOr`, zero/default helpers, `Clamp`, hex checks | adapt / replace | Yes | Keep small generic fallback helpers that remove repeated boilerplate. Reject Kotlin extension-surface breadth, reflection shortcuts, and object identity helpers unless a package has concrete duplication. | [#359](https://github.com/bluetape4k/bluetape-go/issues/359) |
| `ranges/*` | `core` | Generic ordered `Range` with open/closed constructors | keep | No | Current Go API covers explicit boundary semantics and rejects invalid ranges. Kotlin operator overloads and DSL constructors remain intentionally excluded. | None |
| `javatimes/*` | `core` plus standard `time` | `Quarter`, `YearQuarter`, date iteration | keep / split later | No | Keep small reporting-calendar helpers already in `core`. Broad Java Time mirrors, period frameworks, duration DSLs, and parser/formatter wrappers stay out of core unless a future package proves repeated demand. | None |
| `codec/*`: `Base58.kt`, `Base62.kt`, `Base64StringEncoder.kt`, `HexStringEncoder.kt`, `StringEncoder*.kt`, `Url62.kt` | `codec` | Base58, Base62, URL62 alias, Base64, Base64URL, Hex, UTF-8 string helpers | keep / adapt | Yes | Existing `codec` has compatibility vectors and binary-safe APIs. #357 should audit remaining URL62/time/UUID-oriented source parity and replace only proven gaps, preserving current API names when stronger. | [#357](https://github.com/bluetape4k/bluetape-go/issues/357) |
| `collections/BoundedStack.kt`, `RingBuffer.kt`, `PaginatedList.kt`, `permutations/*` | `collections` | Bounded stack, ring buffer, page, lazy permutations | keep | No | The main data structures already exist with Go generics, error-return constructors, snapshot semantics, and examples. Future work should improve gaps, not rebuild the package. | [#360](https://github.com/bluetape4k/bluetape-go/issues/360) |
| `collections/CollectionSupport.kt`, `MapSupport.kt`, `SequenceSupport.kt`, `ListSupport.kt`, `SetSupport.kt` | `collections` | `Chunk`, `ChunkBy`, `Distinct`, `DistinctBy`, `MapErr`, `FilterErr`, `FilterMap`, `GroupBy`, `CountBy` | adapt / replace | Yes | Keep helpers when they encode error-aware generic workflows that Go lacks. Avoid thin aliases for `slices`, `maps`, or plain loops. #360 should replace weak overlap and add only source-backed high-value helpers. | [#360](https://github.com/bluetape4k/bluetape-go/issues/360) |
| `collections/eclipse/*`, Java stream/iterator adapters | None | No counterpart | non-goal | No | Tied to JVM collection libraries and Kotlin sequence idioms. Go callers should use slices, maps, iterators, and explicit adapters local to packages. | None |
| `concurrent/*`: future/completable-future/executor/lock/thread utilities | `concurrency` | `Group`, `Go`, `ForEach`, `Map`, `WorkerPool`, panic-to-error | adapt / replace | Yes | Replace JVM executor/future concepts with context-aware goroutine helpers. #355 should focus cancellation, bounded work, panic handling, lifecycle tests, and README contract clarity. | [#355](https://github.com/bluetape4k/bluetape-go/issues/355) |
| `concurrent/virtualthread/*` | None | No counterpart | non-goal | No | JVM virtual threads do not map to Go. Goroutines and `context.Context` are the target runtime model. | None |
| `exceptions/*` | Package-local sentinels and wrapped errors | Existing packages use sentinel errors such as `ErrInvalidArgument`, `ErrInvalidTime`, and `ErrInvalidUTF8` | adapt | No | Keep Go error values and wrapping. Do not port exception classes or throw-style flow control. | Per implementation issue |
| `functional/*`, `support/ResultSupport.kt` | Proposed rules package or local domain code | No general functional package | split later / non-goal | No | General monads, lambdas, and result DSLs do not belong in core. Rule-engine primitives are already separated from core foundation work. | [#375](https://github.com/bluetape4k/bluetape-go/issues/375), [#377](https://github.com/bluetape4k/bluetape-go/issues/377), [#376](https://github.com/bluetape4k/bluetape-go/issues/376) |
| `utils/Wildcard.kt` | `core` | `MatchWildcard`, `FirstWildcardMatch`, path matching, malformed pattern sentinel | keep | No | Current helpers are Go-native, lexical, portable, and documented. Keep in `core` because package filters and key matchers can share them. | None |
| `utils/XXHasher.kt` | `core` | `XXH64Bytes`, `XXH64String` | keep | No | Current API documents non-cryptographic use and matches repeated cache/key needs. Do not broaden into cryptographic helpers. | None |
| `utils/Resourcex.kt`, `Systemx.kt`, `ShutdownQueue.kt`, temp/env/output helpers | None or package-local | No broad utility package | non-goal | No | Go standard packages (`os`, `io/fs`, `runtime`, `signal`, `context`) are clearer. Shared wrappers should appear only when a package owns a concrete operational contract. | None |
| `support/ClassSupport.kt`, `ClassLoaderSupport.kt`, `JavaTypeSupport.kt`, `KotlinDetector.kt`, reflection helpers | None | No counterpart | non-goal | No | JVM classpath, Kotlin runtime detection, and Java type helpers are not portable Go concerns. | None |
| `support/i18n/*` | Existing locale/currency packages where needed | Locale/currency work lives outside core | split later | No | I18n behavior should remain domain-specific, not a core foundation helper. Existing money/locale work should own compatibility. | None |
| `apache/*` wrapper helpers | Standard library or focused Go packages | No wrapper layer | non-goal | No | Do not port Apache Commons facades. Prefer `strings`, `bytes`, `unicode`, `math`, `path/filepath`, `net`, and small first-party helpers only where repeated bluetape-go code proves value. | None |
| Logging-like helper expectations from JVM ecosystem | Proposed `slog` conventions | No global logger facade | adapt | No | Use Go `log/slog` and explicit dependency injection/context fields rather than a bluetape4k-logging style facade. | [#361](https://github.com/bluetape4k/bluetape-go/issues/361) |

## Replacement Queue

0.12.0 implementation should apply replacements in this order:

1. [#359](https://github.com/bluetape4k/bluetape-go/issues/359) owns `core`
   replacement work. Add source-backed string validation, UUID helpers, and
   default/require consolidation only after tests show repeated use.
2. [#360](https://github.com/bluetape4k/bluetape-go/issues/360) owns
   `collections` helper review. Keep the existing data structures; replace only
   thin or duplicated helpers with higher-value generic functions.
3. [#357](https://github.com/bluetape4k/bluetape-go/issues/357) owns `codec`
   parity gaps. Preserve current binary-safe APIs and compatibility vectors.
4. [#355](https://github.com/bluetape4k/bluetape-go/issues/355) owns
   `concurrency` design. Replace JVM futures/executors with context-aware
   goroutine contracts.
5. [#361](https://github.com/bluetape4k/bluetape-go/issues/361) owns logging
   conventions using `log/slog`; no global facade is introduced here.
6. [#375](https://github.com/bluetape4k/bluetape-go/issues/375),
   [#377](https://github.com/bluetape4k/bluetape-go/issues/377), and
   [#376](https://github.com/bluetape4k/bluetape-go/issues/376) keep rule
   primitives outside `core`.

## Non-Goal Guardrails

- No Kotlin extension-surface port.
- No JVM reflection, classpath, virtual-thread, executor, or
  `CompletableFuture` compatibility layer.
- No Apache Commons wrapper package.
- No global logging facade.
- No public API change is made by this document.

## Acceptance Check

- Matrix covers the Kotlin source package/file groups that shape 0.12.0 core
  foundation work.
- Replacement candidates are linked to implementation issues for `core`,
  `collections`, `codec`, and `concurrency`.
- JVM/Kotlin-only surfaces are explicitly marked as non-goals.
- Public roadmap/index documentation links this 0.12.0 source-parity note.
