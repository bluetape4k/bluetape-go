# PostgreSQL Batch Checkpoint 교훈 (2026-07-14)

**Related issue:** #532
**Affected modules:** `batch`, `batch/sqlcheckpoint`

## L1: checkpoint progress는 consumed-input 경계다

### 문제

output count는 안전한 resume position이 아니다. processor는 input을 filter하거나 여러 output을
emit하거나 내부 retry를 하거나 chunk 일부만 소비한 뒤 실패할 수 있다. written output 기준으로
전진하면 source item을 replay하거나 skip할 수 있다.

### 결정

각 consumed input 뒤 checkpoint를 capture하고, 마지막 captured value는 해당 chunk의 business
output과 함께만 commit한다. empty final chunk는 commit하지 않으며, 실패한 provider call은
caller가 보유한 pending slice를 mutate할 수 없어야 한다.

### 향후 가드

batch persistence는 offset을 durable progress로 보기 전에 filtered item, partial output,
exact-multiple EOF, empty input, processor retry, provider mutation attempt를 테스트해야 한다.

## L2: insert와 update에는 서로 다른 CAS proof가 필요하다

### 문제

없는 checkpoint에는 비교할 stored revision이 없고, 기존 row는 stale writer를 거부해야 한다.
두 경로를 unconditional upsert로 처리하면 concurrent creation을 숨기거나 더 최신 resume
position을 덮어쓸 수 있다.

### 결정

expected revision zero에서만 성공하는 insert와, exact expected revision을 맞춘 뒤 increment하는
update를 사용한다. affected row가 0이면 전체 transaction을 `ErrCheckpointConflict`로 rollback한다.

### 향후 가드

provider conformance는 independent business key와 checkpoint CAS 직전 barrier로 정확한
winner/loser behavior를 증명해야 한다.

## L3: callback return은 transaction ownership을 증명하지 않는다

### 문제

SQL access가 있는 callback은 raw transaction control을 실행하거나 transaction을 aborted 상태로
남기거나 session state 변경 후 panic할 수 있다. Go return 성공만으로 provider가 usable
transaction을 여전히 소유한다는 사실은 증명되지 않는다.

### 결정

guarded query/exec session을 노출하고 fixed savepoint를 만든 뒤 checkpoint CAS 전에 ownership을
probe한다. PostgreSQL `25P02`는 `ROLLBACK TO SAVEPOINT`로만 복구한다. ownership을 증명할 수
없으면 `ErrAtomicityUnknown`에서 멈추고 rollback 성공을 추측하지 않는다.

### 향후 가드

raw BEGIN/COMMIT/ROLLBACK, aborted transaction, savepoint loss, cancellation,
connection loss, panic, competing actor를 테스트한다. ownership evidence는 fail-closed여야 한다.

## L4: commit unknown과 atomicity unknown은 복구가 다르다

### 문제

lost commit response는 provider-owned transaction이 commit됐을 수 있음을 뜻하지만, ownership
failure는 business와 checkpoint attribution 자체가 unknown일 수 있음을 뜻한다. 둘 중 어느
경우든 자동 replay하면 business effect가 중복될 수 있다.

### 결정

provider commit ambiguity는 stable operation `OperationCommit`을 가진 `ErrCommitUnknown`으로
분류한다. 증명되지 않은 callback ownership은 `ErrAtomicityUnknown`으로 분류한다. 둘 다 step을
멈춘다. caller는 key를 quiesced 상태로 유지한다. commit-only unknown은 bounded fresh
atomic-writer `Load`를 쓰고, atomicity unknown은 수동 business/checkpoint reconciliation이 필요하다.

### 향후 가드

public example은 `Step.Run`에서 시작하고 `Report.Err`를 inspect하며 same-key exclusivity를
보존하고 automatic callback replay를 금지해야 한다.

## L5: panic behavior는 supervisor contract에 속한다

### 문제

cleanup만 위해 callback panic을 recover하면 원래 panic identity를 우연히 바꾸거나, 값을 error
string에 노출하거나, 증명되지 않은 transaction boundary 뒤에서 resume할 수 있다.

### 결정

ownership과 rollback이 증명되면 원래 값을 그대로 re-panic한다. 그렇지 않으면 original value로
가는 유일한 경로가 trusted accessor인 sanitized `AtomicityPanic`을 발생시킨다. trusted top-level
supervision이 quiesce와 reconciliation을 수행한다.

### 향후 가드

string, error, nil, typed-nil panic value를 모두 다루고 identity와 default-format redaction을
함께 assert한다.

## L6: source compatibility에는 외부 unkeyed fixture가 필요하다

### 문제

exported options struct에 field를 추가하면 package 내부에서는 compile되지만 unkeyed composite
literal을 쓰는 caller를 깨뜨릴 수 있다.

### 결정

legacy options layout은 바꾸지 않고, new type과 new constructor로 atomic path를 추가한다.
외부 unkeyed `StepOptions` fixture를 `make test`, 따라서 `make ci`의 일부로 compile한다.

### 향후 가드

additive Go API는 다른 package에서 compatibility를 테스트한다. package-local test는 모든
source-level caller shape를 대표할 수 없다.

## L7: cleanup과 pool release에는 hostile small-pool test가 필요하다

### 문제

이미 취소된 context에 의존하는 rollback 또는 cleanup은 connection을 붙잡아 둘 수 있다. 큰 pool에서는
leak을 놓치기 쉽지만, connection 하나에서는 다음 operation이 즉시 막힌다.

### 결정

deterministic transaction cleanup을 사용하고, secret을 렌더링하지 않은 채 rollback cause를
보존한다. 알려진 callback failure가 one-connection pool을 release한다는 점을 증명한다. reader
close는 cancellation을 제거한 상태로 시도하고 error는 joined 처리한다. outer shutdown deadline은
여전히 caller-owned다.

### 향후 가드

모든 SQL provider는 one-connection failure/recovery test를 포함하고, library cleanup guarantee와
caller-owned shutdown budget을 구분해야 한다.

## L8: `IF NOT EXISTS`에는 catalog, ACL, role-graph proof가 필요하다

### 문제

migration syntax는 기존 객체가 expected columns, constraints, owner, ACLs,
RLS/policy/trigger state, role topology를 가진다는 증거가 아니다. runtime에 grant된 role만
확인하면 위험한 inbound edge도 놓친다.

### 결정

schema application은 caller-owned로 두고 exact pre-grant 및 post-grant validation을 요구한다.
승인된 deployer-to-owner membership만 허용하고, 모든 runtime edge, privileged owner/runtime
attribute, unrelated grant, column ACL, hostile catalog drift를 거부한다.

### 향후 가드

모든 fixed-schema SQL provider에는 normal fixture와 one-property-hostile catalog fixture를
붙이고, `pg_auth_members` 양방향을 포함한다.

## L9: isolation은 public provider contract다

### 문제

database 또는 role default의 Repeatable Read를 상속하면 concurrent CAS behavior가 바뀌고,
문서화된 checkpoint conflict 대신 generic transaction failure가 표면화됐다.

### 결정

provider commit은 명시적으로 Read Committed에서 시작한다. ambient default는 무시되고, callback은
해당 isolation에서 올바르게 동작해야 하며, codec과 callback은 concurrency-safe여야 하고,
business semantics가 요구하면 caller가 same-key work를 serialize해야 한다고 문서화한다.

### 향후 가드

concurrency integration test는 ambient isolation을 일부러 바꾸고도 provider의 문서화된 결과를
요구해야 한다.

## L10: capacity 작업은 evidence-based로 남아야 한다

### 문제

correctness test는 atomicity와 failure semantics를 세우지만 production throughput, hot-key
latency, WAL growth, autovacuum behavior, pool sizing을 증명하지 않는다.

### 결정

performance positioning은 qualitative하게 유지하고 benchmark/capacity matrix는 issue #560으로
미룬다. runbook은 universal number 대신 deployment-owned canary threshold와 database telemetry를
요구한다.

### 향후 가드

conformance timing을 capacity claim으로 바꾸지 않는다. 전용 benchmark issue에서 정확한 provider
workload와 topology를 benchmark한다.
