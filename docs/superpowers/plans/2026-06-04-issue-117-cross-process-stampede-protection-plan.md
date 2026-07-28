# Issue 117 Cross-Process Stampede Protection Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: Redis coordination, cache loader concurrency, public behavior contract, integration tests를 포함한다.
- 범위: 여러 Go process가 같은 cache miss를 동시에 처리할 때 loader stampede를 줄이는 first-party 보호 계층을 구현한다.

## 목표

Redis-backed coordination을 사용해 cross-process cache stampede를 완화한다. 단일 process용 `singleflight` 대체가 아니라, distributed lock/lease와 stale value 정책을 조합해 loader 중복 실행을 줄이는 것이 목표다.

## 설계 원칙

- caller-owned `context.Context` cancellation과 deadline을 존중한다.
- Redis key namespace와 TTL을 명확히 하고, key leakage를 error/log에 노출하지 않는다.
- lock holder가 실패해도 lease 만료 뒤 다른 process가 복구할 수 있어야 한다.
- stale-while-revalidate와 negative-cache 동작은 명시적으로 문서화한다.
- Redis outage 시 fallback behavior를 API 계약으로 고정한다.

## 순서

1. #22 cache interface와 #23 Redis near-cache 상태를 다시 확인한다.
2. stampede protection spec에서 lock key, value key, lease TTL, wait policy를 고정한다.
3. loader 중복 실행과 stale value 반환 조건을 테스트로 먼저 작성한다.
4. Redis Lua 또는 atomic command 조합으로 lease acquisition/release를 구현한다.
5. timeout, cancellation, Redis deletion, loader error, panic recovery 경로를 검증한다.
6. Testcontainers 기반 multi-client integration test를 추가한다.
7. README/README.ko.md에 operational caveats와 safe Redis inspection 예시를 추가한다.
8. benchmark가 필요한 경우 raw output을 별도 보존하고 해석은 문서에 한국어로 기록한다.

## 리뷰 게이트

- lock lease가 영구 고착되지 않는지 확인한다.
- context cancellation이 wait/acquire/load 전 구간에 적용되는지 확인한다.
- Redis command count와 round-trip 수가 과도하지 않은지 확인한다.
- stale value 반환이 caller에게 명확히 관찰 가능한지 확인한다.
- error wrapping이 `%w`와 sentinel/typed error 계약을 지키는지 확인한다.
- Testcontainers 테스트가 공유 Docker 자원에서 순차 실행 가능한지 확인한다.

## 검증 게이트

- `go test -count=1 ./cache/...`
- `go test -race -count=1 ./cache/...`
- Redis/Testcontainers package는 필요 시 sequential execution으로 실행한다.
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
