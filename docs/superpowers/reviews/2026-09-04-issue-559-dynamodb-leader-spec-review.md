# #559 spec review (Step 2-R)

## 검토 범위와 방법

검토 대상은 `docs/superpowers/specs/2026-09-04-issue-559-dynamodb-leader-design.md`와
그 문서가 참조하는 현재 `leader`, `leader/sql`, `leader/redis`,
`leader/leadertest`, `dynamodb/batchwrite` 구현이다. 외부 근거는 2026-09-04에
확인한 AWS 공식 PutItem/UpdateItem/Condition expressions/Working with items와
Go SDK v2 문서다. Native lane slot이 기존 완료 agent 기록으로 소진되어 이번
review는 main session이 여섯 관점을 독립 pass로 수행하고 integration을 소유한다.

## 여섯 관점 결과

| 관점 | 확인 내용 | P0 | P1 | P2/P3 |
|---|---|---:|---:|---:|
| Performance | single-item conditional write, bounded retry, one ticker, strong read probe의 round-trip과 deadline이 명시됨 | 0 | 0 | 0 |
| Stability | generation/cancel/done/resigning, post-dispatch cancellation, renewal/resign cleanup과 probe failure가 정의됨 | 0 | 0 | 0 |
| Security | narrow client, alias condition, typed-nil, raw token/provider message redaction, caller-owned credentials가 정의됨 | 0 | 0 | 0 |
| Operator/Ops | TTL은 cleanup only, IAM actions, capacity/error/runbook, logger와 rollback 경계가 정의됨 | 0 | 0 | 0 |
| Developer/API | 기존 `leader.Elector`/`leader.Options`를 유지하고 DynamoDB 고유 option을 분리, zero-value 계약과 fake API가 명시됨 | 0 | 0 | 0 |
| User/Caller | schema, consistency, clock skew, cleanup retry, unsupported Global Tables/fencing/ORM과 example 요구가 명시됨 | 0 | 0 | 0 |

## Main-session integration

- `Campaign`의 normal contention과 provider error를 분리하고, own-token
  strongly consistent probe만 commit을 복구한다는 invariant를 확인했다.
- TTL seconds는 조기 삭제를 막도록 올림하지만 leader correctness에는 쓰지
  않는다는 문장과 test 요구가 서로 일치한다.
- `ErrCommitUnknown` 뒤 같은 elector `Resign` retry와 `ErrCleanupPending`
  상태 전이가 API·failure matrix·test contract에 반복되어 누락이 없다.
- provisioning, IAM 정책 적용, credential/config, retry policy, live AWS는
  caller/operator 범위로 남겼고, 일반 CI는 fake-first다.
- 문서/API 이름과 예상 파일 경로는 plan task와 일치한다. CHANGELOG/release
  tag는 이 issue의 implementation 범위가 아니므로 N/A로 기록한다.

## 판정

`P0=0, P1=0`으로 Step 2-R을 통과한다. P2/P3 수정 요구는 없으며, 실제
구현에서 request map deep-copy, context response checkpoint, strong-read
`ConsistentRead=true`, raw error redaction을 반드시 유지한다.
