# EventBridge 감사 publisher adapter 설계

## 상태와 범위

- 상태: 승인된 설계
- 대상: `bluetape-go` 유지보수자와 `audit/sqloutbox` 호출자
- 언어: Korean technical documentation. API 이름, 식별자, 명령, URL, AWS 공식 용어는 원문을 유지한다.
- 부모 이슈: [#517](https://github.com/bluetape4k/bluetape-go/issues/517)
- 작업 이슈: [#520](https://github.com/bluetape4k/bluetape-go/issues/520)
- 기준 head: `2d130ed78a258751d5baf394f3a82cb0a7b31159`
- 실행 경계: `feat/issue-520-eventbridge-audit` worktree에서 구현·검증한다. EventBridge provisioning, live AWS 호출, tag/release는 범위가 아니다.

이 문서는 사용자가 승인한 #517 실행 순서의 첫 slice인 #520을 독립 PR로
완료하기 위한 기준이다. #521 Kinesis, #522 이후 transport, retry 정책 변경은
이 설계의 승인 범위를 확장하지 않는다.

## 근거와 결정

| 근거 | 확인 내용 |
|---|---|
| `audit/sqloutbox/relay.go`, `store.go` | `Publisher.Publish(context.Context, Record) error`, at-least-once, stable `EventID`/`IdempotencyKey`, cancellation과 bounded failure text 계약 |
| `audit/sqloutbox/redisstreams/publisher.go` | caller-owned narrow AWS/Redis client, typed-nil 검사, transport envelope와 accessor 패턴 |
| `audit/sqloutbox/README.md`, `.ko.md` | topology와 downstream idempotency는 caller/operator 책임이며 broker adapter는 별도 package라는 문서 경계 |
| [AWS SDK for Go v2 EventBridge package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/eventbridge) | `PutEvents` context 시그니처, request entry와 `FailedEntryCount`/per-entry result shape |
| [AWS EventBridge `PutEvents` API](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_PutEvents.html) | HTTP 성공이어도 entry별 오류가 가능하고 응답 순서는 요청 순서를 보존한다. 한 호출은 최대 10 entries, entry 합계는 256 KB 미만이다. |
| [AWS 연구 게이트](https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-aws-research-gate.md) | `audit/sqloutbox.Publisher`와 EventBridge의 적합성, fake-first CI, caller-owned topology/downstream idempotency 결정 |

AWS service module은 현재 root SDK `v1.42.1`과 호환되는
`github.com/aws/aws-sdk-go-v2/service/eventbridge v1.47.0`을 직접 요구한다.
더 최신 module로 root SDK를 함께 올리는 것은 별도 dependency review로 남긴다.

## 문제와 목표

현재 outbox relay는 durable `Record`를 publish하는 경계를 제공하지만 AWS
EventBridge로 전송하는 재사용 가능한 adapter가 없다. 목표는 다음과 같다.

1. `audit/sqloutbox.Publisher` 의미를 유지하는 `audit/sqloutbox/eventbridge` package를 추가한다.
2. caller-owned EventBridge client로 `PutEvents`를 호출하고, stable event ID와 idempotency key를 EventBridge detail에 보존한다.
3. HTTP-level success와 entry-level partial/full failure를 결정론적으로 오류로 매핑한다.
4. cancellation, invalid/oversized entry, nil/typed-nil client와 redacted error를 fake client로 검증한다.
5. EventBridge bus/rule/target provisioning과 downstream idempotency의 소유권을 caller/operator에게 명확히 남긴다.

## 비목표와 안전 경계

- AWS config/credential/region/client lifecycle, retry/backoff, batching, buffering, metrics/logger 설치
- event bus/rule/target 또는 IAM/key policy provisioning
- Kinesis adapter, cross-language wire compatibility, live AWS integration test
- EventBridge `PutEvents`의 최대 10-entry batching API 또는 새로운 broker abstraction
- EventBridge response의 `EventId`를 outbox `EventID` 또는 idempotency key로 대체하는 동작

adapter는 한 `Publish` 호출에서 정확히 하나의 `PutEventsRequestEntry`만 보낸다.
따라서 SDK의 10-entry 상한을 우회하지 않고, relay의 retry/dead-letter 정책은
기존 `sqloutbox.Relay`가 계속 소유한다. EventBridge가 발급한 `EventId`는
호출자에게 별도 성공 결과로 노출하지 않으며 detail의 stable identity가
downstream deduplication 기준이다.

## Public API

```go
package eventbridge

type Client interface {
	PutEvents(context.Context, *awseventbridge.PutEventsInput, ...func(*awseventbridge.Options)) (*awseventbridge.PutEventsOutput, error)
}

var _ Client = (*awseventbridge.Client)(nil)

type Options struct {
	Client        Client
	EventBusName  string // empty means AWS default event bus
	Source        string // required, exact value is preserved
	DetailType    string // required, exact value is preserved
	MaxDetailSize int    // zero uses 256 KiB default
}

func New(options Options) (*Publisher, error)

type Publisher struct { /* immutable caller-owned client and copied options */ }

func (p *Publisher) Publish(context.Context, sqloutbox.Record) error
func (p *Publisher) EventBusName() string
func (p *Publisher) Source() string
func (p *Publisher) DetailType() string
```

`Client`는 SDK method subset 그대로이므로 `*eventbridge.Client`와 fake가
같은 계약을 만족한다. `New`는 nil과 reflection으로 확인한 typed-nil client,
blank `Source`/`DetailType`, invalid UTF-8, AWS 제약을 넘는 길이, 음수
`MaxDetailSize`를 거부한다. `EventBusName`은 빈 값이면 request field를
생략하여 AWS default bus를 사용하고, non-blank 값은 trim하거나 정규화하지
않고 그대로 전달한다. `Source`와 `DetailType`도 exact value를 보존한다.

문자열 길이는 UTF-8 rune 수가 아니라 AWS request의 UTF-8 byte 길이로
검사한다. `Source`는 256 bytes, `DetailType`은 128 bytes,
`EventBusName`은 256 bytes 이하만 허용한다. `MaxDetailSize` 기본값은
`256 << 10`이고 256 KiB를 초과하도록 설정할 수 없다. 실제 preflight는
detail bytes뿐 아니라 EventBridge entry size에 포함되는 `Source`,
`DetailType`, `EventBusName`의 UTF-8 bytes도 더해 256 KiB 미만인지 확인한다.
따라서 metadata overhead가 있는 경우 effective detail limit은 자동으로 더
작아진다.

`Publisher`는 생성 후 immutable이며 client를 닫거나 재구성하지 않는다. 주입된
client가 method 동시 호출에 안전하고 request input을 반환 뒤 보관/변경하지
않는다는 전제에서 concurrent `Publish` 호출에 안전하다. provider가 retry,
goroutine cancellation, global logger를 추가하지 않는다. nil context는
repository convention에 따라 `context.Background()`로 정규화한다.

## Detail envelope

detail은 표준 JSON object이며 `audit/sqloutbox/redisstreams`와 같은 stable
record metadata를 사용한다. `entry_json`은 검증된 `audit.Entry`의 JSON
object를 `json.RawMessage`로 삽입한다.

```json
{
  "record_id": 42,
  "status": "claimed",
  "aggregate_type": "invoice",
  "aggregate_id": "inv-7",
  "revision": 3,
  "event_id": "evt-7",
  "idempotency_key": "invoice:inv-7:3",
  "event_type": "invoice.paid",
  "occurred_at": "2026-09-03T02:00:00Z",
  "recorded_at": "2026-09-03T02:00:01Z",
  "schema_version": 1,
  "attempts": 1,
  "entry_json": { "schema_version": 1, "aggregate": {}, "event": {} }
}
```

모든 record identity와 `Entry.Validate()` 결과는 publish 전에 고정한다.
`Record.ID`와 `Attempts`는 양수여야 하며, `Record.Entry`가 유효하고
record의 aggregate/revision/event ID/idempotency/event type/schema 및
timestamps와 exact match해야 한다. 불일치나 malformed entry는 SDK 호출
없이 `ErrInvalidRecord`로 반환한다. `status`는 진단용으로 보존하되 relay가
전달한 값을 임의로 변경하지 않는다.

## Publish data flow

1. nil context를 정규화하고 `ctx.Err()`를 확인한다.
2. record identity, entry validation, detail JSON 생성 및 bounded size를 SDK 호출 전에 수행한다.
3. `PutEventsInput`에 `Entries` 한 개를 구성한다. `EventBusName`이 빈 값이면 해당 pointer를 생략하고, `Source`, `DetailType`, `Time=Record.OccurredAt`를 전달한다.
4. dispatch 직전에 context를 다시 확인하고 `PutEvents(ctx, input)`을 한 번 호출한다. adapter는 SDK retry를 추가하지 않는다.
5. transport error는 원인에 대해 `errors.Is`를 보존할 수 있지만 `Error()`/`%+v`에는 operation과 안전한 sentinel만 노출한다.
6. 반환 직후 context가 취소되었으면 성공 response라도 cancellation을 반환한다. 취소는 relay가 retry/dead-letter하지 않는 기존 의미를 보존한다.
7. output은 non-nil이고 `len(Entries)==1`이어야 한다. `FailedEntryCount==0`이고 entry error code가 비어 있을 때만 성공이다.

detail 또는 EventBridge entry size가 한도를 넘으면 `ErrDetailTooLarge`로
반환하고 SDK 호출은 0회다. JSON에 raw payload, credentials, client error text를 별도 로그로
기록하지 않는다.

## Failure mapping

```go
var (
	ErrNilClient       = errors.New("eventbridge: client must not be nil")
	ErrInvalidOptions  = errors.New("eventbridge: invalid options")
	ErrInvalidRecord   = errors.New("eventbridge: invalid outbox record")
	ErrDetailTooLarge  = errors.New("eventbridge: detail exceeds limit")
	ErrPublishFailed   = errors.New("eventbridge: publish failed")
	ErrPartialFailure  = errors.New("eventbridge: entry failure")
	ErrMalformedOutput = errors.New("eventbridge: malformed response")
)
```

`*Error`는 고정된 kind/operation, optional safe AWS `ErrorCode`, failure
count와 sanitized cause만 보관한다. `Error()`와 `%+v`는 detail, event ID,
idempotency key, bus/source 값, AWS `ErrorMessage`를 출력하지 않는다.
`Unwrap()`은 transport error 또는 package sentinel만 반환하며 response의
문자열 message를 error chain에 넣지 않는다. 접근 가능한 `FailureCount()`와
`ErrorCode()`는 저카디널리티 운영 계측에 사용할 수 있다.

| 상황 | 오류 | 호출자 의미 |
|---|---|---|
| SDK 호출 전 context 취소 | `context.Canceled`/`context.DeadlineExceeded` | relay가 retry/dead-letter하지 않음 |
| invalid/oversized record/detail | `ErrInvalidRecord`/`ErrDetailTooLarge` | 영구 입력 오류, SDK 호출 0회 |
| network/SDK error | `ErrPublishFailed`를 감싼 `*Error` | relay가 기존 retry/dead-letter 경로를 적용 |
| nil/malformed output | `ErrMalformedOutput`를 감싼 `*Error` | provider contract 위반 또는 SDK anomaly |
| `FailedEntryCount>0` 또는 entry error code | `ErrPartialFailure`를 감싼 `*Error` | 단일 entry이므로 이번 publish가 실패; bounded retry 가능 |

단일 entry에서 `FailedEntryCount==1`은 논리적으로 full failure이지만
EventBridge의 entry-level failure contract를 보존하기 위해 `FailureCount`와
entry index를 함께 노출한다. `FailedEntryCount`와 entry 결과가 모순되거나
entry 수가 1이 아니면 `ErrMalformedOutput`이다. AWS response의
`ErrorMessage`는 보존·포맷·로그하지 않는다.

## Test contract

live AWS 대신 mutex-safe fake client가 request deep copy, 호출 수, context,
configured output/error를 기록한다. 다음을 table-driven 및 race test로
증명한다.

- constructor nil/typed-nil client, required field, UTF-8/length, default bus와 custom bus
- success request mapping, single entry, `entry_json`, stable `event_id`/`idempotency_key`, occurred time
- invalid record, identity mismatch, malformed entry, oversized detail에서 SDK 호출 0회
- pre-dispatch cancellation과 response 후 cancellation
- transport error, nil output, wrong entry count, `FailedEntryCount`, per-entry error code/message redaction
- `errors.Is`, bounded `Error()`/`fmt.Sprintf("%+v")`, `FailureCount`/`ErrorCode` 관찰성
- concurrent publish와 fake request isolation under `go test -race`
- `sqloutbox.Relay`가 transport/entry failure는 mark-failed하고 cancellation은 retry/dead-letter하지 않는 통합 contract

fake에는 credentials, live endpoint, real AWS retry를 넣지 않는다. `go test
./...`, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`,
`make race`, `make ci`와 package targeted normal/race test를 실행하고,
Testcontainers가 필요한 기존 package 실패는 변경 package 결과와 분리해
정확히 보고한다.

## 문서와 운영 경계

`audit/sqloutbox/README.md`와 `README.ko.md`에 EventBridge package 링크,
single-entry/partial-failure/cancellation 의미를 추가하고 locale pair를
동일한 정보로 유지한다. 새 package의 English/Korean README는 다음을
명시한다.

- `EventBusName` 생략은 AWS default bus이며 bus/rule/target provisioning은 caller/operator 책임
- downstream consumer의 idempotency는 detail의 `event_id`/`idempotency_key`를 이용해 caller가 구현
- SDK credential/config/retry/timeout/client lifecycle은 caller 소유
- EventBridge response `EventId`는 stable outbox identity가 아님
- 최소 IAM과 live smoke/production rollout은 이 package CI/DoD에 포함되지 않음

기존 outbox diagram이 relay→publisher 경계를 충분히 설명하므로 새 diagram은
추가하지 않는다. 설계/계획/검토/lesson은 `docs/superpowers`와 `docs/review`,
`docs/lessons`에 한국어로 남긴다.

## 수용 기준과 SPW gate

| 기준 | 판정 방법 |
|---|---|
| `sqloutbox.Publisher` 의미 보존 | compile assertion, relay contract test, cancellation/retry test |
| stable identity 보존 | captured detail JSON assertions |
| deterministic failure mapping | output matrix와 redaction assertions |
| caller-owned topology/client | API와 locale README 검토 |
| fake-first/no live AWS | fake package tests와 CI command evidence |
| bounded, idiomatic Go API | constructor/size/typed-nil/race tests |

SPW-01 requirements, SPW-02 design, SPW-03 plan, SPW-04 implementation,
SPW-05 verification은 각각 issue body/live metadata, 이 설계와 source ledger,
실행 plan, RED→GREEN diff, fresh test/CI evidence로 추적한다. 현재 문서는
SPW-01/02 PASS이며, implementation 이후 나머지를 갱신한다.

## 롤백과 후속 범위

배포 전 rollback은 이 PR의 commit revert로 한정한다. 이미 publish된
EventBridge event의 삭제/재처리는 adapter가 시도하지 않는다. #521 Kinesis와
#522 Step Functions는 이 package의 API를 재사용한다는 가정을 하지 않고,
각각 독립 설계·검토·PR gate를 거친다.
