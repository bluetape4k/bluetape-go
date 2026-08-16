# Issue #545 검증 기록

## 범위

- 대상: `jwt/jwks` optional JWKS provider
- 브랜치: `feat/jwks-provider-545`
- 기준: `origin/develop`의 `9cad5a3d330b1da335e26170d3b97684af0cf44d`
- 설계/계획: `docs/superpowers/specs/2026-08-16-issue-545-jwks-provider-design.md`,
  `docs/superpowers/plans/2026-08-16-issue-545-jwks-provider-plan.md`
- 의도적으로 하지 않은 작업: release promotion, tag, GitHub merge

## 기능 검증

`go test ./jwt/jwks -count=1` 통과.

- RSA RS/PS, ECDSA P-256, Ed25519 EdDSA `golang-jwt/v5` end-to-end 서명 검증
- `KeyFunc`의 nil/context/header/allowlist 경계와 `zip`, `crit`, `jku`, `jwk`,
  `x5u`, `x5c` 거부
- issuer/audience/expiration 정책을 parser option에 위임하는 claims boundary
- RSA exponent, curve/algorithm, Ed25519 raw length, private/symmetric key,
  `key_ops`, `kid`, `use`, duplicate/count 제한
- direct JWKS URL, redirect, status, body cap, context timeout/cancel 및
  redacted typed error
- TTL/cache hit/rotation, unknown `kid` cooldown, explicit refresh,
  single-flight, waiter cancellation, leader takeover, late-result suppression,
  warm lookup non-blocking, defensive public-key copy
- context-aware custom transport cancellation worker cleanup
- bounded rollback drill: blocked endpoint, TTL fail-closed, endpoint restore,
  readiness refresh, known-token verification, overlap-key retirement

## 저장소 게이트

| 명령 | 결과 |
|---|---|
| `go test ./...` | PASS |
| `make test` | PASS |
| `go test -race ./jwt/jwks -count=1` | PASS |
| `go vet ./jwt/jwks` | PASS |
| `make fmt-check` | PASS |
| `make lint` | PASS (0 issues) |
| `go mod verify` | PASS (`all modules verified`) |
| `govulncheck ./jwt/jwks` | 실행 불가: `govulncheck` 미설치 |
| `make ci` (커밋 전) | `tidy-check`가 의도한 신규 go.mod/go.sum diff를 감지하여 중단 |
| `make ci` (커밋 후) | PASS: tidy/fmt/vet/lint/test/race 및 benchmark contract self-test |

`make tidy-check`의 정규화 결과로 `github.com/go-jose/go-jose/v4 v4.1.4`가
직접 dependency block에 위치한다. 이 차이는 신규 package의 실제 import를
반영한 것이며 커밋 후 clean tree에서 다시 확인한다.

## 모듈/라이선스 evidence

```text
go version go1.26.6 darwin/arm64
GOOS=darwin
GOARCH=arm64
go-jose: github.com/go-jose/go-jose/v4 v4.1.4
sum: h1:moDMcTHmvE6Groj34emNPLs/qtYXRVcd6S7NHbHz3kA=
go mod graph edge: github.com/bluetape4k/bluetape-go github.com/go-jose/go-jose/v4@v4.1.4
license: /Users/debop/work/go/pkg/mod/github.com/go-jose/go-jose/v4@v4.1.4/LICENSE
```

기존 graph에는 `google.golang.org/grpc@v1.79.3`의 v4.1.3 edge도 있어 이번
변경은 root module의 직접 요구를 v4.1.4로 고정한다. license 파일은 위 경로에서
확인했다.

## Benchmark raw output

실행 환경: Apple M5, `darwin/arm64`, `go1.26.6`.

```text
BenchmarkLookupCacheHit-10          20       172.9 ns/op       0 http-requests/op     336 B/op   3 allocs/op
BenchmarkLookupParallelHit-10       20       531.2 ns/op       0 http-requests/op     570 B/op   4 allocs/op
BenchmarkLookupForcedRefresh-10     20     95125 ns/op       1.000 http-requests/op 11424 B/op 134 allocs/op
BenchmarkLookupColdMiss-10          20     99965 ns/op       1.000 http-requests/op 13645 B/op 144 allocs/op
BenchmarkLookupTTLExpiry-10         20     92300 ns/op       1.000 http-requests/op 11775 B/op 137 allocs/op
BenchmarkLookupParallelMiss-10      20     17779 ns/op       0.1000 http-requests/op 3627 B/op 24 allocs/op
```

benchmark HTTP metric은 warm-up 이후 timed 구간의 `http-requests/op`로
정규화한다. cache hit와 parallel hit는 warm snapshot에서 추가 HTTP request를
발생시키지 않으며, TTL expiry와 forced refresh는 iteration당 bounded refresh
1회를 검증한다. 전체 rollback drill은 초기 readiness 1회, 만료 실패 1회,
복원 readiness 1회, overlap retirement 1회로 총 4회 요청을 확인한다.

## 남은 gap

- `govulncheck`는 실행 파일이 없어 보안 취약점 스캔 결과를 확보하지 못했다.
- GitHub PR/CI 확인은 PR 생성 후 수행한다.
- release tag/publication은 이번 issue 범위가 아니며 별도 승인 없이 수행하지 않는다.
