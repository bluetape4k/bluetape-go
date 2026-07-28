# Issue #174 JWT Compression and JOSE Scope Research Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

Task: Type E research review
이슈: #174
날짜: 2026-06-12
범위: `docs/superpowers/research/2026-06-12-issue-174-jwt-compression-jose-scope.md`,
`jwt/README.md`, `jwt/README.ko.md`, `CHANGELOG.md`, `WIP.md`, and lesson
capture.

## 통합 판정

PASS.

P0=0 P1=0

The research decision is source-grounded and preserves the existing fail-closed
JWT behavior. The current package remains a signed JWT helper, and compression is
not added outside a future explicit JWE boundary.

## 검토 관점

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Standards/security | 0 | 0 | 0 | 0 | PASS | RFC 7516/7518 place `zip=DEF` in the JWE compression boundary; signed JWT `zip` remains rejected. |
| Dependency | 0 | 0 | 0 | 0 | PASS | `golang-jwt/jwt/v5` stays current dependency; `go-jose/go-jose/v4` is documented only as future JWE candidate; `jwx/v4` is rejected for default scope. |
| Docs/user | 0 | 0 | 0 | 0 | PASS | README pair explicitly states signed JWT compression is a non-goal and future compression belongs to an optional JWE API. |
| Workflow/evidence | 0 | 0 | 0 | 0 | PASS | Type E scope has no production code changes; research and lesson artifacts are included. |

## 계층 요약

| Tier | Result | Notes |
|---|---|---|
| 1 Security | PASS | Rejects header confusion, remote key headers, and signed JWT compression. Future JWE risks are listed as acceptance gates. |
| 2 Ops/SRE reliability | PASS | Avoids `GOEXPERIMENT=jsonv2` dependency burden in normal builds. Future JWE parsing limits are documented. |
| 3 Structural impact | PASS | No public API or runtime dependency changes. |
| 4 Go code quality | PASS | No Go code changes. Existing `jwt` package fail-closed behavior is preserved. |
| 5 Tests/types/silent failure | PASS | Existing tests remain applicable; future JWE tests are specified before implementation. |
| 6 Performance/stability | PASS | Decompression and compact parsing resource limits are required before any JWE feature. |
| 7 Docs/release/evidence | PASS | Research, README pair, WIP, CHANGELOG, and lesson capture are synchronized. |

## 검증 증거

```bash
go test -count=1 ./jwt
git diff --check
rg -n "signed JWT compression|JWE|zip|go-jose|jwx|#174" jwt/README.md jwt/README.ko.md docs/superpowers/research/2026-06-12-issue-174-jwt-compression-jose-scope.md CHANGELOG.md WIP.md
```

## 잔여 위험

No implementation risk remains for #174 because no production behavior changed.
The next risk point is a future JWE issue; it must not reuse the signed JWT
parser path or skip decompression and header-policy gates.
