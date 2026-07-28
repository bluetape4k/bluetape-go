# Issue 36 Probabilistic Bloom Filter Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: probabilistic package, Bloom filter implementation, false-positive contract, tests, benchmarks, docs를 포함한다.
- 범위: in-memory Bloom filter foundation을 구현하고 Redis-backed filter(#182)의 shared boundary를 준비한다.

## 목표

메모리 효율적인 membership check를 위한 first-party Bloom filter를 제공한다. false negative가 없어야 하며, false positive probability와 capacity 설정을 명확히 문서화한다.

## 설계 원칙

- `probabilistic.Config`로 expected insertions와 false-positive probability를 검증한다.
- `Hasher[T]`를 통해 caller-defined value encoding을 허용한다.
- SHA-256 double-hash 기반 index 계산은 deterministic해야 한다.
- thread-safety 보장 범위를 문서와 테스트에 맞춘다.
- Redis-backed 확장을 위해 hash/index boundary를 공유 가능하게 유지한다.

## 순서

1. #36 research와 Bloom filter 수식/parameter contract를 확인한다.
2. config validation, bitset size, hash count, hasher contract를 spec에 고정한다.
3. config boundary, no-false-negative, deterministic hash tests를 먼저 작성한다.
4. in-memory Bloom filter와 hasher helpers를 구현한다.
5. examples, README locale pair, benchmark를 추가한다.
6. #182 Redis filter가 재사용할 shared index helper 필요성을 follow-up으로 기록한다.

## 리뷰 게이트

- false-positive probability를 정확성 보장처럼 표현하지 않는지 확인한다.
- invalid config가 조기에 실패하는지 확인한다.
- hasher error와 empty key가 명확히 처리되는지 확인한다.
- concurrency-safety 문서가 실제 구현과 일치하는지 확인한다.
- benchmark가 allocation/hot path를 실제로 측정하는지 확인한다.

## 검증 게이트

- `go test -count=1 ./probabilistic/...`
- `go test -race -count=1 ./probabilistic/...`
- `go test -run '^$' -bench . ./probabilistic/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
