# RESP3 CLIENT TRACKING Research Lessons (2026-06-04)

**Related issue**: #110
**Impact modules**: `docs/research`, `cache/redisnear`

## L1: strategy boundary에는 dependency-level proof가 필요하다

### 문제

RESP3 `CLIENT TRACKING`은 Redis NearCache 대안처럼 보이지만, 현재
`go-redis/v9` surface는 RESP3와 push notification hook을 제공할 뿐 high-level
typed `CLIENT TRACKING` API를 제공하지 않는다.

### 교훈

lower-level `CLIENT TRACKING` command, push handler, connection affinity,
reconnect behavior가 Testcontainers와 함께 증명되기 전에는 public
`redisnear.NewTracking` constructor를 노출하지 않는다.

### Evidence

- `docs/research/2026-06-04-issue-110-resp3-client-tracking.md`
- `github.com/redis/go-redis/v9 v9.20.0` local dependency inspection.

## L2: research artifact는 searchable research index에 있어야 한다

### 문제

#23에는 이미 `docs/superpowers/research` note가 있었지만, 사용자는 GNO가 milestone
knowledge로 검색할 수 있는 durable `docs/research` material을 명시적으로 요구했다.

### 교훈

Milestone strategy decision은 `docs/research` 아래에 쓰고,
`docs/research/README.md`에서 link하며, PR creation 전에 milestone research note와
연결한다.

### Evidence

- `docs/research/2026-06-04-issue-110-resp3-client-tracking.md` 추가.
- `docs/research/README.md` 갱신.
- `docs/research/2026-06-01-milestone-0.3.0-cache-coordination-research.md` 갱신.
