# Step 6-R Review: Issue 224 Integration Recipes

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #224
브랜치: issue-224-integration-recipes
날짜: 2026-06-24

Subagent lanes were not used due to current subagent/runtime instability; main
integration fallback performed with required lane separation.

## 성능 관점

P0: 0
P1: 0

- Service-free recipes run in-process and avoid sleeps.
- Redis smoke is env-gated and uses a single Testcontainers Redis instance.
- Retry examples use `resilience.NoBackoff()` and bounded `MaxAttempts`.

## 안정성 관점

P0: 0
P1: 0

- Every runnable recipe uses a timeout context.
- Redis lock, leadership, client, and container cleanup are registered with
  `t.Cleanup`.
- Batch failure paths cover temporary write retry and invalid-item skip.

## 보안 관점

P0: 0
P1: 0

- JWT examples use explicit HS256 and a 32-byte HMAC secret.
- Parse examples require expected subject, audience, and expiration where the
  recipe signs an access token.
- Redis lock cleanup only unlocks with the owner token.

## 운영 관점

P0: 0
P1: 0

- Docker-backed Redis smoke is opt-in with
  `BLUETAPE_INTEGRATION_RECIPE_SMOKE=1`.
- README documents serial execution for Testcontainers-backed packages.
- Package docs describe cleanup, timeouts, and failure-path behavior.

## 개발자/API 관점

P0: 0
P1: 0

- Recipes use existing public APIs only.
- No helper abstraction was promoted from example code into a library package.
- Root README and README.ko.md link the new example package.

## 사용자/호출자 관점

P0: 0
P1: 0

- The package gives callers copyable commands for service-free tests, smoke
  tests, and race tests.
- README links point to maintained package-level docs.

## 메인 통합 판정

P0: 0
P1: 0

The change is scoped to examples and documentation. It should proceed if the
example package, Redis smoke, and standard repository gates pass.

## 검증 증거

- PASS `go test -count=1 ./examples/integration`
- PASS `BLUETAPE_INTEGRATION_RECIPE_SMOKE=1 go test -p 1 -count=1 ./examples/integration`
- PASS `go test -race -count=1 ./examples/integration`
- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `make test`
- PASS `make race`
