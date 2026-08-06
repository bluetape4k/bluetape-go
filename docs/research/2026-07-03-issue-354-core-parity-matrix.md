# Issue #354 Core Parity Matrix

Date: 2026-07-03

Issue: [#354](https://github.com/bluetape4k/bluetape-go/issues/354)
Parent: [#353](https://github.com/bluetape4k/bluetape-go/issues/353)
Milestone: `0.12.0`

## Purpose

이 matrix는 0.12.0 implementation이 계속되기 전에 `bluetape4k-projects/bluetape4k/core` source 영역을 현재 또는 제안된
bluetape-go package boundary에 매핑한다. 목표는 selective Go-native parity이지 mechanical Kotlin/JVM port가 아니다.

decision label:

- `keep`: 현재 Go package가 이미 유용한 subset을 소유한다.
- `adapt`: Go idiom, error return, generics, 필요 시 `context.Context`로 개념을 옮긴다.
- `replace`: 0.12.0 implementation issue에서 약하거나 중복되거나 너무 작은 helper를 source-backed Go behavior로 대체한다.
- `split later`: 유용하지만 현재 core foundation 밖에 둔다.
- `non-goal`: JVM/Kotlin-specific이거나 bluetape-go에 너무 넓다.

## Source Evidence

review한 Kotlin source tree:

- `bluetape4k/core/README.md` and `README.ko.md`
- `bluetape4k/core/src/main/kotlin/io/bluetape4k/*`
- notable groups: `apache`, `codec`, `collections`, `concurrent`, `exceptions`, `functional`, `javatimes`, `ranges`,
  `support`, `support/i18n`, `utils`

review한 현재 Go package:

- `core`
- `collections`
- `codec`
- `concurrency`

review한 prior planning note:

- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`
- `docs/superpowers/research/2026-06-24-issue-223-utility-parity.md`
- `docs/research/2026-07-02-issue-37-rule-engine-primitives.md`

## Parity Matrix

| Kotlin source group | Go boundary | Decision | Replacement candidate | Rationale / follow-up |
|---|---|---|---|---|
| root value-object helper: `AbstractValueObject.kt`, `ValueObject.kt`, `DefaultFields.kt`, `GetterSetter.kt`, `ToStringBuilder.kt`, `SortDirection.kt` | existing domain package | non-goal | No | Go value type은 concrete field, method, `String`, `MarshalText`, package-local comparison을 노출해야 한다. inheritance-like base는 unidiomatic이며 current API를 대체하지 않는다. |
| `support/RequireSupport.kt`, `AssertSupport.kt` | `core` | adapt / replace | Yes | error-return validation을 유지한다. Kotlin contract, exception hierarchy, assertion DSL panic은 port하지 않는다. source-backed helper는 [#359](https://github.com/bluetape4k/bluetape-go/issues/359)에서 다룬다. |
| `support/StringSupport.kt` | `core` | adapt / replace | Yes | blank check, byte-length validation, safe truncation, small masking/prefix helper처럼 반복 caller need만 추가한다. Apache-style alias는 거부한다. [#359](https://github.com/bluetape4k/bluetape-go/issues/359). |
| `support/UuidSupport.kt` | `core` 또는 package-local helper | adapt | Yes | parse/validate, zero UUID, text compatibility, 필요 시 byte conversion처럼 좁게 둔다. global UUID abstraction은 repeated use evidence 없이는 추가하지 않는다. [#359](https://github.com/bluetape4k/bluetape-go/issues/359). |
| `support/ObjectSupport.kt`, `AnySupport.kt`, primitive/array helper | `core` | adapt / replace | Yes | 반복 boilerplate를 줄이는 작은 generic fallback helper만 유지한다. Kotlin extension breadth, reflection shortcut, object identity helper는 concrete duplication 없이는 거부한다. [#359](https://github.com/bluetape4k/bluetape-go/issues/359). |
| `ranges/*` | `core` | keep | No | current Go API가 explicit boundary semantics와 invalid range rejection을 덮는다. Kotlin operator overload 및 DSL constructor는 제외한다. |
| `javatimes/*` | `core` plus `time` | keep / split later | No | `Quarter`, `YearQuarter`, date iteration 같은 작은 reporting-calendar helper만 유지한다. broad Java Time mirror와 parser/formatter wrapper는 future package가 반복 demand를 증명할 때까지 제외한다. |
| `codec/*` | `codec` | keep / adapt | Yes | existing `codec`은 compatibility vector와 binary-safe API를 가진다. #357은 URL62/time/UUID-oriented parity gap만 audit하고 더 강한 경우 current API name을 보존한다. [#357](https://github.com/bluetape4k/bluetape-go/issues/357). |
| `collections/BoundedStack.kt`, `RingBuffer.kt`, `PaginatedList.kt`, `permutations/*` | `collections` | keep | No | data structure는 이미 Go generics, error-return constructor, snapshot semantics, example을 가진다. future work는 rebuild가 아니라 gap 개선이다. [#360](https://github.com/bluetape4k/bluetape-go/issues/360). |
| `collections/*Support.kt` | `collections` | adapt / replace | Yes | Go가 부족한 error-aware generic workflow만 유지한다. `slices`, `maps`, plain loop thin alias는 피한다. [#360](https://github.com/bluetape4k/bluetape-go/issues/360). |
| `collections/eclipse/*`, Java stream/iterator adapter | none | non-goal | No | JVM collection library 및 Kotlin sequence idiom에 묶여 있다. Go caller는 slice, map, iterator, package-local adapter를 쓴다. |
| `concurrent/*` future/executor/lock/thread utilities | `concurrency` | adapt / replace | Yes | JVM executor/future 대신 context-aware goroutine contract를 쓴다. #355는 cancellation, bounded work, panic handling, lifecycle test, README contract clarity에 집중한다. [#355](https://github.com/bluetape4k/bluetape-go/issues/355). |
| `concurrent/virtualthread/*` | none | non-goal | No | JVM virtual thread는 Go에 매핑되지 않는다. target runtime model은 goroutine과 `context.Context`다. |
| `exceptions/*` | package-local sentinel 및 wrapped error | adapt | No | Go error value와 wrapping을 유지한다. exception class 또는 throw-style control flow는 port하지 않는다. |
| `functional/*`, `support/ResultSupport.kt` | rules package 또는 local domain code | split later / non-goal | No | general monad, lambda, result DSL은 core에 속하지 않는다. rule-engine primitive는 core foundation 밖에서 추적한다. [#375](https://github.com/bluetape4k/bluetape-go/issues/375), [#377](https://github.com/bluetape4k/bluetape-go/issues/377), [#376](https://github.com/bluetape4k/bluetape-go/issues/376). |
| `utils/Wildcard.kt` | `core` | keep | No | current helper는 Go-native, lexical, portable, documented다. package filter와 key matcher가 공유할 수 있으므로 `core`에 둔다. |
| `utils/XXHasher.kt` | `core` | keep | No | current API는 non-cryptographic use를 문서화하고 repeated cache/key need에 맞는다. cryptographic helper로 넓히지 않는다. |
| `utils/Resourcex.kt`, `Systemx.kt`, `ShutdownQueue.kt` 등 | none or package-local | non-goal | No | `os`, `io/fs`, `runtime`, `signal`, `context` 같은 Go standard package가 더 명확하다. shared wrapper는 concrete operational contract가 있을 때만 만든다. |
| `support/ClassSupport.kt`, `ClassLoaderSupport.kt`, `JavaTypeSupport.kt`, `KotlinDetector.kt` | none | non-goal | No | JVM classpath, Kotlin runtime detection, Java type helper는 Go concern이 아니다. |
| `support/i18n/*` | domain package | split later | No | i18n behavior는 core foundation helper가 아니라 domain-specific이어야 한다. |
| `apache/*` wrapper helper | standard library 또는 focused package | non-goal | No | Apache Commons facade는 port하지 않는다. `strings`, `bytes`, `unicode`, `math`, `path/filepath`, `net`과 작은 first-party helper를 우선한다. |
| JVM logging-like expectation | `slog` convention | adapt | No | global logger facade 대신 Go `log/slog`와 explicit dependency injection/context field를 사용한다. [#361](https://github.com/bluetape4k/bluetape-go/issues/361). |

## Replacement Queue

0.12.0 implementation은 다음 순서로 replacement를 적용한다.

1. [#359](https://github.com/bluetape4k/bluetape-go/issues/359): `core` replacement. source-backed string validation,
   UUID helper, default/require consolidation은 test가 repeated use를 보일 때만 추가한다.
2. [#360](https://github.com/bluetape4k/bluetape-go/issues/360): `collections` helper review. 기존 data structure는 유지하고
   thin 또는 duplicated helper만 high-value generic function으로 대체한다.
3. [#357](https://github.com/bluetape4k/bluetape-go/issues/357): `codec` parity gap. current binary-safe API와 compatibility
   vector를 보존한다.
4. [#355](https://github.com/bluetape4k/bluetape-go/issues/355): `concurrency` design. JVM future/executor 대신
   context-aware goroutine contract로 대체한다.
5. [#361](https://github.com/bluetape4k/bluetape-go/issues/361): `log/slog` 기반 logging convention. global facade는 없다.
6. [#375](https://github.com/bluetape4k/bluetape-go/issues/375),
   [#377](https://github.com/bluetape4k/bluetape-go/issues/377),
   [#376](https://github.com/bluetape4k/bluetape-go/issues/376): rule primitive는 `core` 밖에 둔다.

## Non-Goal Guardrails

- Kotlin extension-surface port 없음.
- JVM reflection, classpath, virtual-thread, executor, `CompletableFuture` compatibility layer 없음.
- Apache Commons wrapper package 없음.
- global logging facade 없음.
- 이 문서는 public API change를 만들지 않는다.

## Acceptance Check

- matrix가 0.12.0 core foundation work를 shaping하는 Kotlin source package/file group을 덮는다.
- `core`, `collections`, `codec`, `concurrency` implementation issue에 replacement candidate가 link되어 있다.
- JVM/Kotlin-only surface는 non-goal로 명시되어 있다.
- public roadmap/index documentation이 이 0.12.0 source-parity note를 link한다.
