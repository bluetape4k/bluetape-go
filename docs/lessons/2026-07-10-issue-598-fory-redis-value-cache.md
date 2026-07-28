# Issue #598 교훈: Payload Bound는 Redis Read에서 시작해야 한다

## Context

direct Redis value cache는 binary envelope를 완벽히 검증하더라도, 먼저 저장된 전체
value를 다운로드하면 resource-bound 약속을 지키지 못할 수 있다. 잘못된 rollout
data, stale writer, manual corruption, compromised peer는 valid key 아래 configured
Fory payload limit보다 큰 value를 둘 수 있다.

## Learning

가장 이른 controllable allocation에서 bound를 강제한다. Redis에서는 envelope header,
configured payload, overflow-detection byte 하나만 읽고, Fory가 value를 보기 전에
overflow를 거절한다. `GETRANGE`는 missing key와 existing empty value 모두에 empty
string을 반환하므로 existence check와 묶어 `cache.ErrCacheMiss`가 계속 absent 또는
expired를 뜻하게 하고, empty corrupt data는 envelope validation에서 실패하게 한다.

Operational documentation도 이 contract의 일부다. Go API가 바뀌지 않아도 `GET`을
`GETRANGE`와 `EXISTS`로 교체하면 least-privilege ACL surface가 바뀐다. restricted
Redis user로 인증해 정확히 문서화된 command set이 lifecycle을 지원함을 증명하는
integration test를 유지한다.

Wire-format independence도 중요하다. shared Fory runtime code는 duplicate locking,
registration, panic, bounds logic을 제거할 수 있지만 public package는 서로 다른
storage semantics를 위해 별도 `BTFV`와 `BTFY` envelope를 유지한다.

## Durable Checks

- 모든 external side effect 직전에 cancellation을 다시 확인한다.
- decode 또는 decompression 전에 network response materialization을 bound한다.
- missing, empty-corrupt, oversized data의 구분을 보존한다.
- namespace, profile, registration name, schema generation, limit, Redis ACL
  command를 하나의 rollout contract로 취급한다.
- provider cause는 sanitized category로 대체한다. raw provider text를 logging하지
  않고 caller-owned hook에서 infrastructure failure를 분류한다.
- explicit Redis readiness polling을 사용하고 local Docker resource가 제한적이면
  shared Testcontainers package를 serial로 실행한다.
- raw output, environment/revision metadata, table, Chart, written analysis가
  생길 때까지 performance claim은 #599에 남긴다.
