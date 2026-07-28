# Redis NearCache Failure Injection Lessons (2026-06-04)

**Related issue**: #116
**Impact modules**: `cache/redisnear`

## L1: 장애 주입 test는 운영 contract를 좁혀야 한다

### 문제

#23의 기존 test는 synthetic receive error와 정상 peer invalidation을 증명했지만,
Redis process가 실제로 사라지는 상황과 이후 recovery path를 별도로 고정하지 않았다.

### 교훈

Pub/Sub 기반 NearCache는 outage에서 local cache를 비워 stale read 위험을 낮추는
contract와, terminal failure 뒤 새 Redis client로 NearCache를 recreate해야 한다는
운영 contract를 분리해서 test와 README에 함께 남긴다.

### Evidence

- `TestNearCacheClearsLocalOnRedisOutage` 추가.
- `TestNearCacheRecreateAfterRedisOutageRestoresPeerInvalidation` 추가.
- `README.md`와 `README.ko.md`에 terminal subscriber failure 또는 Redis restart 후
  recreate guidance를 추가했다.

## L2: Testcontainers 장애 test는 직렬 검증으로 다룬다

### 문제

Redis container 종료/재생성은 Docker runtime state와 Pub/Sub receive loop timing에
민감하다. parallel test execution은 같은 기능을 검증해도 flake 원인을 키울 수 있다.

### 교훈

Redis NearCache failure-injection은 package 단위 직렬 `go test`와 race run으로
검증하고, broader CI에서도 같은 invocation 안에서 sequential하게 실행한다.

### Evidence

- broad validation 전에 `go test -count=1 ./cache/redisnear -run "RedisOutage|RecreateAfterRedisOutage"`가 통과했다.
