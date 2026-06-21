# Issue #178 Step 6-R Code Review

## Scope

- Branch: `issue-178-money-exchange-rate-providers`
- Implementation scope:
  - `money/provider.go`
  - `money/ecb_provider.go`
  - `money/provider_test.go`
  - `money/ecb_provider_test.go`
  - `money/ecb_provider_concurrency_test.go`
  - `money/money_example_test.go`
  - `money/errors.go`
  - `money/doc.go`
  - `money/README.md`
  - `money/README.ko.md`
  - `README.md`
  - `README.ko.md`
  - `CHANGELOG.md`
  - `scripts/generate-money-exchange-rate-provider-diagram.mjs`
  - `docs/images/readme-diagrams/money-exchange-rate-provider-flow.*`

## Execution Mode

The 7-Tier gate was executed as six independent main-session role lanes plus
one main integration review. Native subagents were intentionally not used for
this issue because the user instructed main-session role fallback after repeated
native subagent stalls.

## Tier 1: Performance

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Refresh stampede | Concurrent stale refresh can issue more than one HTTP refresh under heavy contention because #178 uses a simple mutex-protected snapshot, not singleflight. This is acceptable for the first provider because retry/cache semantics are bounded and there is no background goroutine. | Documented as non-blocking. `GoroutineStressTester` covers concurrent rates and stale refresh correctness. |

Verdict: PASS. P0=0 P1=0.

## Tier 2: Stability

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Nil context tests | Staticcheck flags nil context calls by default, but nil context normalization is an explicit repo convention and issue contract. | Added local `nolint:staticcheck` comments only on contract tests. Runtime code normalizes nil context. |

Verdict: PASS. P0=0 P1=0.

## Tier 3: Security

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Endpoint override | `ECBProviderOptions.Endpoint` allows caller-provided HTTP(S) endpoints for tests and controlled deployments. It is validated for non-empty HTTP(S) scheme, but callers must not treat arbitrary untrusted endpoints as authoritative financial data. | README states ECB reference-rate informational boundary and non-accounting/non-trading scope. No credentials or external provider dependencies added. |

Verdict: PASS. P0=0 P1=0.

## Tier 4: Operator/Ops

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Freshness operations | ECB weekends and TARGET closing days can make `ObservedAt` older than `FetchedAt`. | README documents `ObservedAt`, `FetchedAt`, `ExpiresAt`, `Stale`, `RefreshError`, `CacheTTL`, `MaxStale`, and stale fallback behavior. |

Verdict: PASS. P0=0 P1=0.

## Tier 5: Developer/API

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Public API growth | `money` now exposes provider-backed APIs in addition to value APIs. Misuse risk is controlled only if the boundary stays explicit. | `Convert` remains unchanged. Provider-backed conversion is only through `ConvertWithProvider(ctx, amount, target, provider)` and returns the quote used. |

Verdict: PASS. P0=0 P1=0.

## Tier 6: User/Caller

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Live examples | Live ECB calls would make examples flaky and slow. | Compile-checked example uses a fake provider. README shows construction and usage but tests stay local. |

Verdict: PASS. P0=0 P1=0.

## Main Integration

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P3 | Performance | Refresh stampede is not coalesced. | Accepted for #178. If this becomes hot, add same-key/snapshot refresh coalescing in a follow-up. |
| P3 | Stability | Nil context tests require targeted lint suppression. | Suppression is scoped to the two contract-test lines only. |
| P3 | Security/Ops | Endpoint override and ECB informational boundary must remain clear. | README EN/KO and changelog document the boundary. |
| P3 | User | Fake examples are required to avoid live-network documentation tests. | `ExampleConvertWithProvider` uses a local fake provider. |

## Goroutine Stress Evidence

`money/ecb_provider_concurrency_test.go` includes:

- `TestECBProviderGoroutineStressTesterConcurrentRates`
- `TestECBProviderGoroutineStressTesterConcurrentStaleRefresh`
- `TestECBProviderAsyncJobTesterCancellation`

These run under both normal and race-detector gates.

## Verification Evidence

```bash
node scripts/generate-money-exchange-rate-provider-diagram.mjs
go test -count=1 ./money ./testing/concurrency
go test -race -count=1 ./money ./testing/concurrency
go test -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check
```

Observed results:

- Diagram gate: `nodes=11 routes=10 segments=18 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0 titleGap=58`.
- `go test -count=1 ./money ./testing/concurrency`: pass.
- `go test -race -count=1 ./money ./testing/concurrency`: pass.
- `go test -count=1 ./...`: pass.
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint`: pass with `0 issues`.
- `make test`, `make race`, `make ci`: pass.
- `git diff --check`: pass.

## Gate Verdict

- P0: 0
- P1: 0
- P2: 0
- P3: 4
- Final verdict: PASS. Step 6-R can close.
