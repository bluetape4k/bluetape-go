# Issue #173 Task 7 Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #173
Task: Cross-Instance Integration, Stress, Race, and Cancellation
날짜: 2026-06-12

## 범위

- `jwt/redis_integration_test.go`
- `jwt/redis_repository_test.go`
- `jwt/distributed_provider_test.go`

## Test Evidence

| Phase | Evidence | Status |
| --- | --- | --- |
| Test-first | Added Redis-backed cross-instance provider, stress, cancellation, deadline, failure, and eviction tests before any Task 7 production-code edit. | PASS |
| GREEN | `go test -p 1 -count=1 ./jwt ./jwt/redis` passed. | PASS |
| Race | `go test -race -p 1 -count=1 ./jwt ./jwt/redis` passed. | PASS |
| Whitespace | `git diff --check` passed. | PASS |

Task 7 did not require new production code after Task 6; the new integration tests passed against the existing Redis repository and distributed provider implementation.

## Spec and Quality Review

| Requirement | Evidence | Status |
| --- | --- | --- |
| HMAC keys are shared across Redis-backed distributed provider instances. | `TestRedisDistributedProvidersShareHMACKeysAcrossInstances`. | PASS |
| RSA keys are shared across Redis-backed distributed provider instances. | `TestRedisDistributedProvidersShareRSAKeysAcrossInstances`. | PASS |
| Retained old `kid` parses after forced rotation. | `TestRedisDistributedProviderParsesAfterForcedRotationByKID`. | PASS |
| Evicted `kid` fails with `ErrKeyNotFound`. | `TestRedisDistributedProviderRejectsEvictedKID`. | PASS |
| Redis client failure propagates to callers. | `TestRedisDistributedProviderRepositoryFailurePropagates`. | PASS |
| Constructor cancellation/deadline after create does not persist candidates. | `TestRedisDistributedProviderConstructorCanceledAfterCreateDoesNotPersistCandidate`; `TestRedisDistributedProviderConstructorDeadlineAfterCreateDoesNotPersistCandidate`. | PASS |
| Concurrent rotate/sign/parse survives bounded goroutine stress. | `TestRedisDistributedProviderConcurrentRotateSignParseStress` uses `GoroutineStressTester`. | PASS |
| Concurrent empty repository rotate converges on one current winner. | `TestRedisRepositoryConcurrentEmptyRotateConvergesOnOneCurrentWinner` uses `GoroutineStressTester`. | PASS |
| Provider cancellation/deadline behavior is stable under async stress. | `TestRedisDistributedProviderContextCancellationStress`; `TestRedisDistributedProviderDeadlineStress` use `AsyncJobTester`. | PASS |

## 판정

P0=0 P1=0

Task 7 verdict: PASS
