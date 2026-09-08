# 0.22.0 구현 계획 검토

## 검토 범위와 기준

- 대상: `docs/superpowers/plans/2026-09-06-issue-550-milestone-0220-plan.md`
- 기준 설계: `docs/superpowers/specs/2026-09-06-issue-550-milestone-0220-design.md` (commit `1e5089b`)
- 검토 방식: 사용자가 요청한 inline 순차 구현을 보존하기 위해 main session이
  여섯 관점과 통합 검토를 각각 독립 체크리스트로 수행했다. 별도 subagent
  provenance나 독립 모델 판정으로 주장하지 않는다.
- 대상 이슈: `#547`, `#550`, `#551`, `#552`, `#554`, `#561`

## 관점별 결과

| 관점 | 검토 내용 | 초기 finding | 조치 | 최종 |
|---|---|---|---|---|
| 성능 | 값 encoding, DB round-trip, query round-trip, serial container, race/stress 명령 | P2: container와 full race 비용이 크지만 bounded 명령이 명확하지 않음 | Task 7.2–7.4에 targeted→serial heavyweight→full 순서를 고정 | P0=0/P1=0 |
| 안정성 | context/timeout, response·rows·channel·container close, readiness/flaky retry | P1: image/serializer 미확정 상태에서 integration을 성공으로 오인할 위험 | Task 0.2a와 Task 4.1/5.1의 RED feasibility 및 `PENDING` 규칙을 추가 | P0=0/P1=0 |
| 보안 | SQL bind/identifier, HTTP body limit, credential/public endpoint, error/log redaction | P1 후보: spatial SQL fragment와 provider payload 경계가 구현 단계에서 흐려질 수 있음 | Task 1/2의 identifier·bind 규칙, Task 3–5의 bounded/redacted 계약을 각 테스트에 매핑 | P0=0/P1=0 |
| 운영 | caller-owned lifecycle, logger, readiness, rollback, issue/PR/merge gate | P2: mutable existing DB tag가 재현성을 떨어뜨릴 수 있음 | Task 0.2a에 fixture-local immutable digest와 mismatch `PENDING`을 명시 | P0=0/P1=0 |
| 개발자/API | Go-native package 경계, zero value/error/context, dependency order, plan 명령 | P1: 공간 encoding이 `WKB 또는 WKT`로 열려 있어 구현자가 임의 선택 가능 | Task 0.2a에서 EWKB point + explicit SRID와 engine read path를 고정 | P0=0/P1=0 |
| 사용자/caller | README locale, examples, unsupported capability, Nominatim policy, migration | P2: `geo`/`graph` core non-goal과 새 package 선택 기준을 반복 설명해야 함 | Task 6 및 각 Task 문서에 index/non-goal/운영 책임과 compile example을 명시 | P0=0/P1=0 |

## 통합 검토

1. 다섯 구현 slice는 independent package로 남고 `integration`과 `delivery`만
   후속 dependency를 갖는다. `#548/#555/#553` 재작업은 계획에 없다.
2. 구현 순서는 PostGIS → MySQL → MariaDB → geocoding → FalkorDB → Gremlin이며,
   실제 DB/컨테이너는 `-p 1` 직렬이다. 한 slice의 failure가 다른 slice의
   abstraction merge를 유발하지 않는다.
3. 범용 spatial/graph/dialect abstraction, AGE/Neptune/cloud credential,
   benchmark parity, public Nominatim endpoint는 명시적 non-goal이다.
4. PR은 한 개지만 Step 8에서 package slice별로 6 관점 리뷰를 수행한다.
   merge는 Step 10의 fresh exact-head 승인 뒤에만 가능하다.
5. 계획의 모든 spec requirement는 마지막 traceability 표에서 task와 fresh
   command로 연결된다. dependency/serializer/image처럼 현재 코드로 확정할
   수 없는 부분은 feasibility RED와 `PENDING` stop condition으로 드러난다.

## 계획 자체 검증

- `SPW-01`: artifact 목적, 독자, 이슈·spec·repository source·official source와
  unsupported claim을 식별했다 — **PASS**
- `SPW-02`: 정확한 files, 순서, RED/GREEN, tests, docs, hazards, rollback,
  PR/merge approval gate를 포함했다 — **PASS**
- `SPW-03`: 한국어 technical register, identifier/command/URL 보존과
  terminology audit를 적용했다 — **PASS**
- `SPW-04`: spec-to-task-to-command traceability와 image/serializer unknown을
  명시했다 — **PASS**
- `SPW-05`: 제목·표·code span·checkbox·명령을 마지막으로 read-back했다 — **PASS**

## 최종 판정

`P0=0`, `P1=0`, `P2=0` (모두 계획에 반영), `P3=0`.
계획은 승인된 설계와 한 통합 PR/한 merge 제약을 보존하면서 구현 가능한
순서와 증거를 제공한다. 이 artifact와 plan을 커밋한 뒤에야 Task 1 RED에
진입한다.
