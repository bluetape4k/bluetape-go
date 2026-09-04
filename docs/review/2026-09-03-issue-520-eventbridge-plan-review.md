# Issue #520 EventBridge 실행 계획 검토

## 범위

- 대상: `docs/superpowers/plans/2026-09-03-issue-520-eventbridge-plan.md`
- 게이트: Step 3-R (6개 관점 + main-session integration)
- 기준 설계: `docs/superpowers/specs/2026-09-03-issue-520-eventbridge-design.md`
- 기준 head: `2d130ed78a258751d5baf394f3a82cb0a7b31159`

현재 실행 계약에 따라 네이티브 독립 reviewer lane은 생성하지 않고 main
session이 각 관점을 분리해 검토했다. 이 문서는 구현 전 계획의 품질과
실행 순서를 검증하는 기록이며, 구현 테스트 결과를 대신하지 않는다.

## 6개 관점

| 관점 | P0 | P1 | 계획 검토 결과 |
|---|---:|---:|---|
| Performance | 0 | 0 | Task 4에서 single-entry만 사용하고 detail + source/type/bus overhead를 dispatch 전 계산한다. benchmark나 SDK retry를 범위에 넣지 않는다. |
| Stability | 0 | 0 | Task 2의 fake가 blocking/cancellation을 재현하고, Task 4가 preflight·dispatch 직전·response 직후 context 우선순위를 구현한다. malformed output matrix가 누락되지 않았다. |
| Security | 0 | 0 | Task 3의 sentinel/typed error와 redaction tests가 transport cause와 `ErrorMessage`를 분리한다. Task 5는 credential/topology/logging ownership을 문서화한다. |
| Operator/Ops | 0 | 0 | Task 5에 default bus, IAM, client lifecycle, timeout/retry, downstream idempotency, rollout/rollback 경계가 모두 있다. live AWS를 CI에 넣지 않는다. |
| Developer/API | 0 | 0 | SDK v2 `v1.47.0` direct module, compile assertions, immutable options, exact accessor behavior, Go docs와 no dependency churn이 파일 단위로 고정됐다. |
| User/Caller | 0 | 0 | Redis envelope field parity, `EventId` 비동일성, relay retry/dead-letter semantics, EN/KO README pair와 example이 수용 기준에 연결됐다. |

## Main-session integration

계획과 설계를 대조해 다음 연결을 확인했다.

1. `Options.MaxDetailSize`의 detail cap과 EventBridge 전체 entry의 256 KiB
   미만 제약이 Task 4의 동일한 preflight로 구현된다.
2. `Record.OccurredAt`, stable `EventID`, `IdempotencyKey`와 Redis Streams의
   field naming이 envelope task에서 같은 source를 사용한다.
3. `ErrPartialFailure`는 single entry의 `FailedEntryCount`와 per-entry code를
   보존하고, `ErrorMessage`는 error chain에 넣지 않는다는 설계와 일치한다.
4. 테스트는 live AWS 또는 Testcontainers에 의존하지 않으며, full repository
   Testcontainers gate 실패 시 changed package 증거를 별도로 남기도록 했다.
5. PR/merge/cleanup는 implementation과 분리된 Task 7 gate이며, fresh exact
   head approval 없이는 merge하지 않는다.

## 계획 품질 검사

| 검사 | 결과 |
|---|---|
| 파일 구조/책임 명시 | PASS — 16개 산출물의 책임과 write scope가 표로 고정됨 |
| TDD 순서 | PASS — fake/RED → sentinel/envelope → request/response GREEN → docs → verification |
| 타입 일관성 | PASS — `Client`, `Options`, `Publisher`, sentinels, `detailEnvelope` 이름이 task 간 일치 |
| 명령/기대 결과 | PASS — focused normal/race, vet, Make gates, `gh` exact-head gate가 구체적임 |
| 범위/rollback | PASS — #520 single-entry adapter와 docs만 변경, Kinesis/#522/provisioning은 후속 범위 |
| 미완성 표기 scan | PASS — `TBD`, `TODO`, 무정의 placeholder 표현 0건 |

## 최종 판정

P0=0, P1=0. Step 3-R PASS. 계획은 Task 2 RED 테스트와 Task 4 GREEN
구현으로 이동할 준비가 됐다. 구현 중 AWS SDK generated type 또는 existing
outbox contract가 계획과 충돌하면 해당 task를 먼저 멈추고 이 문서와 설계를
수정한 뒤 재검토한다.
