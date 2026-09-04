# Issue #520 EventBridge 설계 검토

## 범위와 기준

- 대상 문서: `docs/superpowers/specs/2026-09-03-issue-520-eventbridge-design.md`
- 게이트: Step 2-R (performance, stability, security, operator/Ops, developer/API, user/caller 6개 관점 + main-session integration)
- 기준 head: `2d130ed78a258751d5baf394f3a82cb0a7b31159`
- 요구사항 근거: live issue [#520](https://github.com/bluetape4k/bluetape-go/issues/520), parent [#517](https://github.com/bluetape4k/bluetape-go/issues/517), AWS research gate

네이티브 독립 reviewer lane은 현재 실행 계약상 생성하지 않았으며, main
session이 여섯 관점을 독립적으로 read-only 수행하고 integration을 소유했다.

## 독립 관점 검토

| 관점 | P0 | P1 | 검토 결과 |
|---|---:|---:|---|
| Performance | 0 | 0 | 단일 entry만 사용하고 detail 및 EventBridge entry size를 dispatch 전 bounded preflight한다. retry/batching/benchmark를 추가하지 않아 비용이 예측 가능하다. |
| Stability | 0 | 0 | nil/typed-nil client, invalid record, malformed output, pre/post-dispatch cancellation을 호출 순서와 함께 정의했다. `Publisher` immutable 및 fake request isolation을 race test로 고정한다. |
| Security | 0 | 0 | detail, credentials, bus/source, AWS `ErrorMessage`를 error/log에 노출하지 않는 redaction 계약과 caller-owned credential boundary가 명시됐다. payload는 검증된 JSON만 전송한다. |
| Operator/Ops | 0 | 0 | default bus 생략 의미, topology/IAM/client lifecycle 소유권, downstream idempotency와 retry/dead-letter 경계를 README에 남긴다. live AWS와 provisioning은 명시적으로 제외했다. |
| Developer/API | 0 | 0 | SDK method subset, compile assertion, exact string preservation, accessors, bounded options, `errors.Is`/typed error surface가 정의됐다. 새 broker abstraction이나 dependency churn은 금지한다. |
| User/Caller | 0 | 0 | `sqloutbox.Publisher` 계약, stable `event_id`/`idempotency_key`, EventBridge `EventId`와의 구별, English/Korean README parity, fake-first 사용법이 수용 기준에 연결됐다. |

## 통합 검토 및 수정

초기 통합 read에서는 두 가지 경계를 명시적으로 보강했다.

1. EventBridge 문서의 “256 KB 미만”은 detail만의 크기가 아니라 source,
   detail type, optional bus metadata를 포함한 entry size로 해석해야 하므로
   설계에 metadata overhead 합산 preflight를 추가했다.
2. SDK request `Time`은 `audit/sqloutbox.Record.OccurredAt`를 사용하도록
   고정해 Redis Streams envelope와 outbox record의 시간 source를 일치시켰다.

그 외에는 scope creep, public API ambiguity, cancellation 우선순위,
single-entry failure mapping의 P0/P1 결함을 발견하지 못했다. `FailedEntryCount`
및 per-entry result의 모순은 `ErrMalformedOutput`, entry failure는
`ErrPartialFailure`로 결정론적으로 매핑하며, response `ErrorMessage`는
error chain에 넣지 않는다.

## 검토 체크리스트

| 체크 | 판정 | 증거 |
|---|---|---|
| SPW-01 요구사항 | PASS | live #520 범위·수용 기준과 #512 연구 gate를 설계 source ledger에 기록 |
| SPW-02 설계 | PASS | narrow client, immutable publisher, bounded detail, failure/cancellation contract |
| API/ABI 영향 | PASS | 새 child package와 direct EventBridge module만 추가; 기존 `sqloutbox`/Redis API 불변 |
| 데이터/보안 | PASS | stable identity 보존, raw AWS message/detail redaction, caller-owned credentials |
| 운영/롤백 | PASS | provisioning/live rollout 제외, commit revert와 기존 relay retry 경계 명시 |
| 문서/locale | PASS | parent README 두 locale와 child README pair를 구현 단계에서 동기화하도록 고정 |

## 최종 판정

P0=0, P1=0. Step 2-R은 PASS이며 implementation plan 작성으로 진행한다.
계획에서는 정확한 AWS module version, RED test 순서, error redaction 구현,
locale parity, targeted/full CI와 remote PR gate의 증거 위치를 파일 단위로
고정해야 한다.
