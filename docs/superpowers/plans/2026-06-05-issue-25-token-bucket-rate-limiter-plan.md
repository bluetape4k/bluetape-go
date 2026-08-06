# Issue 25 Token Bucket Rate Limiter Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: first-party rate limiter package, Redis-backed distributed limiter, Lua scripts, tests, README updates를 포함한다.
- 범위: local token bucket과 Redis distributed token bucket을 같은 caller contract로 제공한다.

## 목표

Go idiom에 맞는 token bucket rate limiter를 구현한다. local path는 low overhead를 유지하고, Redis path는 여러 process 간 quota state를 공유한다.

## 설계 원칙

- 모든 blocking/waiting path는 `context.Context`를 받는다.
- Redis client lifecycle은 caller-owned로 둔다.
- distributed limiter는 Redis Cluster-safe namespace/key layout을 사용한다.
- retry/backoff를 자동으로 숨기지 않고 caller가 wait policy를 선택하게 한다.
- error strings에 token, namespace secret, raw Redis value를 노출하지 않는다.

## 순서

1. #25 research, spec, existing package boundaries를 확인한다.
2. rate, burst, now source, wait policy, reservation semantics를 테스트로 먼저 고정한다.
3. local in-memory token bucket을 구현하고 deterministic clock tests를 작성한다.
4. Redis Lua script 기반 atomic allow/reserve path를 구현한다.
5. cancellation, deadline, Redis deletion, namespace isolation, time skew를 테스트한다.
6. examples와 README locale pair에 local/distributed 사용법과 operational caveats를 기록한다.
7. benchmarks가 문서에 쓰이면 raw output과 해석을 분리한다.

## 리뷰 게이트

- local limiter와 Redis limiter가 같은 high-level contract를 제공하는지 확인한다.
- wait path가 context cancellation을 즉시 반영하는지 확인한다.
- Redis command count가 hot path에서 bounded인지 확인한다.
- clock injection으로 tests가 deterministic한지 확인한다.
- error wrapping과 redaction이 일관적인지 확인한다.

## 검증 게이트

- `go test -count=1 ./ratelimit/...`
- `go test -race -count=1 ./ratelimit/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
