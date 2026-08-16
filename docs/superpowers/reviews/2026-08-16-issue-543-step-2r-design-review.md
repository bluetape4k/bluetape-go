# Issue #543 Gin adapter Step 2-R 설계 검토

## 검토 범위와 기준

- 대상 설계: `docs/superpowers/specs/2026-08-16-issue-543-gin-adapter-design.md`
- 기준 commit: `9fb45d11b8d0b777bbf96c34d32cf803b5b77ff1`
- 검토 방식: 독립 6개 관점 + main integration read-back
- 정책: P0/P1은 구현 계획 진입을 막고, P2는 계획·테스트·문서에 반영한다.
- 구현 코드/의존성 변경 전 read-only 설계 검토다.

## 독립 관점 결과

| 관점 | P0 | P1 | P2 | 판정 | 반영 |
| --- | ---: | ---: | ---: | --- | --- |
| Performance | 0 | 0 | 4 | COMMENT | benchmark baseline/bridge/full, parallel, `-cpu`, metric 방향, `make bench-web-gin`을 설계에 추가 |
| Stability | 0 | 2 | 3 | REQUEST CHANGES → 보완 | `resilience.NonRetryable`, policy context post-check, attempt state/bridge race 테스트를 추가 |
| Security | 0 | 3 | 4 | REQUEST CHANGES → 보완 | trusted-proxy fail-closed, parser error redaction, 기본 503 redaction, strict Bearer grammar/8KiB를 추가 |
| Operator/Ops | 0 | 2 | 3 | REQUEST CHANGES → 보완 | production bootstrap 순서, observer/readiness/runbook, benchmark ledger와 opt-in 재현 명령을 추가 |
| Developer/API | 0 | 2 | 5 | REQUEST CHANGES → 보완 | Gin-native `RateLimitOptions`, `AuthenticationError`, RFC 9457, source-level dependency boundary를 추가 |
| User/Caller | 0 | 2 | 4 | REQUEST CHANGES → 보완 | compile example/migration 설명, callback/error 분류, conformance recipe, 한·영 parity matrix 요구를 추가 |

## P1 remediation read-back

### Retry와 response side effect

response가 기록된 뒤 handler 오류가 발생하면 operation을 성공으로 숨기지
않는다. `resilience.NonRetryable`로 감싸 core retry는 재시도하지 않고
circuit-breaker는 failure로 기록한다. response가 아직 비어 있으면
`policyCtx.Err()`와 새 Gin error를 정상 policy 오류로 처리한다.

### Cancellation과 panic

request pointer는 정상 반환과 panic/Recovery 모두에서 `defer`로 복구한다.
policy context는 handler 반환 직후 다시 검사하며, context-capable JWT parser와
legacy parser의 best-effort 범위를 구분한다. parse 직후 cancellation이면 reader
저장과 downstream 실행을 금지한다.

### 보안과 오류 공개

trusted proxy는 caller가 제공한 server-established predicate만 사용하고 nil은
fail-closed다. JWT callback에는 token/parser 원문을 전달하지 않고
`AuthenticationError.Kind`만 제공한다. 기본 rate-limit/resilience backend 오류는
redacted RFC 9457 503 Problem이며 custom error handler의 공개 범위는 caller
책임이다.

### API와 운영

rate limit은 `net/http` callback을 public API로 누출하지 않는 Gin-native
`RateLimitOptions`를 사용한다. README에는 Recovery → request context →
authentication/rate-limit → route resilience 순서의 bootstrap, readiness,
trusted-peer, observer와 migration example을 추가한다. Gin은 단일 root module
graph에만 존재할 수 있고 source import는 `web/gin`에 한정한다.

### Benchmark와 범위

benchmark는 JWT parser path만 포함하고 JWKS network/provider는 독립 Issue #545로
명시한다. no-op/direct-core/bridge/full, serial/parallel, warm/cold, policy와
problem 분기를 분리하고 `ReportAllocs`, `-benchmem`, `-cpu 1,2,4`, 고정
`-count=5` 명령과 environment ledger/raw output/table/chart/use-case/caveat를
보존한다.

## Main integration verdict

- 현재 설계 read-back 기준 P0: **0**
- remediation 후 남은 P1 blocker: **0**
- P2: 구현 계획과 회귀 테스트로 추적
- 구현 진입 조건: `resilience.NonRetryable` core 변경, Gin-native options,
  redaction/fallback, race/conformance/benchmark 및 README 산출물을 계획에
  명시하고 TDD 순서로 실행한다.

## SPW-01~05 검토 기록

- SPW-01: maintainer/adapter consumer/운영자를 명시하고 공식 Gin, Issue,
  Makefile, core source evidence와 연결했다.
- SPW-02: public API, 오류/response, retry/cancellation, 테스트, benchmark,
  README와 수용 기준을 설계·리뷰 artifact에서 일치시켰다.
- SPW-03: 기술 식별자와 명령을 제외한 검토 서술은 자연스러운 한국어로
  정리했다.
- SPW-04: P1 finding마다 설계 변경 위치와 구현 검증 항목을 매핑했다.
- SPW-05: 최종 read-back에서 RFC 9457, #545 JWKS 분리, root module source
  isolation, `make test/race`, `make bench-web-gin` 계약을 재확인했다.

