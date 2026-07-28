# Issue 24 Redis Distributed Lock Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: Redis coordination primitive, public API, Lua/atomic behavior, Testcontainers tests, docs를 포함한다.
- 범위: Go 서비스 간 짧은 critical section 보호를 위한 Redis-backed lock을 구현한다.

## 목표

caller가 명시적으로 lease TTL과 context deadline을 선택하는 Redis distributed lock을 제공한다. 구현은 Redisson compatibility를 주장하지 않고, first-party Go contract로 fencing, ownership token, release safety를 문서화한다.

## 순서

1. lock research와 #24 issue contract를 확인한다.
2. namespace, lock key, owner token, lease TTL, retry/wait policy를 spec에 고정한다.
3. acquire/release/extend 실패 테스트를 먼저 작성한다.
4. Redis atomic command 또는 Lua script로 ownership-safe acquire/release를 구현한다.
5. cancellation, expired lease, wrong-owner release, Redis deletion을 테스트한다.
6. examples와 README에 misuse caveats와 safe operational checks를 추가한다.
7. 필요한 경우 benchmark evidence는 raw output으로 분리한다.

## 리뷰 게이트

- lock이 영구 고착될 수 있는 경로가 없는지 확인한다.
- release가 owner token을 검증하는지 확인한다.
- context cancellation과 retry wait가 일관적인지 확인한다.
- key names와 token values가 error/log에 노출되지 않는지 확인한다.
- Redis Cluster hash-tag 요구사항이 문서화되어 있는지 확인한다.

## 검증 게이트

- `go test -count=1 ./coordination/...`
- `go test -race -count=1 ./coordination/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
