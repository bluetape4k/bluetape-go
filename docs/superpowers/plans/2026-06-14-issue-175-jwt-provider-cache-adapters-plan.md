# Issue #175 JWT Provider Cache Adapters Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development or superpowers:executing-plans to
> implement this plan task-by-task. Keep task checkboxes updated.

**Goal:** Add optional JWT provider cache adapters that reduce repeated parse
and signature verification cost without weakening provider-owned validation.

**Architecture:** Add `CachedProvider` and `CachedDistributedProvider` in
package `jwt`. They wrap existing providers, use trusted
`cache.Cache[string,*Reader]` backends, build scoped token-digest cache keys,
coalesce same-key misses per adapter, dynamically clip TTL after validation,
and revalidate cached hits against provider key state before returning.

**Tech Stack:** Go, `cache.Cache`, `golang.org/x/sync/singleflight`,
`testing/concurrency.GoroutineStressTester`,
`testing/concurrency.AsyncJobTester`, `$bluetape4k-diagram` README assets.

---

## Source Specification

- Spec: `docs/superpowers/specs/2026-06-14-issue-175-jwt-provider-cache-adapters-design.md`
- Step 2-R evidence: `docs/superpowers/reviews/2026-06-14-issue-175-jwt-provider-cache-adapters-step-2r-spec-review.md`
- Issue: #175
- Branch/worktree: `.worktrees/issue-175-jwt-provider-cache-adapters`

## File Structure

- Modify `jwt/parser_options.go`: add `customClock bool` to `parseConfig`.
- Create `jwt/cache_options.go`: cache config, options, validation helpers,
  typed-nil checks, trust-scope generation.
- Create `jwt/cache_key.go`: low-allocation cache key/profile builder.
- Create `jwt/cached_provider.go`: `CachedProvider`.
- Create `jwt/cached_distributed_provider.go`: `CachedDistributedProvider`.
- Create `jwt/cached_provider_test.go`: local adapter behavior and edge cases.
- Create `jwt/cached_distributed_provider_test.go`: distributed adapter
  behavior, hit revalidation, rotation/reset clearing.
- Create `jwt/cache_key_test.go`: profile/key determinism and raw-token
  non-leak tests.
- Create `jwt/cache_failure_test.go`: cache error propagation and stale delete
  failure tests.
- Create `jwt/cache_concurrency_test.go`: `GoroutineStressTester` and
  `AsyncJobTester` coverage.
- Create `jwt/cache_benchmark_test.go`: hit/miss/key-builder benchmarks.
- Modify `jwt/jwt_example_test.go`: compile-checked local cached-provider
  example.
- Modify `jwt/redis/example_test.go`: compile-checked distributed
  cached-provider construction example.
- Modify `jwt/README.md` and `jwt/README.ko.md`: selection guide, local and
  distributed examples, caveats, operator notes.
- Modify root `README.md` and `README.ko.md` only if package table/status text
  references JWT cache adapter deferral.
- Modify `CHANGELOG.md`: `[Unreleased]` note.
- Keep `cache/README.md` and `cache/README.ko.md` unchanged unless the
  implementation changes the generic cache contract.
- Keep generated diagram outputs:
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.dot`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.plain`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow-graphviz.svg`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow-graphviz.png`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.svg`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.png`
- Create Step 3-R review:
  `docs/superpowers/reviews/2026-06-14-issue-175-jwt-provider-cache-adapters-step-3r-plan-review.md`.
- Create Step 6-R review:
  `docs/superpowers/reviews/2026-06-14-issue-175-jwt-provider-cache-adapters-step-6r-code-review.md`.

## Step 3-R Plan Review Plan

Before implementation, run the mandatory 7-Tier gate as six independent
subagent lanes plus main integration:

1. Tier 1 Performance: singleflight, warm-hit cost, key-builder allocations,
   benchmark gates, distributed hit lookup cost.
2. Tier 2 Stability: context semantics, cache failures, stale deletion,
   rotation/reset, race/stress gates.
3. Tier 3 Security: trust scope, trusted cache boundary, hit revalidation,
   raw token/digest non-leak, auth caveats.
4. Tier 4 Operator/Ops: clear scope, diagnostics, rotation/reset runbook,
   process-local vs unsupported shared external cache.
5. Tier 5 Developer/API: constructor signatures, options, interface
   assertions, docs, compatibility with existing provider patterns.
6. Tier 6 User/Caller: examples, migration, README EN/KO parity,
   `WithParseClock` bypass clarity.

Exit condition: integrated table shows `P0=0 P1=0`.

## Step 4-T Implementation Tasks

### Task 1: Parser Option Cacheability Marker

**Files:**
- Modify `jwt/parser_options.go`
- Create/update `jwt/cache_key_test.go`

- [ ] Write failing tests proving `WithParseClock` marks parse config as
  custom-clock/cache-bypass.
- [ ] Add `customClock bool` to `parseConfig`.
- [ ] Set `customClock = true` in `WithParseClock`.
- [ ] Add internal cache-profile normalization that returns
  `cacheable=false` for custom clocks.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'ParseClock|CacheProfile'
```

### Task 2: Cache Options and Constructor Validation

**Files:**
- Create `jwt/cache_options.go`
- Create/update `jwt/cached_provider_test.go`
- Create/update `jwt/cached_distributed_provider_test.go`

- [ ] Write failing tests for nil and typed-nil provider/cache rejection.
- [ ] Write failing tests for nil option, zero/negative max TTL, empty/control
  key prefix, empty/control trust scope, and nil clock.
- [ ] Implement `CacheOption`, `cacheConfig`, defaults, random per-adapter
  trust scope, and typed-nil helper.
- [ ] Preserve repo error style with `OptionError` names from the spec.
- [ ] Add repo-style Go doc comments for every exported constructor, option
  type/function, adapter type, and method.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'Cached.*Constructor|CacheOption'
```

### Task 3: Cache Key/Profile Builder

**Files:**
- Create `jwt/cache_key.go`
- Create/update `jwt/cache_key_test.go`

- [ ] Write failing tests for deterministic keys across identical token/profile
  inputs.
- [ ] Write failing tests proving keys differ by token digest, parse profile,
  algorithm, trust scope, key prefix, issuer, subject, audience order, leeway,
  and expiration-required flag.
- [ ] Write tests proving raw token, token digest, and profile hash are not
  exposed in errors/log-like strings.
- [ ] Implement low-allocation key builder using explicit byte append and
  hashing, not hot-path `fmt.Sprintf`.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'CacheKey|CacheProfile'
```

### Task 4: Local CachedProvider Behavior

**Files:**
- Create `jwt/cached_provider.go`
- Create/update `jwt/cached_provider_test.go`

- [ ] Write failing tests for local cache miss, warm hit, `TryParse`, compose
  bypass, TTL clipping by token exp, TTL clipping by key exp, no cache when TTL
  is non-positive, custom clock bypass, and parse failure non-caching.
- [ ] Write failing tests for local `CachedProvider.ParseContext(nil)`,
  already-canceled context, already-expired deadline, and
  `ClearCache(nil/canceled/deadline)`.
- [ ] Prove the local already-done context path performs no `Get`, `Set`,
  `Delete`, cache mutation, or provider delegation.
- [ ] Write a blocked same-key singleflight waiter test proving waiter context
  cancellation returns `context.Canceled` without cache mutation or extra
  provider delegation.
- [ ] Write hit revalidation tests for nil cached reader, algorithm mismatch,
  missing/stale key, key algorithm mismatch, and stale-entry deletion before
  reparse.
- [ ] Implement `CachedProvider` with `Signer`, `Parser`, and `Rotator`
  assertions and doc comments.
- [ ] Implement per-adapter `singleflight.Group` for same-key misses.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'CachedProvider'
```

### Task 5: Distributed CachedProvider Behavior

**Files:**
- Create `jwt/cached_distributed_provider.go`
- Create/update `jwt/cached_distributed_provider_test.go`

- [ ] Write failing tests for distributed miss, warm hit with key revalidation,
  unknown/stale `kid`, forced rotation clearing, delete/reset clearing, and
  process-local clear scope.
- [ ] Write tests proving nil/canceled/deadline contexts prevent cache
  mutation and provider delegation.
- [ ] Implement `CachedDistributedProvider` context-only methods with doc
  comments.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'CachedDistributedProvider'
```

### Task 6: Cache Failure and Cancellation Semantics

**Files:**
- Create `jwt/cache_failure_test.go`
- Create/update `jwt/cached_provider.go`
- Create/update `jwt/cached_distributed_provider.go`

- [ ] Add a fake cache that can fail `Get`, `Set`, `Delete`, and `Clear`.
- [ ] Test only `cache.ErrCacheMiss` falls through to provider parsing.
- [ ] Test non-miss `Get` errors preserve `errors.Is` and block parsing.
- [ ] Test `Set`, stale-entry `Delete`, and rotation/reset `Clear` failures
  are caller-visible.
- [ ] Assert `errors.Is` preservation for `Set`, stale-entry `Delete`,
  `ClearCache`, `ForcedRotate`/`ForcedRotateContext` clear failure, and
  `DeleteKeyChainsContext` clear failure.
- [ ] Use `AsyncJobTester` for cancellation-aware cache operations.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'CacheFailure|Async'
```

### Task 7: Concurrency and Race Gates

**Files:**
- Create `jwt/cache_concurrency_test.go`
- Create/update adapter implementation files

- [ ] Use `GoroutineStressTester` for concurrent parse/cache hits and misses.
- [ ] Test same-token cold burst runs one wrapped provider parse per adapter
  instance.
- [ ] Stress cache hits/misses concurrent with forced rotation, distributed
  delete/reset, stale-key invalidation, cache errors, and failure non-caching.
- [ ] Verify:

```bash
go test -count=1 ./jwt -run 'Cached.*Stress|ColdBurst'
go test -race -count=1 ./jwt ./cache ./testing/concurrency
```

### Task 8: Benchmarks

**Files:**
- Create `jwt/cache_benchmark_test.go`

- [ ] Add uncached parse benchmark.
- [ ] Add cached cold miss and cached warm hit benchmarks.
- [ ] Cover HMAC and RSA.
- [ ] Cover distributed warm hit with key revalidation.
- [ ] Cover miss+`Set`, hot `b.RunParallel`, same-key cold burst, and focused
  key-builder benchmarks.
- [ ] Record `-benchmem` output in review evidence.
- [ ] Record wrapped-provider parse counts and distributed key
  lookup/revalidation counts where practical, especially for warm hit,
  same-key cold burst, and distributed warm-hit benchmarks.
- [ ] Verify:

```bash
go test -run '^$' -bench 'Cached|CacheKey|Parse' -benchmem ./jwt
```

### Task 9: Documentation and Diagram

**Files:**
- Modify `jwt/README.md`
- Modify `jwt/README.ko.md`
- Modify root `README.md` and `README.ko.md` if needed
- Modify `CHANGELOG.md`
- Keep diagram generator/assets current

- [ ] Update JWT selection guide to replace the deferred cache adapter row.
- [ ] Add local cached-provider example.
- [ ] Add distributed cached-provider example.
- [ ] Add compile-checked examples: `ExampleNewCachedProvider` in
  `jwt/jwt_example_test.go` and distributed cached-provider construction in
  `jwt/redis/example_test.go` or an equivalent compile-checked example file.
- [ ] Document trusted process-local cache boundary, unsupported untrusted
  shared/external cache, trust scope, clear scope, `WithParseClock` bypass,
  diagnostics, and auth-framework limitations.
- [ ] Add an operator runbook subsection in both README files covering
  adapter-owned `ForcedRotate`, `ForcedRotateContext`,
  `DeleteKeyChainsContext`, `ClearCache` failure response, process-local clear
  scope, multi-instance cache behavior, unsupported untrusted/shared external
  cache backends, diagnostics/monitoring, and no raw-token logging.
- [ ] Keep EN/KO README parity.
- [ ] Record parity evidence that `jwt/README.md` and `jwt/README.ko.md`
  contain matching coverage for: selection guide row, imports, local cached
  example, distributed cached example, trusted-cache boundary,
  `WithParseClock` bypass, clear scope, diagnostics/operator notes, and
  non-auth-framework caveats.
- [ ] Regenerate diagram:

```bash
node scripts/generate-jwt-provider-cache-adapter-diagram.mjs
```

### Task 10: Final Verification

- [ ] Run focused tests:

```bash
go test -count=1 ./jwt ./cache ./testing/concurrency
go test -count=1 ./jwt ./jwt/redis -run 'Example.*Cached'
```

- [ ] Run repository-wide race gate:

```bash
make race
```

- [ ] If `make race` fails because of an unrelated package, document the
  failure, run targeted fallback race coverage, and keep the failure in review
  evidence:

```bash
go test -race -count=1 ./jwt ./cache ./testing/concurrency
```

- [ ] Run broader repo checks:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
```

- [ ] Run docs/diagram checks:

```bash
node scripts/generate-jwt-provider-cache-adapter-diagram.mjs
git diff --check
rg -n 'CachedProvider|CachedDistributedProvider|ClearCache|ForcedRotate|DeleteKeyChainsContext|process-local|trusted cache|unsupported shared|external cache|diagnostics|monitoring|raw-token' jwt/README.md jwt/README.ko.md
```

## Step 3 DoD

| Item | Status |
|---|---|
| Spec referenced | Done |
| Step 2-R evidence referenced | Done |
| TDD tasks ordered | Done |
| Cache trust and security tasks explicit | Done |
| Concurrency/race gates explicit | Done |
| Benchmark gates explicit | Done |
| README EN/KO parity required | Done |
| Diagram regeneration required | Done |
| Step 3-R review plan defined | Done |
