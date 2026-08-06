# Issue #232 Bloomberg Provider Step 6-R Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

범위: Bloomberg-backed exchange-rate provider evaluation, README pair,
research note, WIP, changelog, and research index updates.

Baseline: `develop` at `a76d396`.

## 게이트 결과

P0=0 P1=0

Final verdict: PASS.

## 관점별 결과

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance/runtime | 0 | 0 | 0 | 0 | PASS | No runtime provider implementation was added. The future B-PIPE path is constrained to caller-owned snapshot/cache infrastructure instead of a default request-time feed. |
| Stability/correctness | 0 | 0 | 0 | 0 | PASS | The spec keeps `Rate(ctx, base, target)`, `ConvertWithProvider`, source metadata, freshness, stale fallback, and sentinel error mapping aligned with the desired IMF/current context contract. |
| Security | 0 | 0 | 0 | 0 | PASS | Bloomberg credentials, entitlements, mutual TLS/network setup, SDK access, usage monitoring, and diagnostics are documented as customer-owned concerns. No secret or credential path was added. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | SAPI, B-PIPE, and Data License are differentiated by deployment topology, monitoring, entitlement ownership, and freshness semantics. Default CI remains free of Bloomberg dependencies. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | The decision rejects a default `money.NewBloombergProvider` and requires any future implementation to live behind the existing `ExchangeRateProvider` contract with fake-backed tests. |
| User/caller docs | 0 | 0 | 0 | 0 | PASS | The README pair now states Bloomberg-backed rates require customer-owned Bloomberg access, entitlements, and deployment topology and are not part of default `money` behavior. |

## 발견 사항

P0/P1 발견 사항 없음.

Repaired P2: narrowed the stability lane wording from "existing ECB/IMF
contract" to "desired IMF/current context contract" because the future
Bloomberg adapter must not serve stale data for caller-owned context failures.
The ECB provider should be reviewed separately before treating that behavior as
a shared provider invariant.

The main risk was documenting a Bloomberg provider as if it were a default
public data source. The research note and README wording now avoid that by
requiring licensed customer infrastructure and by keeping default tests free of
Bloomberg SDKs, credentials, and paid access.

## 검증

| Command / Review | Status | Evidence |
|---|---|
| `git diff --check` | PASS | Whitespace check clean. |
| `rg -n "Bloomberg|customer-owned|Data License|B-PIPE|SAPI" money/README.md money/README.ko.md docs/research/2026-06-21-issue-232-bloomberg-exchange-rate-provider.md` | PASS | README pair and research note contain the intended decision wording. |
| `go test -count=1 ./money` | PASS | Existing money package tests remain green after docs-only change. |
| `make ci` | PASS | Full repo validation completed successfully, including Testcontainers-backed packages. |
| Step 6-R 7-tier lanes | PASS | Performance, security, operator/Ops, developer/API, dependency/licensing P0=0 P1=0; stability P0=0 P1=0 after P2 wording repair. |

## 잔여 위험

The exact Bloomberg security mnemonic, field mnemonic, pricing source, and
entitlement behavior can only be finalized by a licensed Bloomberg customer
environment. That is intentionally deferred to a future optional adapter issue.
