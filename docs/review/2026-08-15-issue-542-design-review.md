# Issue #542 설계 검토

## 검토 범위와 기준

- 대상: `docs/superpowers/specs/2026-08-15-issue-542-http-conformance-design.md`
- 이슈: [#542](https://github.com/bluetape4k/bluetape-go/issues/542)
- 기준: [#508](https://github.com/bluetape4k/bluetape-go/issues/508),
  [research gate](https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-ecosystem-parity-research-gate.md)
- 현재 코드: `web/problem.go`, `web/context.go`, `resilience/http.go`,
  `ratelimit/http.go`, `resilience/* OnEvent`, `ratelimit/ratelimittest`
- 검토 방식: performance, stability, security, operator/Ops, developer/API,
  user/caller의 여섯 관점을 독립적으로 읽고 main-session에서 통합했다.
- 판정: P0=0, P1=0. 구현 전 plan 단계에서 P2/P3 기록을 반영한다.

## 독립 관점 결과

| 우선순위 | 관점 | 근거 | 처분 |
| --- | --- | --- | --- |
| P3 | performance | 새 production hot path가 없고, scenario를 순차 실행하며 benchmark를 범위에서 제외했다. | 별도 수정 없음. `go test`와 race 결과만 요구한다. |
| P2 | stability | 임의 handler는 timeout 뒤에도 goroutine을 무한히 실행할 수 있다. | 명세에 buffered result channel, request cancel, cleanup bounded drain, 기본 2초 상한, timeout을 성공으로 바꾸지 않는 실패 의미를 추가했다. 구현에서 두 번째 상한을 지킨다. |
| P3 | security | trusted proxy predicate가 false일 때 auth/trace 값을 읽지 않고, `X-Forwarded-For`를 기본 key로 신뢰하지 않는다. global logger/default transport 변경도 제외했다. | 현재 설계 유지. negative case를 plan에 고정한다. |
| P2 | operator/Ops | CI에서 실패한 계약 이름과 timeout 종류를 식별해야 하며, test-only package에는 runtime rollback이 없다. | scenario 이름과 `cancellation/timeout` 실패 분류를 명시했다. 운영 rollback은 N/A로 기록한다. |
| P2 | developer/API | 초기 설계가 runner API를 추상적으로 적을 위험이 있었다. | `Adapter`, `Scenario`, `Observation`, `Run`의 필드와 입력 검증을 명세에 고정했다. `RoundTripper`는 별도 경계로 유지한다. |
| P2 | user/caller | framework adapter 사용자는 지원 범위와 incoming request body 소유권을 오해할 수 있다. | README와 명세에 test-only 성격, `net/http` 우선, framework adapter 비포함, owner별 close 계약을 명시한다. |

## Main-session 통합 검토

1. **범위 경계:** `webtest`는 test support package이며 production middleware나
   Gin/Echo/Fiber dependency를 추가하지 않는다. `RoundTripper` body close는
   `CloseTracker` fixture를 공유하고 transport runner로 과도하게 일반화하지
   않는다.
2. **실패 경로:** 사전 취소, in-flight 취소, timeout, retryable response body
   close, panic/error policy, 빈 rate-limit key, proxy spoofing을 각각 관찰할
   수 있다. 무제한 sleep이나 global recorder는 허용하지 않는다.
3. **호환성:** 기존 public API 변경 없이 새 import path만 추가하고, 기존
   package 테스트가 harness를 선택적으로 사용한다. Go 1.26.3와 standard
   library만 전제로 한다.
4. **증거 무결성:** 기준선은 새 worktree에서 `go test -count=1 ./...`가
   통과했다. 설계의 behavior claim은 현재 소스와 live Issue/#508/wiki
   문서에 대응하며, benchmark 수치는 주장하지 않는다.
5. **문서·릴리스:** 루트 English/Korean README와 `webtest` README를
   동기화한다. test-only 패키지이므로 release/runtime migration은 N/A다.

## 수정 후 재검토 결과

- `Adapter`의 함수형 형태, `Scenario`의 필드와 입력 검증, `Run`의 timeout·취소
  의미를 다시 읽었다.
- `Observation`의 필드와 복사 규칙을 다시 읽어 assertion이 recorder 내부를
  오염시키지 않음을 확인했다.
- `RoundTripper`를 Handler runner와 분리하고 `CloseTracker` 소유권을 명시했다.
- 여섯 관점에서 P0/P1 수정 요구가 남아 있지 않다.

## SPW-01~05 기록

- **SPW-01 — PASS:** review artifact의 독자, 목적, 이슈/상위 조사, 현재 소스,
  검토 범위와 미지원 영역을 고정했다.
- **SPW-02 — PASS:** scope/basis, 여섯 관점 결과, severity, evidence,
  disposition, integration verdict와 gaps를 포함했다.
- **SPW-03 — PASS:** `korean-naturalness-checklist.md`를 적용해 같은 개념에
  `harness`, `adapter`, `scenario`, `관찰값`, `소유권`을 일관되게 사용했다.
- **SPW-04 — PASS:** 설계 수정 전후에 현재 Go 소스, issue, research gate,
  기준선 테스트 사실을 대조했다.
- **SPW-05 — PASS:** 표, 제목, 링크, 명령, 우선순위, P0/P1 판정과 수정 후
  재검토 문장을 저장 후 다시 읽었다.
