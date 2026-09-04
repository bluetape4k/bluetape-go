# #573 plan review (Step 3-R)

## 검토 범위

검토 대상은 승인 spec과 `docs/superpowers/plans/2026-09-04-issue-573-redis-cache-plan.md`다.
현재 repository layout, Redis client method signatures, shared key/error/TTL
helpers와 Testcontainers fixture를 기준으로 ordering·coverage를 점검했다.
Native lane slot은 기존 완료 agent 기록으로 소진되어 여섯 관점은 main
session의 독립 pass로 수행하고 integration은 같은 세션이 담당한다.

## 여섯 관점 결과

| 관점 | 확인 내용 | P0 | P1 | 조치 |
|---|---|---:|---:|---|
| Performance | single-key command/Lua, no scan/worker, concurrent CAS race 증거가 Task 3–6에 있음 | 0 | 0 | 없음 |
| Stability | validation→fake→Bucket→MapCache→container 순서, cancellation/unknown/cleanup가 있음 | 0 | 0 | 없음 |
| Security | typed-nil, codec/provider redaction, key/hash-tag and caller-owned config가 Task 2/5에 있음 | 0 | 0 | 없음 |
| Operator/Ops | persistence/eviction/ACL/TLS, readiness/cleanup, rollback와 PR evidence가 문서/Task 5–7에 있음 | 0 | 0 | 없음 |
| Developer/API | no dependency, generic method set, existing helper reuse와 concrete commands가 명시됨 | 0 | 0 | 없음 |
| User/Caller | locale README, examples, durable/near/stampede distinction, unsupported operations가 포함됨 | 0 | 0 | 없음 |

## Required-check audit

1. 모든 spec requirement가 Task 2–5 코드/test/doc와 Task 6 verification에 매핑된다.
2. RED test/fake가 implementation보다 앞서며, Bucket 공통 contract 이후
   MapCache specialization 순서가 의존성을 만족한다.
3. Testcontainers는 fake/unit/race 뒤 직렬 실행하고 환경 미충족을 명시적으로
   PENDING으로 기록한다.
4. public API의 Korean Go doc, README locale pair, examples와 root index가
   포함된다. 신규 module/BOM/coverage registration은 없으므로 N/A다.
5. cross-key clear/RedisJSON/eviction/near-cache integration은 후속으로
   분리되며 rollback은 PR revert다.

## Main-session integration 판정

구현 가능한 task 누락과 P0/P1을 발견하지 못했다. `Client` narrow subset은
`*redis.Client`와 fake가 만족하는지 compile test로 확인하고, Lua result parser는
provider가 반환할 수 있는 `int64`/`string`/`[]byte` 변형을 fail closed로
처리한다는 보완 조건을 구현 단계에 반영한다. `P0=0, P1=0`으로 Step 3-R을
통과한다.
