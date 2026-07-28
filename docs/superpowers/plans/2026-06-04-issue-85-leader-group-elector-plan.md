# Issue 85 Leader Group Elector Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: distributed coordination API, Redis-backed leader election, integration tests, documentation을 포함한다.
- 범위: 단일 leader가 아니라 group별 leader election을 지원한다.

## 목표

여러 service instance가 Redis를 통해 group-scoped leader를 선출하고 lease 갱신/포기를 수행할 수 있게 한다. API는 Kubernetes leader election을 그대로 복제하지 않고 Go package의 작은 계약으로 유지한다.

## 순서

1. existing coordination primitives와 #85 issue contract를 확인한다.
2. group key, candidate id, lease TTL, renew interval, clock source를 spec에 고정한다.
3. acquisition, renewal, leadership loss, resignation 테스트를 먼저 작성한다.
4. Redis atomic operation 또는 Lua script로 group-scoped leadership state를 구현한다.
5. cancellation, Redis outage, clock drift, stale leader 복구를 테스트한다.
6. examples와 README에 operational caveats를 기록한다.

## 리뷰 게이트

- group namespace가 충돌 없이 구성되는지 확인한다.
- leadership loss가 caller에게 관찰 가능한지 확인한다.
- renew loop가 context cancellation으로 종료되는지 확인한다.
- Redis key/value가 redacted error contract를 지키는지 확인한다.
- Testcontainers 테스트가 deterministic한지 확인한다.

## 검증 게이트

- `go test -count=1 ./leader/...`
- `go test -race -count=1 ./leader/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
