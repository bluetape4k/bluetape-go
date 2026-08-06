# Cross-Process Stampede Protection

Issue: #117
Date: 2026-06-04

## Lesson

Redis lock-only coordination은 NearCache load collapse에 충분하지 않다. NearCache
peer는 value를 공유하지 않으므로 lock은 loader를 serialize할 뿐 waiter가 winner의
result를 재사용하게 해주지 않는다.

## Decision

caller-provided codec과 short-lived token-bound result envelope을 갖춘 명시적
`cache/rediscoord` wrapper를 사용한다. `cache/redisnear`는 기본적으로
invalidation-only package로 유지한다.

## Implementation Note

waiter는 `Set`이 아니라 wrapped `GetOrLoad`를 통해 local state를 채워야 한다.
NearCache `Set`은 peer invalidation을 publish하기 때문이다. coordinator result는
local fill이지 write/invalidation command가 아니다.

## Operational Note

`LockTTL`은 progress와 mutual exclusion을 모두 제한한다. loader가 lease보다 오래
실행되면 다른 process가 acquire하고 load할 수 있다. lease는 예상 loader duration에
맞게 조정하고, Redis payload exposure를 deployment security boundary의 일부로
취급한다.
