# Issue #173 Task 4 Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #173
Task: Redis DTO Codec and Secret-Safe Validation
날짜: 2026-06-12

## 범위

- `jwt/redis_dto.go`
- `jwt/redis_dto_test.go`

## TDD 증거

| Phase | Evidence | Status |
| --- | --- | --- |
| RED | `go test -count=1 ./jwt -run 'TestEncodeDecode|TestDecode|TestDTO|TestRedisDTO'` failed before implementation with missing DTO symbols. | PASS |
| GREEN | Same targeted command passes after implementation. | PASS |
| Regression | `go test -count=1 ./jwt ./jwt/redis` passes. | PASS |

## Spec and Quality Review

| Requirement | Evidence | Status |
| --- | --- | --- |
| DTO version is fixed. | `redisKeyVersion = 1`; unknown version test rejects it. | PASS |
| HMAC material is base64 encoded. | `encodeRedisKeyChain` writes `HMAC`; decode calls `newHMACKeyChain`. | PASS |
| RSA private key material is base64 PKCS#1. | `x509.MarshalPKCS1PrivateKey` and `x509.ParsePKCS1PrivateKey`. | PASS |
| Decode checks max payload before JSON. | `decodeRedisKeyChain` length check precedes `json.Unmarshal`. | PASS |
| Decode validates kid, algorithm family, HMAC length, and RSA material. | Targeted DTO tests cover all cases. | PASS |
| Errors do not leak key material. | `TestDTOErrorsDoNotLeakKeyMaterial`. | PASS |
| No public raw-key repository helper was added. | DTO codec remains package-private inside `jwt`; public raw-key helper search found no new production helper. | PASS |

## 판정

P0=0 P1=0

Task 4 verdict: PASS
