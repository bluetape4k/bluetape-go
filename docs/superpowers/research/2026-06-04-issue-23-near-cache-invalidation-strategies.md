# Issue 23 Near Cache Invalidation Strategies Research

Issue: #23
Milestone: 0.3.0
Date: 2026-06-04

## Research Question

`bluetape-go` near cache는 local L1 cache와 Redis-backed invalidation을 지원해야 한다.
design question은 application-level Redis Pub/Sub invalidation만 노출할지, 아니면 RESP3
`CLIENT TRACKING` 기반 Redis server-assisted client-side caching을 위한 first-class extension
point도 남길지다.

## Primary Evidence

| Source | Evidence | Decision impact |
|---|---|---|
| GitHub issue #23 | local TTL cache, explicit invalidation, Redis Pub/Sub peer invalidation, Testcontainers two-client proof가 필요하다. | 첫 구현은 app-level Pub/Sub invalidation을 증명해야 한다. |
| Redis client-side caching introduction | Redis server-assisted client-side caching은 client가 읽은 key를 track하고 다른 client가 해당 key를 write하면 invalidation을 보낸다. | RESP3 tracking은 단순 channel format이 아니라 다른 invalidation strategy다. |
| Redis `CLIENT TRACKING` command reference | invalidation message는 RESP3에서 같은 connection으로 전달되거나 다른 connection으로 redirect될 수 있다. option에는 `BCAST`, `PREFIX`, `OPTIN`, `OPTOUT`, `NOLOOP`가 있다. | future tracking implementation에는 dedicated lifecycle과 option modeling이 필요하다. |
| Redis client-side caching reference | default tracking은 client-key relationship을 server-side에 저장한다. broadcasting mode는 server tracking memory를 피하지만 prefix notification을 보낸다. RESP2 Pub/Sub redirection은 compatibility mechanism이지 ordinary broadcast Pub/Sub이 아니다. | app-level Pub/Sub과 server-assisted tracking을 하나의 semantic mode로 합치면 안 된다. |
| go-redis repository and Redis blog | `go-redis/v9`는 RESP3를 지원하고 이 repository의 기존 Redis dependency다. | Redis dependency를 교체하지 않아도 RESP3는 기술적으로 가능하지만 high-level client-side caching behavior는 별도 proof가 필요하다. |
| Current `cache` package | `cache.Memory`는 TTL, `Delete`, `Clear`, `GetOrLoad`, same-key duplicate-load suppression, stress/cancellation coverage를 이미 제공한다. | benchmark evidence 전에는 Ristretto/BigCache를 고르지 말고 `cache.Memory`를 default local L1 store로 재사용한다. |
| Milestone 0.3.0 research | Ristretto와 BigCache는 local-cache candidate이고 `go-redis/v9`는 Redis backend candidate다. | #107이 storage choice를 benchmark하기 전까지 storage와 invalidation strategy를 독립적으로 유지한다. |

## Strategy Comparison

| Strategy | Strengths | Risks |
|---|---|---|
| Application-level Redis Pub/Sub invalidation | `go-redis/v9`로 단순하게 구현 가능하다. ordinary Pub/Sub과 동작하며 namespace, operation, origin ID, version 같은 bluetape-go-specific field를 실을 수 있다. Testcontainers로 검증하기 쉽고 managed Redis/proxy의 RESP3 support와 독립적이다. | 이 near-cache contract를 경유한 write만 invalidation을 publish한다. subscriber는 disconnect 중 message를 놓칠 수 있고 reconnect 시 local state를 clear해야 한다. broadcast volume은 namespace/channel design에 좌우된다. |
| RESP3 `CLIENT TRACKING` | Redis가 실제 read key를 관측하고 어떤 client가 수정해도 invalidate한다. default tracking mode에서는 notification volume을 줄일 수 있고 `PREFIX`, `BCAST`, `NOLOOP`, `OPTIN`, `OPTOUT` 같은 Redis-native option을 지원한다. Lettuce/Redisson client-side caching semantics에 가깝다. | connection-pool/lifecycle behavior가 어렵다. disconnect 시 local cache flush/retracking이 필요하다. Go push-message handling proof가 필요하고 managed Redis/proxy RESP3 support가 다를 수 있다. API는 low-level protocol detail을 새지 않게 option을 노출해야 한다. |

## Design Decision

public API에 hard-coded mode 하나를 넣지 말고 near-cache invalidation을 strategy boundary로 노출한다.

첫 구현은 application-level Redis Pub/Sub invalidation이어야 한다. #23을 직접 만족하며 기존
`go-redis/v9` dependency와 Testcontainers로 검증할 수 있기 때문이다. RESP3 `CLIENT TRACKING`은
가능하면 같은 public near-cache contract를 만족하는 별도 follow-up implementation으로 추적한다.

## Proposed API Direction

hidden behavior가 있는 overloaded enum 하나 대신 constructor 또는 concrete strategy type을 사용한다.

- `redisnear.NewPubSub(...)`: application-level invalidation channel과 payload contract.
- future `redisnear.NewTracking(...)`: RESP3 `CLIENT TRACKING`을 통한 Redis server-assisted client-side caching.
- shared local store default: `cache.Memory[K, V]`.
- shared public behavior: `Get`, `Set`, `Delete`, `Clear`, `GetOrLoad`, local TTL, explicit invalidation,
  close/shutdown semantics, documented reconnect behavior.

strategy option은 construction time에 보여야 한다. Pub/Sub과 tracking은 lifecycle, wire semantics,
invalidation guarantee가 다르므로 runtime switching은 지원하지 않는다.

## Pub/Sub Implementation Notes for #23

- 최소 namespace, operation, key, origin ID, protocol version을 가진 stable invalidation message를 정의한다.
- arbitrary Redis keyspace change에서 peer cache를 invalidate하지 않는다. near-cache write/delete/clear만 message를 publish한다.
- 적절한 곳에서는 origin ID로 redundant local invalidation을 피한다.
- subscription disconnect 또는 reconnect ambiguity가 있으면 stale read를 피하기 위해 local cache를 clear한다.
- Testcontainers로 두 near-cache instance가 peer invalidation을 관측한다는 점을 증명한다.
- timing/concurrency와 cancellation coverage에는 `GoroutineStressTester`와 `AsyncJobTester`를 사용한다.

## RESP3 Follow-up Notes

- public constructor shape를 확정하기 전에 `go-redis/v9` push-message와 connection-pinning behavior를 검증한다.
- tracking이 default mode, `BCAST` + `PREFIX`, `OPTIN`, `OPTOUT` 중 무엇을 쓸지 결정한다.
- Redis server 및 managed Redis compatibility requirement를 문서화한다.
- disconnect behavior를 정의한다: local flush, tracking re-enable, error surfacing.
- `NOLOOP`가 local write-through 또는 cache-aside flow와 어떻게 상호작용하는지 확인한다.
- external Redis write가 tracked local entry를 invalidate한다는 Testcontainers proof를 추가한다.

## Adopt / Borrow / Skip Decisions

| Candidate | Decision | Rationale |
|---|---|---|
| `cache.Memory` | Adopt for first L1 | 이미 merged/tested/generic이며 #23 semantics에 충분하다. |
| Ristretto / BigCache | Defer to #107 | useful performance candidate지만 storage choice는 benchmark-driven이어야 한다. |
| App-level Redis Pub/Sub | Adopt for #23 | issue acceptance criteria를 직접 만족하고 predictable test가 가능하다. |
| RESP3 `CLIENT TRACKING` | Track as follow-up | Lettuce/Redisson parity와 external-write invalidation에는 가치가 있지만 lifecycle risk가 더 높다. |
| One enum hiding both modes | Reject | lifecycle과 consistency semantics가 실질적으로 다른데 이를 숨긴다. |

## Source Links

- Redis client-side caching introduction:
  https://redis.io/docs/latest/develop/clients/client-side-caching/
- Redis `CLIENT TRACKING` command:
  https://redis.io/docs/latest/commands/client-tracking/
- Redis client-side caching reference:
  https://redis.io/docs/latest/develop/reference/client-side-caching/
- go-redis repository:
  https://github.com/redis/go-redis
- Redis blog: Go-Redis is now an official Redis client:
  https://redis.io/blog/go-redis-official-redis-client/

## Step 1-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Official docs checked | Done | Redis client-side caching introduction/reference and `CLIENT TRACKING`. |
| Current repo checked | Done | existing `cache.Memory`, issue #23, milestone 0.3.0 research, `go.mod`. |
| Third-party assumptions checked | Done | `go-redis/v9`는 Redis dependency로 남고 client level에서 RESP3를 지원한다. |
| Strategy risks identified | Done | Pub/Sub disconnect loss, RESP3 connection lifecycle, managed Redis compatibility, storage benchmark timing. |
| Decision recorded | Done | Pub/Sub first, RESP3 tracking as separate strategy/follow-up. |
