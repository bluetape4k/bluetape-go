# Issue #545 Step 2-R JWKS provider 설계 리뷰

날짜: 2026-08-16
범위: `docs/superpowers/specs/2026-08-16-issue-545-jwks-provider-design.md`
게이트: Type-A Step 2-R, six independent lanes plus main integration review
근거: Issue #545, Epic #540, Issue #497 JWKS 우선 결정, go-jose/v4 및 jwx/v3 공식 API 조사

## 통합 판정

PASS. 설계 보강 후 최신 blocker count는 `P0=0 P1=0`이다. 구현 전 단계이므로
소스 테스트 결과가 아니라 public contract, failure mode, cache/concurrency,
운영 복구, caller misuse 경계를 검토했다.

## 관점별 결과

| 관점 | 초기 결과 | 최신 결과 | 통합 근거 |
|---|---:|---:|---|
| Performance | P0=0 P1=3, lane timeout 뒤 main fallback | P0=0 P1=0 | forced-refresh generation/cooldown coalescing, 실패 재시도 제한, mutex 밖 HTTP/decode, 원자적 publication과 benchmark metadata를 명시했다. |
| Stability | P0=0 P1=0, lane timeout 뒤 main fallback | P0=0 P1=0 | leader context 취소 시 live waiter 1회 takeover, waiter 취소 독립성, flight cleanup, TTL 만료 fail-closed를 명시했다. |
| Security | P0=0 P1=0, lane timeout 뒤 main fallback | P0=0 P1=0 | redirect 기본 거부, endpoint scheme/host/userinfo/fragment 검사, asymmetric-only policy, alg/key 일치, redacted error, x5u 추가 fetch 금지를 확인했다. |
| Operator/Ops | P0=0 P1=1, P2=3 | P0=0 P1=0 | network-free startup과 readiness `Refresh`, 3회/5분 page 기본값, bounded recovery, rollback/failover ownership, mixed-version overlap을 runbook 계약으로 추가했다. |
| Developer/API | P0=0 P1=1, P2=2 | P0=0 P1=0 | exported sentinel/type/error matrix, typed `Algorithm`, option 경계, `KeyFunc` construction error, nil context/receiver semantics를 고정했다. |
| User/Caller | P0=0 P1=1, P2=4 | P0=0 P1=0 | `KeyFunc`는 claims를 검증하지 않는다는 경계와 안전한 `ParseWithClaims` 예제 요구, request별 context lifecycle, readiness와 error retry guidance를 명시했다. |

타임아웃 lane은 `lane timed out; main integration fallback performed`로 기록했다.
보완 결과는 원래 blocked lane의 late result가 아니라 main integration에 귀속한
read-only evidence로 취급했다.

## 메인 통합 변경

- `KeyFunc(ctx)`는 `(golangjwt.Keyfunc, error)`를 반환하고, 매 요청 context로
  새 adapter를 만들며 취소된 closure 재사용을 금지한다.
- `Algorithm` type과 asymmetric 상수를 도입하고 `WithAllowedAlgorithms`와
  `Lookup`의 raw string 오용을 줄였다. HMAC/대칭키는 계속 거부한다.
- `ErrFetch`, `ErrMalformedSet`, `ErrUnsupportedAlgorithm`, `FetchError`,
  `SetError`, root `jwt` sentinel alias와 `errors.Is`/`errors.As`/retry matrix를
  public contract로 고정했다.
- 기본 TTL 5분, forced-refresh cooldown 1초, `now >= fetchedAt+TTL`, nil/empty/
  duplicate option 검증과 deterministic clock seam을 명시했다.
- unknown `kid` 및 실패 refresh를 generation/cooldown 단위로 합치고,
  leader 취소 takeover와 mutex 밖 HTTP/read/decode를 요구했다.
- `New`는 network-free이며 startup caller가 readiness `Refresh`를 수행한다.
  TTL 만료 뒤 stale key는 반환하지 않고, endpoint rollback/failover와 retry/
  backoff/observability ownership은 caller에게 남긴다.
- 두 README에 claims validation, error matrix, 운영 복구, rollback,
  mixed-version, 3회/5분 page 기본값을 source-equivalent로 기록하고, root
  README에서는 JWKS optional boundary와 JWE deferred를 분리한다.
- benchmark에는 `go version`, `GOOS/GOARCH`, CPU, raw output, HTTP/lock count를
  보존하며 `go-jose/v4 v4.1.4` module/license/checksum evidence와
  `CHANGELOG.md` `[Unreleased]` 항목을 구현 gate에 추가했다.

## 유예한 P2/P3

구현 단계에서 확인할 세부사항은 남아 있지만 P1 blocker는 없다.

- 구체적인 lock wait 수치 threshold는 machine noise를 피하기 위해 고정하지
  않고, named benchmark fixture와 HTTP/lock count를 deterministic acceptance로
  사용한다.
- `Algorithm` 상수의 package-level Go doc 문구와 실제 key copy helper 분할은
  Step 3 구현 계획에서 고정한다.
- 별도 운영 서비스의 metric backend, endpoint failover automation, universal
  alert policy는 library scope 밖이며 README caller-owned runbook으로 남긴다.

## SPW-01~05

- SPW-01: 독자와 목적, 기존 `jwt`/#497 경계, 구현 가능한 API·오류·cache·테스트·문서 계약을 명시했다.
- SPW-02: 문제 → 대안 → package/API → key policy → fetch/cache → 운영 → 테스트 → 문서 순으로 의존성을 정리했다.
- SPW-03: 독자-facing prose는 한국어로 작성하고 Go 식별자, 명령, URL, sentinel은 원문 token을 보존했다.
- SPW-04: Issue #545/#497, root `jwt`, go-jose/v4, jwx/v3 공식 자료와 수용 기준을 연결했다.
- SPW-05: 파일 read-back 및 `git diff --check`를 통과했고 redirect, empty set, nil context, defensive copy, benchmark, readiness 보강을 반영했다.

## 시각 자료 판정

이번 설계는 새 사용자-facing diagram을 요구하지 않는다. cache/flight 상태는
문장과 테스트 계약으로 충분하므로 `$bluetape-diagram` gate는 N/A로 기록한다.

## 검증

- `git diff --check`: PASS
- 설계 단계의 변경 파일: 설계 문서와 본 리뷰 문서만
- 소스 구현/무거운 테스트: Step 3 이후 수행

최종: `P0=0 P1=0`, Step 2-R PASS.
