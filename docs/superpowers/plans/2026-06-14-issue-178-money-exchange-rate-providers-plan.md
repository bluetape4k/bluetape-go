# Issue #178 Money Exchange Rate Providers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development or superpowers:executing-plans to
> implement this plan task-by-task. Keep task checkboxes updated.
>
> Current session execution note: native subagents are intentionally not used
> for this issue because the user instructed main-session role fallback after
> repeated native subagent stalls. Run the same 7-Tier lanes as independent
> main-session role passes and record the fallback in review artifacts.

**Goal:** Add provider-backed exchange-rate conversion to `money` with an
explicit provider contract, caller-visible quote metadata, cancellation-aware
IO, retry/cache freshness semantics, and a first-party ECB daily reference-rate
provider.

**Architecture:** Keep existing value-only `NewExchangeRate` and `Convert`
unchanged. Add `ExchangeRateProvider`, `ExchangeRateQuote`,
`ConvertWithProvider`, provider-specific sentinel errors, and an `ECBProvider`
that fetches one ECB EUR-base daily XML snapshot, caches the snapshot under a
mutex, computes direct/reverse/cross rates without float math, and exposes
stale fallback failures through `ExchangeRateQuote.RefreshError`.

**Tech Stack:** Go standard library (`context`, `net/http`, `encoding/xml`,
`sync`, `time`), existing `github.com/govalues/money` and
`github.com/govalues/decimal`, `httptest`,
`testing/concurrency.GoroutineStressTester`,
`testing/concurrency.AsyncJobTester`, `$bluetape4k-diagram` README assets.

---

## Source Specification

- Issue: #178
- Spec: `docs/superpowers/specs/2026-06-14-issue-178-money-exchange-rate-providers-design.md`
- Research: `docs/superpowers/research/2026-06-14-issue-178-money-exchange-rate-providers-research.md`
- Step 2-R evidence: `docs/superpowers/reviews/2026-06-14-issue-178-money-exchange-rate-providers-step-2r-spec-review.md`
- Follow-ups: #231 for IMF provider, #232 for Bloomberg provider.
- Branch/worktree: `.worktrees/issue-178-money-exchange-rate-providers`

## File Structure

- Create `money/provider.go`: provider interface, quote value, nil-context
  normalization, typed-nil provider detection, and `ConvertWithProvider`.
- Modify `money/errors.go`: provider sentinel errors.
- Create `money/ecb_provider.go`: `ECBProviderOptions`, defaults,
  validation, `ECBProvider`, cached snapshot, retry loop, HTTP fetch, XML
  parser, and rate computation.
- Create `money/provider_test.go`: contract tests for provider conversion,
  nil/typed-nil providers, invalid currencies, same-currency behavior, and
  provider error propagation.
- Create `money/ecb_provider_test.go`: ECB options, XML, HTTP, cache,
  freshness, retry, cancellation, and unsupported currency tests.
- Create `money/ecb_provider_concurrency_test.go`: `GoroutineStressTester`
  and `AsyncJobTester` coverage.
- Modify `money/money_example_test.go`: compile-checked provider-backed
  conversion example using a fake provider.
- Modify `money/doc.go`: provider-backed exchange-rate package summary.
- Modify `money/README.md` and `money/README.ko.md`: selection guide,
  examples, ECB informational boundary, freshness/failure semantics, and
  follow-up links.
- Modify root `README.md` and `README.ko.md`: package summary no longer says
  only caller-supplied exchange rates.
- Modify `CHANGELOG.md`: `[Unreleased]` entry for ECB provider-backed FX.
- Create `scripts/generate-money-exchange-rate-provider-diagram.mjs`: source
  derived `$bluetape4k-diagram` asset generator following existing diagram
  script validation gates.
- Create generated README diagram assets:
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow.dot`
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow.plain`
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow-graphviz.svg`
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow-graphviz.png`
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow.svg`
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow.png`
- Create Step 3-R review:
  `docs/superpowers/reviews/2026-06-14-issue-178-money-exchange-rate-providers-step-3r-plan-review.md`.
- Create Step 6-R review:
  `docs/superpowers/reviews/2026-06-14-issue-178-money-exchange-rate-providers-step-6r-code-review.md`.
- Create Step 7-R PR review:
  `docs/superpowers/reviews/2026-06-14-issue-178-money-exchange-rate-providers-step-7r-pr-review.md`.

## Step 3-R Plan Review Plan

Before implementation, run the mandatory 7-Tier gate as six independent
main-session role lanes plus main integration:

1. Tier 1 Performance: cache lock contention, cross-rate decimal cost, retry
   overhead, allocation risk, benchmark/stress validation.
2. Tier 2 Stability: cancellation/deadline propagation, retry stop rules,
   HTTP body close, stale fallback semantics, cache race safety.
3. Tier 3 Security: endpoint validation, no hidden credentials, safe default
   client behavior, XML parsing boundary, source metadata integrity.
4. Tier 4 Operator/Ops: freshness/runbook clarity, source diagnostics,
   non-accounting/trading boundary, release readiness.
5. Tier 5 Developer/API: provider contract, zero-value invalidity,
   typed-nil handling, option validation, package fit, test ergonomics.
6. Tier 6 User/Caller: README EN/KO parity, examples, misuse resistance,
   error discoverability, follow-up provider clarity.

Exit condition: integrated table shows `P0=0 P1=0`.

## Step 4-T Implementation Tasks

### Task 1: Provider Contract and Conversion Wrapper

**Files:**
- Create `money/provider.go`
- Modify `money/errors.go`
- Create/update `money/provider_test.go`

- [x] Write failing tests for `ConvertWithProvider` success returning both
  converted money and the exact quote used.
- [x] Write failing tests for nil provider and typed-nil provider returning
  `ErrExchangeRateProvider`.
- [x] Write failing tests proving nil context is normalized to
  `context.Background()` and does not panic.
- [x] Write failing tests for provider errors preserving `errors.Is` and not
  returning a valid `Money`.
- [x] Write failing tests proving a provider quote with invalid
  `ExchangeRate` returns `ErrInvalidExchangeRate` and no valid `Money`.
- [x] Write failing tests for invalid amount currency and invalid target
  currency returning existing money/currency sentinels.
- [x] Add `ExchangeRateProvider`, `ExchangeRateQuote`, and
  `ConvertWithProvider`.
- [x] Add provider sentinels:
  `ErrExchangeRateProvider`, `ErrExchangeRateUnavailable`,
  `ErrExchangeRateStale`, and `ErrUnsupportedExchangeRate`.
- [x] Add Go doc comments for every exported type/function/field where
  required by Go lint.
- [x] Verify:

```bash
go test -count=1 ./money -run 'ConvertWithProvider|ExchangeRateProvider'
```

### Task 2: ECB Options and Constructor Validation

**Files:**
- Create `money/ecb_provider.go`
- Create/update `money/ecb_provider_test.go`

- [x] Write failing tests for default options: endpoint is ECB daily XML,
  timeout/cache TTL/retry defaults are positive and documented, and no
  background goroutine is started.
- [x] Write failing tests for negative `Timeout`, `CacheTTL`, `MaxStale`,
  `RetryCount`, and `RetryBackoff` returning `ErrExchangeRateProvider`.
- [x] Write failing tests for empty endpoint and non-HTTP(S) endpoint scheme
  returning `ErrExchangeRateProvider`.
- [x] Write tests proving nil `Client` and nil `Now` use safe defaults.
- [x] Implement `ECBProviderOptions`, `ECBProvider`, default constants,
  constructor validation, nil context normalization, and endpoint parsing.
- [x] Keep `RetryCount` semantics explicit: it excludes the first attempt.
- [x] Verify:

```bash
go test -count=1 ./money -run 'ECB.*Option|NewECBProvider'
```

### Task 3: ECB XML Fetch and Parse

**Files:**
- Update `money/ecb_provider.go`
- Create/update `money/ecb_provider_test.go`

- [x] Write failing `httptest.Server` success tests for ECB daily XML with
  `Cube time='YYYY-MM-DD'` and rate entries.
- [x] Write failing tests for HTTP non-2xx status wrapping
  `ErrExchangeRateProvider` and closing the response body.
- [x] Write failing tests for malformed XML, missing observation date, missing
  rates, duplicate currencies, invalid currency code, and malformed rate
  value.
- [x] Write failing tests proving caller `context.Canceled` and
  `context.DeadlineExceeded` are preserved with `errors.Is`.
- [x] Write failing tests proving provider `Timeout` expires a slow server
  request and never weakens a stricter caller deadline.
- [x] Implement fetch with `http.NewRequestWithContext`, per-fetch timeout that
  never weakens stricter caller deadlines, response body close, and XML decode.
- [x] Parse ECB observation date as UTC midnight for `ObservedAt`; set
  `FetchedAt` from `Now`.
- [x] Store snapshot source as `ECB`.
- [x] Verify:

```bash
go test -count=1 ./money -run 'ECB.*Fetch|ECB.*Parse|ECB.*HTTP|ECB.*Context'
```

### Task 4: ECB Rate Computation

**Files:**
- Update `money/ecb_provider.go`
- Create/update `money/ecb_provider_test.go`

- [x] Write failing tests for same-currency rate `1` without HTTP fetch.
- [x] Write failing tests for EUR to quote currency, quote currency to EUR,
  and non-EUR cross rate such as USD to KRW.
- [x] Write failing tests for absent snapshot currency returning
  `ErrUnsupportedExchangeRate`.
- [x] Write failing tests for invalid `Currency{}` base/target returning
  `ErrInvalidCurrency`.
- [x] Implement ECB direct/reverse/cross computation using decimal values and
  existing `NewExchangeRate`; do not use `float32` or `float64`.
- [x] Ensure quote metadata is preserved for direct, reverse, and cross rates.
- [x] Verify:

```bash
go test -count=1 ./money -run 'ECB.*Rate|ECB.*Cross|UnsupportedExchangeRate'
```

### Task 5: Cache, Freshness, Stale Fallback, and Retry

**Files:**
- Update `money/ecb_provider.go`
- Create/update `money/ecb_provider_test.go`

- [x] Write failing tests proving fresh cache hit avoids a second HTTP request.
- [x] Write failing tests proving stale cache with
  `AllowStaleOnError=false` returns `ErrExchangeRateStale` when refresh fails.
- [x] Write failing tests proving stale cache with `AllowStaleOnError=true`
  returns quote with `Stale=true` and non-nil `RefreshError` preserving
  `errors.Is`.
- [x] Write failing tests proving snapshots older than `MaxStale` are not
  returned even when `AllowStaleOnError=true`.
- [x] Write failing tests for cache miss with no successful fetch returning
  `ErrExchangeRateUnavailable` when appropriate.
- [x] Write failing tests for retry success after transient 500 or network
  failure.
- [x] Write failing tests proving retry does not repeat after
  `context.Canceled` or caller deadline.
- [x] Implement snapshot cache guarded by `sync.RWMutex`.
- [x] Implement retry loop with bounded backoff and context-aware sleep.
- [x] Keep all refresh IO caller-driven from `Rate`; do not add background
  refresh goroutines.
- [x] Verify:

```bash
go test -count=1 ./money -run 'ECB.*Cache|ECB.*Stale|ECB.*Retry'
```

### Task 6: Concurrency and Cancellation Stress

**Files:**
- Create/update `money/ecb_provider_concurrency_test.go`
- Update `money/ecb_provider.go` if races or cancellation gaps are found.

- [x] Add `GoroutineStressTester` coverage with concurrent `Rate` calls across
  EUR/USD, USD/EUR, USD/KRW, KRW/USD, and same-currency paths.
- [x] Add `GoroutineStressTester` coverage for concurrent stale refresh where
  only valid snapshots are published to readers.
- [x] Add `AsyncJobTester` coverage for already-canceled and deadline contexts.
- [x] Prove `go test -race` passes for `./money ./testing/concurrency`.
- [x] Verify:

```bash
go test -count=1 ./money ./testing/concurrency -run 'ECB.*Concurrent|ECB.*Async|GoroutineStress|AsyncJob'
go test -race -count=1 ./money ./testing/concurrency
```

### Task 7: Examples and Public Documentation

**Files:**
- Modify `money/money_example_test.go`
- Modify `money/doc.go`
- Modify `money/README.md`
- Modify `money/README.ko.md`
- Modify `README.md`
- Modify `README.ko.md`
- Modify `CHANGELOG.md`

- [x] Add compile-checked `ExampleConvertWithProvider` with a fake in-memory
  provider, not live ECB network.
- [x] Add README examples for `NewECBProvider`, `ConvertWithProvider`, and
  stale fallback handling.
- [x] Update EN/KO selection guides to replace "provider-backed fetching is
  deferred to #178" with current ECB provider status.
- [x] Document ECB reference-rate source, TARGET/weekend freshness caveat,
  informational-only boundary, and non-accounting/non-trading/non-tax scope.
- [x] Document timeout, retry, stale fallback, source, `ObservedAt`,
  `FetchedAt`, `ExpiresAt`, `Stale`, and `RefreshError`.
- [x] Link #231 and #232 as follow-up provider expansions.
- [x] Keep Korean README natural and semantically equivalent to English.
- [x] Verify examples compile:

```bash
go test -count=1 ./money -run 'Example|ConvertWithProvider'
```

### Task 8: README Diagram Asset

**Files:**
- Create `scripts/generate-money-exchange-rate-provider-diagram.mjs`
- Create generated files under `docs/images/readme-diagrams/`
- Modify `money/README.md`
- Modify `money/README.ko.md`

- [x] Build the diagram source from actual package boundaries:
  caller, `ConvertWithProvider`, `ExchangeRateProvider`, `ECBProvider`,
  snapshot cache, ECB daily XML, `Convert`, and failure/stale paths.
- [x] Generate DOT, Graphviz SVG/PNG, and final hand-authored SVG/PNG assets
  following the existing diagram script pattern.
- [x] Include geometry validation: no node overlap, orthogonal route checks,
  endpoint checks, enough lane/card clearance, balanced margins, and title gap.
- [x] Include text validation: no draft filler words, no raw Mermaid, no
  "Graphviz only" artifact text, and no default web fonts.
- [x] Render PNG with `rsvg-convert` and validate SVG with `xmllint`.
- [x] Add diagram references to both money README files with matching EN/KO
  alt text.
- [x] Verify:

```bash
node scripts/generate-money-exchange-rate-provider-diagram.mjs
git diff --check
```

### Task 9: Full Local Verification

**Files:** all changed files.

- [x] Run targeted tests:

```bash
go test -count=1 ./money ./testing/concurrency
```

- [x] Run race tests:

```bash
go test -race -count=1 ./money ./testing/concurrency
```

- [x] Run repository tests:

```bash
go test -count=1 ./...
```

- [x] Run standard project gates:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check
```

- [x] If a repo-wide gate fails outside this issue's scope, capture the exact
  command and failure in the review artifact and use the smallest targeted
  passing evidence that proves #178.

### Task 10: Step 6-R Code Review

**Files:**
- Create `docs/superpowers/reviews/2026-06-14-issue-178-money-exchange-rate-providers-step-6r-code-review.md`

- [x] Run the 7-Tier gate as six independent main-session role lanes plus main
  integration.
- [x] Tier 1 Performance: lock contention, decimal/cross-rate path, retry/backoff,
  allocation hot spots, stress/race evidence.
- [x] Tier 2 Stability: cancellation, deadline, body close, stale fallback,
  retry stop rules, malformed payloads, race evidence.
- [x] Tier 3 Security: endpoint validation, XML parsing, credential-free design,
  source metadata integrity, dependency surface.
- [x] Tier 4 Operator/Ops: freshness docs, diagnostics, changelog, rollback,
  non-accounting/trading runbook language.
- [x] Tier 5 Developer/API: Go docs, typed-nil handling, option validation,
  module boundaries, examples, compatibility.
- [x] Tier 6 User/Caller: README EN/KO parity, misuse resistance, error
  discoverability, follow-up links.
- [x] Fix every P0/P1 and rerun affected lanes until integrated verdict is
  `P0=0 P1=0`.

### Task 11: PR Preparation and Step 7-R

**Files:**
- Create `docs/superpowers/reviews/2026-06-14-issue-178-money-exchange-rate-providers-step-7r-pr-review.md`

- [x] Ensure research, spec, plan, Step 2-R, Step 3-R, Step 6-R, code, tests,
  docs, diagram assets, and changelog are committed with Lore commit messages.
- [x] Create PR with `--body-file`; verify live body with
  `gh pr view <number> --json body`.
- [x] Keep the last `##` section in the PR body as `## DoD Status`.
- [x] Run Step 7-R with the same 7-Tier main-role fallback.
- [x] Do not merge until the user explicitly approves the PR merge.

## Completion Criteria

- `P0=0 P1=0` for Step 3-R, Step 6-R, and Step 7-R.
- `go test -count=1 ./money ./testing/concurrency` passes.
- `go test -race -count=1 ./money ./testing/concurrency` passes.
- `go test -count=1 ./...` passes or an unrelated pre-existing failure is
  documented with targeted #178 evidence.
- `make ci` passes or an unrelated pre-existing failure is documented with
  targeted #178 evidence.
- `git diff --check` passes.
- `money/README.md`, `money/README.ko.md`, `README.md`, `README.ko.md`, and
  `CHANGELOG.md` reflect the new behavior.
- README diagram assets are generated by the script and visually/geometry
  validated.
- PR body references #178 and follow-up issues #231/#232, and records DoD
  evidence.
