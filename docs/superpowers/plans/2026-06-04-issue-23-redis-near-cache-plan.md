# Issue 23 Redis Near Cache Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: Redis-backed cache package, local near-cache coordination, Testcontainers integration tests, docs/examples를 포함한다.
- 범위: #22 cache interface 위에 Redis 저장소와 local near-cache를 구현한다.

## 목표

Redis를 공유 저장소로 사용하면서 process-local near-cache로 hot read latency를 줄인다. correctness는 Redis state를 기준으로 하고, near-cache는 TTL과 invalidation 정책으로 제한한다.

## 순서

1. #22 cache interface와 error contract를 확인한다.
2. caller-owned `redis.Cmdable`와 namespace option을 사용하는 Redis cache options를 설계한다.
3. key derivation, serialization boundary, TTL semantics를 테스트로 고정한다.
4. Redis get/set/delete/clear path와 local near-cache lookup path를 구현한다.
5. near-cache stale window, invalidation, Redis deletion, context cancellation을 테스트한다.
6. Testcontainers Redis integration tests를 추가하고 shared resource 충돌을 피하도록 순차 실행을 문서화한다.
7. README/README.ko.md에 operational caveats, key layout, lifecycle ownership을 반영한다.

## 리뷰 게이트

- Redis client lifecycle을 package가 소유하지 않는지 확인한다.
- local near-cache가 source-of-truth처럼 동작하지 않는지 확인한다.
- serialization error와 Redis error가 redacted/wrapped되는지 확인한다.
- TTL과 namespace validation이 production misuse를 줄이는지 확인한다.
- `context.Context` cancellation이 Redis call에 전달되는지 확인한다.

## 검증 게이트

- `go test -count=1 ./cache/...`
- `go test -race -count=1 ./cache/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
