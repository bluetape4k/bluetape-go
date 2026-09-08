# Issue #555 구현 계획 검토

## 검토 범위와 근거

- 대상 issue: <https://github.com/bluetape4k/bluetape-go/issues/555>
- 승인 spec commit: `90f544a4a16a573e5e8878edf48ae9133d5eca6d`
- 계획 작성 기준 commit: `6beb00fa7c9fce36a7e026e90a4c103409b876a1`
- 검토 대상:
  `docs/superpowers/plans/2026-09-06-issue-555-graph-backend-conformance-plan.md`
- 검토 대상 SHA-256:
  `0ab1b89b9a599da137c1f80f59aee907ff87fa0a12615fca1344175f7e1dd309`
- 근거: 승인 spec, live issue metadata, `graph`, `graph/neo4j`,
  `internal/testcleanup`, 기존 provider benchmark와 shared test-support pattern
- 명령 범위: 계획과 저장소를 읽고 Markdown·용어·scope를 검증했다. 구현,
  Testcontainers와 Go test는 아직 실행하지 않았다.

## 최초 Step 3-R 결과와 조치

여섯 독립 관점의 중복 finding은 main integration에서 하나의 lifecycle 또는 evidence
결함으로 정규화했다.

| 우선순위 | Lens | 근거 | 조치 | 상태 |
| --- | --- | --- | --- | --- |
| P1 | performance | Logical query submission과 Bolt retry, provider 반복 process와 benchmark/evidence 의미가 섞였다. | Provider별 request-builder counter, logical submission과 wire retry 구분, 세 개의 독립 10분 process와 phase duration을 고정했다. | 해결 |
| P1 | stability | Timeout/cancellation 완료 경쟁, `Started` 순서, cleanup/close join과 partial resource ownership이 불완전했다. | `call`의 cancellation precedence, bidirectional `Started`, join-before-cleanup, scenario 선행 defer와 partial container 사전 등록을 고정했다. | 해결 |
| P1 | stability | Cancellation callback이 `CaseTimeout` deadline을 받지 않았고 pre-canceled factory 계약이 모순됐다. | `context.WithTimeout(parent, cfg.CaseTimeout)`과 deadline test를 추가하고 pre-canceled parent는 factory callback 0회·context error 보존으로 통일했다. | 해결 |
| P1 | security | Metadata, query, URI, credential, callback panic과 terminator panic이 진단 경계에서 노출될 수 있었다. | Strict allowlist, parameter binding, digest-only image log, 전 phase sanitizer와 panic sentinel을 추가했다. | 해결 |
| P1 | operator/Ops | Digest/readiness, exact rollback, exact-head CI/Nightly와 retry evidence가 부족했다. | Immutable image, bounded readiness, migration SHA/revert, run ID·SHA·job·artifact와 first-attempt 판정을 고정했다. | 해결 |
| P1 | developer/API | 공개 test-support API의 zero/partial config, callback 문서, helper 정의 순서와 example 완결성이 부족했다. | 모든 exported symbol Go doc, keyed `DefaultConfig`, helper 선행 정의, external compile example와 executable harness test를 추가했다. | 해결 |
| P1 | user/caller | Capability, logical-key traversal, lifecycle owner와 실패 진단 사용법이 호출자 관점에서 불완전했다. | Stateful external example, capability matrix, fixed provider/phase/status/category/timeout/duration log와 cleanup ownership을 고정했다. | 해결 |

## 최종 영향 lane 검토

| Lens | 최종 결과 | 증거 |
| --- | --- | --- |
| performance | `P0=0 P1=0` | Exact request builder, logical submission counter, bounded result, 독립 10분 provider process와 duration evidence를 read-back했다. |
| stability | `P0=0 P1=0` | 독립 최종 재검토에서 operation deadline, pre-cancel contract, `Started`, join, scenario cleanup, partial container와 exact rollback을 승인했다. |
| security | `P0=0 P1=0` | 독립 재검토 뒤 terminator panic도 main integration에서 고정 sentinel과 secret 비노출 test에 연결했다. |
| operator/Ops | `P0=0 P1=0` | `lane timed out; main integration fallback performed`. Digest/readiness, bounded cleanup, phase telemetry, exact rollback, CI/Nightly retry 판정을 main session이 read-back했다. |
| developer/API | `P0=0 P1=0` | `lane timed out; main integration fallback performed`. Helper 선행 정의, `t.Context()`, exported docs, example, panic/error 경계를 main session이 read-back했다. |
| user/caller | `P0=0 P1=0` | 독립 재검토에서 네 개의 caller-facing P1이 모두 해소됐음을 확인했다. |
| main integration | `P0=0 P1=0` | Spec coverage, Task 0 위험 예측, fake-before-Docker 순서, migration과 delivery stop condition을 확인했다. |

## P2/P3 처분

- Scenario별 `t.Run`은 strict ordered lifecycle과 하나의 provider fixture cleanup 순서를
  흩뜨릴 수 있어 적용하지 않았다. Provider top-level subtest, 고정 phase log와 anchored
  provider 재실행 명령이 실패 격리를 담당한다.
- Fake harness microbenchmark는 DB callback 지연을 대표하지 않고 compiler-dependent
  수치를 API gate로 만들 수 있어 도입하지 않았다. Bounded materialization, logical query
  counter, provider phase duration과 기존 mapping benchmark를 유지한다.
- 그 밖의 P2는 계획에 반영했다. 남은 P3는 구현 blocker가 아니다.

## Writer DoD

- `SPW-01 PASS`: 독자는 test-support API 구현자·provider adapter 작성자·reviewer이며,
  승인 spec과 live repository evidence를 고정했다.
- `SPW-02 PASS`: Writing Plans header, checkbox task, exact path·명령·RED/GREEN·commit,
  rollback과 implementation handoff를 확인했다.
- `SPW-03 PASS`: 한국어 기술 문장에 KO-01부터 KO-07까지 적용하고 식별자·명령·수치를
  보존했다.
- `SPW-04 PASS`: finding을 lifecycle, provider, migration, docs, CI/Nightly evidence와
  spec coverage 표에 다시 연결했다.
- `SPW-05 PASS`: heading, 표, 목록, code fence, link와 stop condition을 최종 read-back했다.

### Korean naturalness

- `KO-01 PASS`: SHA, timeout, count, command와 판정 의미를 고정했다.
- `KO-02 PASS`: 추상적 안정성 주장을 callback order, counter와 exact command로 바꿨다.
- `KO-03 PASS`: 번역투와 반복 문장을 제거하고 긴 lifecycle 문장을 단계별로 나눴다.
- `KO-04 PASS`: callback, operation context, cleanup, close, provider 용어를 일관되게 썼다.
- `KO-05 PASS`: 검토 문서에 불필요한 홍보 문구나 유머를 넣지 않았다.
- `KO-06 PASS`: plan, review, example, README locale pair와 공개 진단 요구를 함께 확인했다.
- `KO-07 PASS`: 문맥 용어 audit에서 status/category/timeout과 cancellation/deadline 차원을
  뒤바꾼 표현을 찾지 못했다.

## 판정과 남은 gate

Step 3-R 최종 판정은 `PASS`, `P0=0 P1=0`이다. 구현은 아직 시작하지 않았고, plan
승인과 실행 방식 선택 전에는 coordinator의 `plan`·`plan-review` gate도 `PENDING`으로
유지한다. PR, exact-head CI, Nightly, merge, tag와 publication은 각각 별도 gate다.
