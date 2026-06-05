# Lessons Learned — Redis NearCache Failure Injection (2026-06-04)

**Related issue**: #116
**Impact modules**: `cache/redisnear`

## L1: 장애 주입 테스트는 운영 계약을 좁혀야 한다

### 문제

#23의 기존 테스트는 synthetic receive error와 정상 peer invalidation을 증명했지만,
Redis process가 실제로 사라지는 상황과 이후 복구 경로를 별도로 고정하지 않았다.

### 교훈

Pub/Sub 기반 NearCache는 outage에서 local cache를 비워 stale read 위험을 낮추는
것과, terminal failure 뒤 새 Redis client로 NearCache를 recreate해야 한다는 운영
계약을 분리해서 테스트와 README에 함께 남겨야 한다.

### Evidence

- Added `TestNearCacheClearsLocalOnRedisOutage`.
- Added `TestNearCacheRecreateAfterRedisOutageRestoresPeerInvalidation`.
- Updated `README.md` and `README.ko.md` with recreate guidance after terminal
  subscriber failure or Redis restart.

## L2: Testcontainers 장애 테스트는 직렬 검증으로 다룬다

### 문제

Redis container 종료/재생성은 Docker runtime 상태와 Pub/Sub receive loop timing에
민감하다. 병렬 테스트 실행은 같은 기능을 검증해도 flake 원인을 키울 수 있다.

### 교훈

Redis NearCache failure-injection은 package 단위 직렬 `go test`와 race run으로
검증하고, broader CI는 같은 invocation 안에서 순차 실행한다.

### Evidence

- `go test -count=1 ./cache/redisnear -run "RedisOutage|RecreateAfterRedisOutage"`
  passed before broad validation.
