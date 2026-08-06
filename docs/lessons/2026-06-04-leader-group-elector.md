# LeaderGroupElector

## Context

Issue #85는 최대 `MaxLeaders` worker만 coordinated task를 동시에 실행하도록
`bluetape-go`에 Redis-backed group elector를 추가한다.

## Decision

- Go Redis group key는 Kotlin/JVM Redis group key와 분리한다.
- Redis ZSET에 `memberID:random` token과 server-time expiry score를 사용한다.
- group election은 semaphore-like behavior이므로 full slot에서 즉시
  `ErrNotLeader`를 반환하지 않고 `Campaign`을 context-bounded로 둔다.

## Outcome

구현은 기존 single-elector lifecycle shape를 따르면서 count/status method와 expiry
reclamation을 추가한다.

## Verification

planned gate는 `leader`와 `leader/redis` targeted `go test`, `make ci`,
diff check, local 7-tier review다.

## Future Guard

Redis coordination primitive를 추가할 때는 ownership token을 opaque하게 유지하고,
expiry decision에는 server-side time을 사용한다. 명시적 compatibility adapter가
없으면 Kotlin/Go non-interop을 문서화한다.
