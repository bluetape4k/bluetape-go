# DynamoDB 조건부 쓰기 리더 provider 설계

## 상태와 범위

- 상태: 사용자가 승인한 0.20.0 Type A 실행의 설계 기준
- 작업 이슈: [#559](https://github.com/bluetape4k/bluetape-go/issues/559)
- 부모 이슈: [#526](https://github.com/bluetape4k/bluetape-go/issues/526)
- 관련 이슈: #527, #538
- 대상 package: `leader/dynamodb` (`package dynamodbleader`)
- 기준 head: `352c0bdbbef7ef41362027e3ecb591ed38be1c32`
- 실행 경계: caller-owned AWS client와 table을 사용한다. provisioning, credential
  loading, retry policy, live AWS 호출은 이 PR이 소유하지 않는다.

이 문서는 #559의 API와 lifecycle을 고정한다. DynamoDB TTL 삭제 시점은
correctness 근거가 아니며, Global Tables와 fencing token은 후속 설계로
남긴다. 구현과 테스트는 Go-native narrow client, fake-first, 기존
`leader.Elector` 계약을 따른다.

## 근거와 source ledger

| 근거 | 결정에 사용한 내용 |
|---|---|
| `leader/elector.go`, `leader/options.go`, `leader/errors.go` | 공통 `Elector`, lease 옵션 검증, `ErrCommitUnknown`/`OperationError`와 redaction 계약 |
| `leader/sql/lifecycle.go`, `leader/redis/elector.go` | bounded campaign, renewal goroutine, owner-token cleanup, commit-unknown reconciliation의 로컬 기준 |
| `leader/leadertest/harness.go`, `runner.go` | acquire/경합/취소/renew/resign/expiry/redaction 공통 conformance 시나리오 |
| `dynamodb/batchwrite/batchwrite.go` | AWS SDK v2 method subset, caller-owned client, `%w` 오류 wrapping 관례 |
| [AWS PutItem API](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_PutItem.html) | `attribute_not_exists` 조건부 Put, `ConditionalCheckFailedException`, key 제약 |
| [AWS Condition expressions](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.ConditionExpressions.html) | 전체 primary key로 식별한 item에 조건식을 평가하고 false이면 write를 거부하는 의미 |
| [AWS UpdateItem API](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_UpdateItem.html) | 조건부 takeover/renewal update와 condition failure 의미 |
| [AWS Working with items](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/WorkingWithItems.html) | 조건부 write 실패도 capacity를 사용할 수 있고 TTL은 별도 cleanup이라는 운영 주의 |
| [AWS SDK for Go v2 DynamoDB](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb) | `PutItem`, `UpdateItem`, `DeleteItem`, `GetItem` 호출 시그니처 |

현재 `go.mod`에 DynamoDB service module `v1.57.3`이 이미 있으므로 새
dependency는 추가하지 않는다. AWS 공식 문서는 2026-09-04에 확인했고, live
credential을 요구하는 테스트는 일반 CI에서 실행하지 않는다.

## 문제와 목표

AWS 배포에서 별도 coordinator 없이 DynamoDB item의 조건부 쓰기를 이용해 한
group의 leader를 선택할 수 있는 provider가 없다. 목표는 다음과 같다.

1. `leader.Elector`와 동일한 `Campaign`, `Resign`, `IsLeader`, `Leader` API를 제공한다.
2. 조건부 Put과 expired-owner conditional Update로 최초 획득과 stale takeover를 원자적으로 처리한다.
3. lease deadline은 epoch milliseconds로 저장하고, 활성 leader 판단은 strongly consistent `GetItem`과 deadline 비교로 수행한다.
4. renewal/resign의 owner token 조건과 bounded cleanup을 보장한다.
5. transport 오류, conditional contention, output-plus-error, cancellation을 구분하고 fake에서 검증한다.

## 선택지와 경계

| 선택지 | 결정 | 이유 |
|---|---|---|
| `New(client, tableName, leader.Options, ...Option)` | 채택 | 기존 leader provider의 공통 옵션을 그대로 쓰고 DynamoDB 고유 설정만 option으로 확장한다. |
| 범용 DynamoDB repository/ORM | 거부 | #559는 한 item lease provider이며 repository abstraction은 범위를 넓히고 lifecycle 소유권을 흐린다. |
| TTL 만료를 leader 해제의 근거로 사용 | 거부 | DynamoDB TTL은 비동기 cleanup이다. correctness는 `lease_until_ms`와 strongly consistent read에만 의존한다. |
| backend clock/Global Tables/fencing token | 연기 | SDK 호출마다 caller clock을 계산하고, clock skew·다중 리전 fencing은 별도 합의가 필요하다. |
| package 내부 retry worker/logger/credential loader | 거부 | retry, logging sink, credential/config, client lifecycle은 caller-owned로 남긴다. |

## Public API

```go
package dynamodbleader

type Client interface {
    PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
    UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
    DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
    GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

type Option func(*config) error

func WithAttributeNames(key, owner, lease, ttl string) Option
func WithClock(clock func() time.Time) Option
func WithRetryDelay(delay time.Duration) Option
func WithLogger(logger *slog.Logger) Option

func New(client Client, tableName string, options leader.Options, optionFns ...Option) (*Elector, error)

type Elector struct { /* constructor-only, caller-owned client */ }

func (e *Elector) Campaign(context.Context) error
func (e *Elector) Resign(context.Context) error
func (e *Elector) IsLeader() bool
func (e *Elector) Leader(context.Context) (string, error)
```

기본 attribute 이름은 `group`, `owner_token`, `lease_until_ms`, `expires_at`다.
`WithAttributeNames`는 네 이름을 모두 non-blank·서로 다르게 검증하고,
expression alias를 사용하므로 DynamoDB reserved word도 안전하게 다룬다.
`tableName`과 `leader.Options`의 문자열은 blank만 거부하며 caller가 준
non-blank byte를 trim하거나 정규화하지 않는다. `Lease`와 `RenewInterval`은
epoch milliseconds로 표현할 수 있도록 각각 최소 1ms이고
`RenewInterval < Lease`여야 한다. clock 기본값은 `time.Now`, retry delay
기본값은 25ms이며 option으로 양수만 허용한다.

`Client`는 필요한 네 SDK method만 포함한다. `*dynamodb.Client`와 mutex-safe
fake가 이를 구현하며, provider는 client를 닫거나 AWS config/credentials를
생성하지 않는다. `WithLogger`를 생략하면 caller의 `slog.Default()`를
사용하고 global logger 설정을 변경하지 않는다. 로그에는 operation과
저카디널리티 상태만 남기며 table/group/token/provider message는 남기지 않는다.

nil interface와 `reflect.IsNil` 가능한 typed-nil client는 생성 시 거부한다.
zero-value `Elector`는 constructor-only이며 method가 panic하지 않는다는
계약을 두지 않는다.

## Item schema와 시간 의미

한 group은 단일 partition-key item으로 저장한다.

| attribute | DynamoDB type | 의미 |
|---|---|---|
| `group` (또는 custom key) | `S` | `leader.Options.Group`, primary key |
| `owner_token` | `S` | `MemberID`와 random 128-bit hex를 조합한 opaque token |
| `lease_until_ms` | `N` | injected clock 기준 absolute epoch milliseconds |
| `expires_at` | `N` | 같은 deadline을 올림한 epoch seconds. TTL cleanup hint일 뿐 |

owner token은 각 `Elector` 생성마다 새로 만들고 error/log/response에 raw token을
출력하지 않는다. deadline은 `now.Add(Lease)`로 계산한다. TTL seconds는
deadline이 초 경계에 걸리지 않으면 다음 정수 초로 올림해 조기 삭제를 피한다.
clock skew는 caller/operator가 관리하며 provider가 server time을 가장하지
않는다.

## Lifecycle과 conditional write

### Campaign

1. nil/canceled context를 확인하고 local `cleanup`, `owned`, `campaigning` 상태를 검사한다.
2. `PutItem`에 전체 item과 `attribute_not_exists(#key)`를 전달한다. 성공하면 ownership을 publish하고 renewal goroutine을 시작한다.
3. `ConditionalCheckFailedException`이면 `UpdateItem`을
   `SET #owner=:owner, #lease=:lease, #ttl=:ttl`로 호출하고 조건
   `attribute_not_exists(#lease) OR #lease <= :now`를 사용한다. update
   성공만 takeover이며, 조건 실패는 bounded retry 대상이다.
4. conditional 이외의 transport/provider error는 동일 elector token을
   strongly consistent `GetItem`으로 reconcile한다. own token이 살아 있으면
   성공으로 승격하고, item이 없거나 다른 owner면 원래 typed operation error를
   반환한다. probe도 실패하면 `leader.ErrCommitUnknown`과 cleanup pending을
   함께 반환한다.
5. provider 응답 직후 context가 취소되면 caller cancellation이 우선한다.
   dispatch가 이미 일어났을 수 있으므로 local cleanup을 pending으로 두고
   `context`와 `leader.ErrCommitUnknown`을 보존한다. 호출자는 fresh cleanup
   context로 같은 elector의 `Resign`을 재시도한다.
6. busy contention만 retry delay 후 반복한다. package는 retry 횟수나 전체
   deadline을 소유하지 않으며 caller context가 상한이다.

### Renewal

renewal goroutine은 `RenewInterval` ticker 하나만 소유한다. 각 update는
owner token 일치와 `lease_until_ms > now` 조건을 함께 검사하고 deadline/TTL을
갱신한다. conditional failure은 ownership loss로 간주해 `IsLeader=false`로
전환하고 다른 owner를 삭제하지 않는다. transport 오류는 bounded strongly
consistent probe를 한 번 시도한다. own active item이면 late response를
복구해 renewal을 계속하고, probe 실패이면 cleanup pending으로 멈춘다.
caller가 `Resign`하면 cancel → renewal done join → 조건부 Delete 순서로
정리한다. ticker, goroutine, context는 모든 경로에서 닫힌다.

### Resign

`DeleteItem`은 `#owner = :owner` 조건을 사용한다. item이 없어졌거나 다른
owner로 교체되어 conditional failure가 반환되면 이미 해제된 것으로 보고
성공한다. transport 오류는 strongly consistent probe로 판단한다. own item이
남아 있거나 probe가 실패하면 `leader.OperationError`와
`leader.ErrCommitUnknown`을 반환하고 cleanup pending을 유지한다. item이
없거나 다른 owner이면 cleanup을 resolved로 기록하고 성공한다. delete 응답
뒤 cancellation은 context error를 우선하되, dispatch 결과가 불명확한 경우
`ErrCommitUnknown`을 함께 보존한다.

동일 elector의 retry는 같은 opaque token을 사용한다. `Resign`은 idempotent하며
동시에 호출되어도 generation/resigning counter가 renewal과 cleanup 상태를
잃지 않도록 mutex로 보호한다. 실패 뒤 `Campaign`은
`leader.ErrCleanupPending`을 반환한다.

### Leader와 IsLeader

`Leader`는 `GetItem(ConsistentRead=true)`만 사용한다. item이 없거나
`lease_until_ms <= now`이면 `"", nil`이다. active item의 owner가 비어 있거나
lease가 number가 아니거나 필수 attribute가 빠졌으면
`ErrMalformedItem`을 반환하며 raw item/value를 error에 넣지 않는다. 호출자
context가 response 뒤 취소되면 decode/발행 없이 context error를 반환한다.
`IsLeader`는 local ownership 상태만 반환하며 backend read를 수행하지 않는다.

## 오류와 redaction

```go
var (
    ErrInvalidClient  = errors.New("dynamodb leader: invalid client")
    ErrInvalidOptions = errors.New("dynamodb leader: invalid options")
    ErrMalformedItem  = errors.New("dynamodb leader: malformed item")
)
```

provider 오류는 `leader.NewOperationError("dynamodb", operation, cause)`로
감싼다. `Error()`와 `%+v`에는 AWS message, table ARN, group, owner token,
expression value를 포함하지 않는다. `errors.Is`/`errors.As`로 원인과
`leader.ErrCommitUnknown`, `types.ConditionalCheckFailedException`을 확인할 수
있어야 한다. conditional contention 자체는 error가 아니며 bounded retry를
통해 다음 시도로 넘긴다. malformed item은 고정 sentinel만 노출한다.

## Failure matrix

| 경계 | 반환/상태 | 의미 |
|---|---|---|
| dispatch 전 nil/canceled context | `leader.ErrInvalidContext` 또는 context error, 호출 0회 | caller가 IO를 허용하지 않음 |
| Put/Update conditional failure | busy 또는 expiry 조건 재평가 | 정상 경합, 다른 owner 보존 |
| transport error + own token probe | success 및 renewal 시작 | 응답만 잃은 commit 복구 |
| transport error + empty/other owner probe | typed `OperationError` | 변경 없음이 확인됨 |
| transport/probe error | `OperationError` + `ErrCommitUnknown`, cleanup pending | 결과 불명확, caller cleanup 필요 |
| renewal conditional failure | `IsLeader=false`, cleanup 없음 | lease가 만료/교체됨 |
| resign conditional failure | nil | 이미 gone/replaced, stale owner 삭제 없음 |
| resign transport/probe 불명확 | `OperationError` + `ErrCommitUnknown` | 같은 elector로 bounded retry |
| malformed active item | `ErrMalformedItem` | schema/producer 계약 위반 |

## 테스트와 수용 기준

mutex-safe fake는 모든 request map과 `AttributeValue`를 deep-copy하고 호출
순서/횟수/context를 기록한다. 다음 test set을 fake-first로 작성한다.

- constructor의 nil/typed-nil client, blank table, duplicate/blank attribute, clock/retry/lease 검증
- 최초 acquire, duplicate/in-progress, exact contention에서 winner 1개, 조건부 takeover와 expiry
- renewal 성공, conditional loss, transport error 후 own probe recovery, probe failure cleanup
- resign idempotent, stale resign, output-plus-error, conditional failure, retry 후 resolved
- `Leader`의 `ConsistentRead=true`, missing/expired/malformed item, injected clock
- dispatch 전/후 cancellation, no late decode, `errors.Is`/`errors.As`, raw marker redaction
- `leader/leadertest.Harness`와 의미가 맞는 15개 시나리오를 adapter control로 실행하거나, 불일치한 backend clock 항목을 명시적으로 제외
- concurrent campaign/renew/resign의 bounded stress와 `go test -race`

일반 CI는 fake/unit, examples, vet/lint만 실행한다. Floci/DynamoDB 또는 live
AWS 테스트는 explicit environment/build tag로만 opt-in하고 connection
readiness와 cleanup을 별도 기록한다. #560 benchmark follow-up은 이 provider
구현에서 새 benchmark를 추가하는 범위가 아니며, deterministic local evidence가
생기면 후속 기록으로 연결한다.

SPW-01 요구사항은 live #559 body와 parent/related metadata로 확인했다.
SPW-02 이 설계, SPW-03 실행 plan, SPW-04 RED→GREEN 구현, SPW-05 fresh
verification evidence는 각각 이 문서의 해당 섹션과 PR evidence에 연결한다.

## 문서와 rollback

`leader/README.md`와 `README.ko.md`에는 DynamoDB package 링크, schema/IAM
action(`dynamodb:GetItem`, `PutItem`, `UpdateItem`, `DeleteItem`), strongly
consistent read, TTL cleanup, provisioned/on-demand capacity, retry/error 및
caller-owned credentials/client lifecycle을 추가한다. 새 package README와
compile-checked example은 같은 정보를 locale pair로 제공한다.

rollback은 consumer가 새 package를 사용하지 않도록 PR commit을 revert하는
것으로 한정한다. 이미 기록된 item을 provider가 임의로 삭제하거나 migration하지
않는다.
