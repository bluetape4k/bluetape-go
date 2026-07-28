# Issue #173 Task 3 Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #173
Task: Redis Options, Namespace, and Key Layout
날짜: 2026-06-12

## 범위

- `jwt/redis_options.go`
- `jwt/redis_options_test.go`
- `jwt/redis/doc.go`

## TDD 증거

| Phase | Evidence | Status |
| --- | --- | --- |
| RED | `go test -count=1 ./jwt -run 'TestOptions|TestRepositoryKeyNames'` failed before implementation with missing `RedisRepositoryOptions` and bounds constants. | PASS |
| GREEN | Same targeted command passes after implementation. | PASS |
| Regression | `go test -count=1 ./jwt ./jwt/redis` passes. | PASS |

## Spec and Quality Review

| Requirement | Evidence | Status |
| --- | --- | --- |
| Redis client is required and caller-owned. | `RedisRepositoryOptions.Client`; nil client returns `ErrInvalidOptions`. | PASS |
| Namespace is required and constrained. | `normalizeRedisNamespace` trims, rejects empty, rejects over 128 bytes, and allows only ASCII `[A-Za-z0-9._-]`. | PASS |
| Capacity bounds follow JWT repository bounds. | Default/min/max use `defaultRepositorySize`, `minRepositorySize`, and `maxRepositorySize`. | PASS |
| Payload bounds are fixed. | Default `32 << 10`, min `1024`, max `1 << 20`. | PASS |
| Retention leeway is explicit. | `RetentionLeeway` normalizes and rejects negative values. | PASS |
| Key names are versioned and namespaced. | `metaKey`, `currentKey`, `keysKey`, and `orderKey` produce `bluetape:jwt:v1:<namespace>:...`. | PASS |
| Facade package docs exist. | `jwt/redis/doc.go` documents Redis signing authority and caller-owned client lifecycle. | PASS |

## 판정

P0=0 P1=0

Task 3 verdict: PASS
