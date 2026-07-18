# redisvalue

[English](README.md) | [한국어](README.ko.md)

`redisvalue`는 bounded serialized Redis L2와 선택적인 process-local L1
decorator를 제공합니다. Coherent multi-process near cache가 아니라 cache-aside
인프라입니다.

<!-- redisvalue-contract: l1-boundary -->
## L1과 L2 경계

`ValueCache[V]`만 값을 직렬화해 Redis에 저장합니다. `TieredCache[V]`의 L1은
`V`를 그대로 저장하므로 정상 L1 hit에서는 Redis와 serializer를 호출하지
않습니다. 따라서 pointer identity는 process-local입니다. 서로 다른 decorator가
동일한 L2 bytes를 cold read하면 서로 다른 객체를 decode합니다.

<!-- redisvalue-contract: config -->
## 설정

`DefaultConfig()` 결과를 복사한 뒤 cache별로 필요한 section을 override합니다.

| 설정 | 기본값 | 의미 |
|---|---:|---|
| `Value.RemoteTTL` | `1h` | 기본 Redis TTL. `0`이면 영구 보관 |
| `Value.MaxValueBytes` | `1 MiB` | 허용할 최대 serialized bytes |
| `Value.ClearBatchSize` | `100` | `SCAN COUNT` hint와 최대 `UNLINK` argument 수 |
| `Tiered.LocalTTL` | `30m` | L1 TTL 상한 |
| `Tiered.InvalidationWaitTimeout` | `30s` | public invalidation 대기 budget |
| `Tiered.LocalCleanupTimeout` | `1s` | mandatory/explicit L1 cleanup budget |

`Set`과 `GetOrLoad`는 entry별 Redis TTL을 받고, `SetDefault`와
`GetOrLoadDefault`는 복사된 `RemoteTTL`을 사용합니다.

<!-- redisvalue-contract: ownership -->
## 소유권과 reference

직접 연결한 `*redis.Client`, serializer, L1 lifecycle은 caller가 소유합니다.
공급한 L1의 cache operation은 `TieredCache`만 수행해야 합니다. 다른 경로로
L1을 읽거나 변경하지 마십시오. Pointer-valued `V`는 L1이 원래 reference를
보관하므로 cache에 있는 동안 immutable snapshot으로 다룹니다.

<!-- redisvalue-contract: l1-provenance -->
공급하는 `Local`은 새 cache이거나 비어 있어야 합니다. 기존 값이 있다면 정확히
같은 remote namespace/schema/tenant의 값만 있어야 합니다. L1 key는 내부에서
namespace-qualified되지 않은 caller의 raw logical key이므로 다른 decorator와
재사용하거나 공유하지 마십시오.

<!-- redisvalue-contract: load-policy -->
## Read와 load 정책

Read 순서는 L1, L2, 같은 key의 첫 leader loader입니다. 정확한
`cache.ErrCacheMiss`만 다음 tier로 넘어가며 provider, decode, local error는 즉시
중단합니다. 하나의 process-local flight는 첫 leader의 context, TTL, loader,
value, error를 공유합니다. Cross-process stampede control은 이 decorator의
범위가 아닙니다.

<!-- redisvalue-contract: ttl -->
## TTL 의미

Redis TTL `0`은 만료 없음이고 음수 TTL은 거부합니다. 1ms보다 짧은 양수는
wire에서 최소 1ms로 저장합니다. Known write에서는 L1이 해당 L2 TTL보다 오래
남지 않도록 L1 TTL을 줄입니다. L2 hit는 기존 key의 remaining TTL을 원자적으로
알 수 없으므로 refill 후 stale window는 `LocalTTL`만큼 생길 수 있습니다. 이
window가 허용되지 않으면 L1 TTL을 충분히 짧게 잡거나 이 topology를 사용하지
마십시오.

<!-- redisvalue-contract: errors -->
## Error와 blocked recovery

Mutation은 Redis-first입니다. `SET` 또는 `DEL`을 호출한 뒤 provider error나
늦은 cancellation이 발생하면 commit-unknown일 수 있어 mandatory L1 cleanup을
수행합니다. Cleanup 성공을 증명하지 못하면 decorator는 `ReasonLocalBlocked`로
전환해 fail closed합니다. 성공한 explicit `ClearLocal`만 이 상태를 복구합니다.
Public error는 redacted 상태로 `errors.Is`/`errors.As` 검사를 지원합니다.

<!-- redisvalue-contract: clear -->
## Clear와 fleet reset

`InvalidateLocal`은 이 decorator의 entry 하나만 제거합니다. `ClearLocal`은 이
decorator만 비우며 explicit repair 역할도 합니다. `ValueCache.Clear`는 L2
namespace만 지우는 admin operation이고, `TieredCache.Clear`는 L2를 먼저 지운
뒤 이 decorator의 L1을 비웁니다. Redis clear는 non-atomic `SCAN`과 bounded
sequential `UNLINK`이므로 concurrent write가 남을 수 있습니다. Fleet reset은
writer quiesce, clear-admin으로 L2 clear, 모든 process의 `ClearLocal` 또는 재시작,
namespace 확인, traffic 재개 순서로 수행합니다.

<!-- redisvalue-contract: topology -->
## 지원하는 Redis topology

안정적인 writable primary 하나에 직접 연결한 `*redis.Client`를 사용합니다.
Failover client, routing이 불명확한 proxy, Redis Cluster, Ring은 지원하지
않습니다. 이 package의 key, scan, mutation ordering, commit-unknown 증명은 한
primary command domain을 전제로 합니다.

<!-- redisvalue-contract: operations -->
## 운영과 ACL

일반 identity에는 namespace 대상 `GETRANGE`, `EXISTS`, `SET`, `DEL`만
부여하고 정확한 key pattern `bluetape:cache:value:<namespace>:*`을 사용합니다.
별도 clear-admin client로 `ValueCache`를 구성해 `SCAN`과 namespace-scoped
`UNLINK`만 부여하고 `FLUSHDB`와 `FLUSHALL`은 거부합니다. ACL key pattern만으로
tenant isolation이 되지는 않습니다. `SCAN`에서 foreign key name이 보일 수
있으므로 Redis network와 TLS도 분리하십시오.

Service budget에 맞춰 caller-owned go-redis `DialTimeout`, `ReadTimeout`,
`WriteTimeout`, `PoolTimeout`을 설정합니다. Readiness는 `PING`만이 아니라
안정적인 writable primary command path를 증명해야 합니다. Serialized payload와
TTL 정책에 맞춰 Redis `maxmemory`와 eviction policy를 정합니다. 이 package는
no telemetry 정책이므로 caller-owned go-redis hooks를 설치하고 memory, eviction,
command latency/timeout, provider reason, blocked decorator, partial-clear progress를
관찰합니다. Partial clear는 resumable cursor가 아니므로 cursor 0에서 다시
시작합니다. `ClearProgress.ScannedKeys`는 지금까지 `SCAN`이 반환한 matching key
수일 뿐 total, percentage, cursor가 아닙니다.

<!-- redisvalue-contract: versioning -->
## Versioning과 rollout

`serialization.VersionedSerializer` 사용을 권장합니다. 호환되지 않는 wire
변경은 namespace를 회전합니다. Namespace를 재사용하는 rollout은 upgrade와
rollback의 정확한 reader/writer matrix를 증명하고, rollback window와 최대 finite
Redis TTL이 끝날 때까지 old reader를 유지해야 합니다. TTL `0` data는 호환성을
제거하기 전에 명시적인 admin cleanup 계획이 필요합니다.

<!-- redisvalue-contract: resp3 -->
## RESP3와 인접 cache package

현재 Pub/Sub `cache/redisnear`를 이 decorator에 감싸지 마십시오. 두 component가
local invalidation을 동시에 소유하면서도 L1 coherence는 보장하지 못합니다.
`cache/rediscoord`의 loader ownership도 이 decorator의 local same-key flight와
분리합니다. Issue #536은 향후 coherent near-cache mode를 위한 public RESP3
client-tracking capability와 correctness proof gate입니다. #535는 RESP3를
요구하거나 제공하지 않습니다.

<!-- redisvalue-contract: tests -->
## 테스트

Docker resource를 공유한다면 일반 테스트와 Testcontainers 테스트를 순차 실행합니다.

```bash
go test -p 1 -count=1 ./cache/redisvalue
go test -race -p 1 -count=1 ./cache/redisvalue
```

<!-- redisvalue-contract: untrusted-payload -->
## 신뢰하지 않는 payload

Redis bytes를 신뢰하지 마십시오. Executable deserialization을 피하고 malformed
input에서 panic 대신 error를 반환하는 serializer를 사용합니다. Serializer가
temporary allocation, nesting/recursion, decompression, CPU work를 제한해야 합니다.
`MaxValueBytes`는 Redis admission bytes만 제한하며 decoder work는 제한하지 않습니다.

<!-- redisvalue-contract: authentication -->
## Payload 인증

Tamper-sensitive deployment는 `VersionedSerializer`와 별도로 authenticated
envelope로 payload를 감싸야 합니다. Built-in versioning은 compatibility와 format
mismatch를 찾지만 악의적인 변경은 탐지하지 않습니다.

<!-- redisvalue-contract: namespace -->
## Namespace trust domain

Namespace 하나는 하나의 exclusive tenant, schema, clear trust domain입니다.
Authorization boundary는 아닙니다. 호환되지 않는 tenant나 wire format은 서로 다른
namespace와 Redis ACL/network isolation을 사용해야 합니다.

<!-- redisvalue-contract: scan-bounds -->
## SCAN bound

`SCAN COUNT`는 hint입니다. Client는 Redis가 정한 page 하나를 보관하고 이를
`ClearBatchSize` 단위의 `UNLINK` argument chunk로 나누지만, 반환된 page나 이
package 외부 key의 byte size까지 제한할 수는 없습니다.

<!-- redisvalue-contract: serializer-concurrency -->
## Serializer concurrency

Serializer는 caller-owned이며 construction 후 immutable하고 concurrent
`Marshal`/`Unmarshal`에 안전해야 합니다. Package는 serializer를 clone하지 않고
global lock으로 호출을 직렬화하지도 않습니다.

<!-- redisvalue-contract: compatibility-matrix -->
## Compatibility matrix

Built-in versioned envelope는 backward-readable만 지원합니다. Version-2 reader는
version-1 bytes를 읽지만 version-1 reader는 version-2 bytes를
`serialization.ErrUnsupportedVersion`으로 거부합니다. Application이 정확한
bidirectional serializer matrix를 증명하기 전에는 upgrade/rollback window에서
namespace를 재사용할 수 없습니다.
