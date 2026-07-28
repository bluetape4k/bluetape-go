# Issue #110 RESP3 CLIENT TRACKING NearCache Research

Issue: #110
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type E - Research / Maintenance

## Research Question

`bluetape-go`가 기존 application-level Redis Pub/Sub strategy의 first-class alternative로 RESP3
`CLIENT TRACKING` 기반 Redis server-assisted NearCache invalidation을 제공해야 하는가?

## Executive Decision

현재 slice에서는 RESP3 `CLIENT TRACKING`을 production `redisnear.NewTracking` constructor로 구현하지 않는다.

#23의 strategy boundary는 유지하되, public API를 약속하기 전에 별도 Testcontainers spike를 요구한다. 현재
`go-redis/v9` dependency는 RESP3 negotiation과 RESP3 push notification handling이 가능하지만, typed
`CLIENT TRACKING` API나 documented high-level client-side caching contract를 제공하지 않는다. 따라서 직접 production
implementation을 만들 수는 있어도 lower-level `Do("CLIENT", "TRACKING", ...)` command, push notification handler,
connection-pinning rule을 bluetape-go가 소유하고 먼저 증명해야 한다.

## Primary Evidence

| Source | Evidence | Impact |
|---|---|---|
| Redis client-side caching docs | Redis server-assisted client-side caching은 client가 read한 key를 추적하고 다른 client가 그 key를 modify하면 invalidation message를 보낸다. | RESP3 tracking은 `redisnear.NewPubSub`을 우회하는 external write를 invalidate할 수 있다. |
| Redis `CLIENT TRACKING` command docs | `CLIENT TRACKING`은 default tracking, `BCAST`, `PREFIX`, `OPTIN`, `OPTOUT`, `NOLOOP`, `REDIRECT`를 지원하며 invalidation은 RESP3 push message 또는 redirected notification으로 전달된다. | future API는 tracking mode를 Pub/Sub option 뒤에 숨기지 말고 명시적으로 모델링해야 한다. |
| Redis client-side caching reference | default mode는 server-side client-key state를 유지하고, broadcasting mode는 더 넓은 prefix notification과 server memory tradeoff를 가진다. | memory 및 notification-volume tradeoff가 Pub/Sub과 다르므로 문서화가 필요하다. |
| `go-redis/v9` README and source at v9.20.0 | `Options.Protocol`은 RESP2/RESP3를 지원하고 default protocol initialization은 RESP3이며, `PushNotificationProcessor` / `RegisterPushNotificationHandler`가 있다. | dependency 교체 없이 RESP3 proof가 기술적으로 가능하다. |
| `go-redis/v9` source search | first-class `ClientTracking` 또는 `CLIENT TRACKING` typed method는 발견되지 않았고 low-level `Do` / `Conn.Do`는 사용할 수 있다. | public bluetape-go support는 command shape, lifecycle, parsing risk를 직접 소유한다. |
| Current `cache/redisnear` implementation | `NewPubSub`은 app-level invalidation message shape, origin filtering, receive-error local clear, recreate guidance, Testcontainers outage coverage를 이미 소유한다. | Pub/Sub은 안정적인 0.3.0 implementation으로 남는다. RESP3가 이 contract를 약화하면 안 된다. |

## Strategy Comparison

| Dimension | Pub/Sub `NewPubSub` | RESP3 tracking candidate |
|---|---|---|
| Invalidation source | bluetape-go write/delete/clear가 explicit invalidation message를 publish한다. | Redis가 tracked key를 관찰하고 다른 client mutation 때 invalidate한다. |
| External writes | external writer가 bluetape-go protocol을 publish하지 않으면 덮지 않는다. | 잠재적으로 덮을 수 있으며 이것이 핵심 value proposition이다. |
| Wire contract | namespace, origin, operation, version을 가진 bluetape-go JSON message. | Redis-native invalidation push payload. |
| Connection model | ordinary Pub/Sub subscription과 terminal subscriber failure 뒤 recreate path. | RESP3 connection behavior, tracking enablement, push handler registration, reconnect re-enable policy가 필요하다. |
| API stability | #23/#116에서 구현 및 테스트 완료. | spike evidence 없이 public API로 준비되지 않았다. |
| Managed Redis / proxy risk | ordinary Pub/Sub은 널리 지원된다. | RESP3 push 및 tracking support는 provider/proxy별 검증이 필요하다. |

## Recommended API Direction

constructor-per-strategy를 유지한다.

```go
// 0.3.0에서 구현 및 안정화된 경로.
redisnear.NewPubSub[V](ctx, redisnear.Options[V]{...})

// candidate only. lifecycle semantics가 spike로 증명되기 전에는 노출하지 않는다.
redisnear.NewTracking[V](ctx, redisnear.TrackingOptions[V]{...})
```

기존 `Options` type 안에 `StrategyRESP3` 같은 runtime enum을 넣지 않는다. lifecycle, wire semantics,
provider compatibility risk가 Pub/Sub과 실질적으로 다르기 때문이다.

나중에 구현한다면 candidate `TrackingOptions`는 다음을 명시해야 한다.

- `Client` 또는 pinned `Conn` provider.
- `Namespace` / key prefix mapping.
- `Mode`: default tracked keys, `BCAST`, `OPTIN`, `OPTOUT`.
- `Prefixes`: `BCAST`에서만 valid.
- `NoLoop`: Redis `NOLOOP`에 매핑.
- `OnError`: receive, reconnect, tracking re-enable, handler failure.
- `Local`: Pub/Sub과 같은 `cache.LoadingCache[string,V]` abstraction.

## Lifecycle Requirements Before Implementation

public API를 약속하기 전에 future implementation은 다음을 모두 증명해야 한다.

1. RESP3 negotiation이 강제되고 Redis 또는 proxy가 RESP2로 fallback하면 명확히 실패한다.
2. read 추적이 필요한 같은 connection에서 `CLIENT TRACKING ON ...`이 enable되거나, implementation이 명시적 redirect
   connection을 문서화한다.
3. invalidation push payload가 normal command response와 race 없이 parsing된다.
4. disconnect, reconnect, pool connection replacement, `ConnMaxLifetime`이 local cache flush와 tracking re-enable을 유발한다.
5. local write-through 및 cache-aside flow에서 `NOLOOP` behavior가 정의된다.
6. external Redis write가 Testcontainers proof에서 tracked local entry를 invalidate한다.
7. managed Redis / Redis proxy compatibility가 supported, unsupported, unknown 중 하나로 문서화된다.

## Test Plan For A Future Spike

spike는 별도 PR에 두고 수렴 전까지 internal로 남겨도 된다.

- Testcontainers와 `go-redis/v9` `Protocol: 3`로 Redis를 시작한다.
- tracking invalidation용 push notification handler를 등록한다.
- connection affinity requirement를 증명한 뒤에만 `Conn.Do` 또는 `Client.Do`로 low-level `CLIENT TRACKING ON`을 enable한다.
- tracked connection으로 key `k1`을 읽고 local cache를 채운다.
- `redisnear`를 쓰지 않는 별도 Redis client에서 `k1`을 mutate한다.
- push handler가 invalidation을 관찰하고 local entry가 제거되는지 assert한다.
- reconnect/recreate와 local flush를 반복한다.
- `NOLOOP`으로 local writer semantics를 확인한다.

## Decision Matrix

| Option | Decision | Reason |
|---|---|---|
| 지금 `NewTracking` 구현 | Reject | spike 없이 lifecycle 및 low-level protocol risk가 너무 크다. |
| 0.3.0에서 #23 `NewPubSub`만 production strategy로 유지 | Adopt | 이미 구현, stress-tested, outage-tested, documented 상태다. |
| `NewTracking`을 future strategy boundary로 보존 | Adopt | external-write invalidation value가 API path를 열어 둘 만큼 크다. |
| Pub/Sub과 RESP3를 하나의 `Options.Strategy` enum 뒤에 숨김 | Reject | materially different connection 및 failure semantics를 흐린다. |
| #107에 RESP3 tracking benchmark 추가 | Defer | #107은 구현된 cache behavior를 benchmark해야 하며 RESP3 benchmark는 spike 뒤에 가능하다. |

## Acceptance Criteria Mapping

| #110 criterion | Status | Evidence / decision |
|---|---|---|
| `go-redis/v9`가 dependency 교체 없이 robust `CLIENT TRACKING`을 지원할 수 있는지 문서화 | Done | RESP3 + push handler + low-level `Do`로 가능성은 있지만 spike 없이는 public API로 충분히 robust하지 않다. |
| public constructor/API shape 결정 | Done | future `redisnear.NewTracking`과 explicit `TrackingOptions`; hidden enum 없음. |
| Redis server version 및 managed Redis compatibility assumption 검증 | Partial | Redis docs는 feature를 지원하지만 provider/proxy compatibility는 spike/documentation gate로 남는다. |
| disconnect/reconnect behavior 정의 | Done | local flush 및 tracking re-enable이 필요하며, 아니면 recreate하고 tracked local entry를 serve하지 않는다. |
| `NOLOOP` 및 external-write interaction 정의 | Done | `NOLOOP`은 explicit해야 하며 external-write invalidation이 RESP3를 재검토하는 주된 이유다. |
| Testcontainers proof 추가 또는 계획 | Done | 위 test plan이 다음 implementation gate다. |

## Source Links

- Redis client-side caching introduction:
  https://redis.io/docs/latest/develop/clients/client-side-caching/
- Redis `CLIENT TRACKING` command:
  https://redis.io/docs/latest/commands/client-tracking/
- Redis client-side caching reference:
  https://redis.io/docs/latest/develop/reference/client-side-caching/
- go-redis repository:
  https://github.com/redis/go-redis
- Local dependency inspected:
  `github.com/redis/go-redis/v9 v9.20.0`

## 최종 권고

#110은 production implementation이 아니라 research decision으로 닫는다.

다음 milestone work는 계획대로 #107 benchmark baseline과 #24 Redis distributed lock을 진행한다. RESP3 tracking은
Testcontainers로 push invalidation handling, connection affinity, reconnect semantics가 증명된 focused spike 뒤에만 다시 연다.
