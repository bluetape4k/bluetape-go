# DynamoDB leader

[English](README.md)

`github.com/bluetape4k/bluetape-go/leader/dynamodb`는 caller-owned DynamoDB의
단일 item lease로 leader를 선출합니다. 조건부 `PutItem`/`UpdateItem`/
`DeleteItem`과 strongly consistent `GetItem`을 사용하며 `leader.Elector`를
구현합니다.

## 사용

```go
elector, err := dynamodbleader.New(
    client, // caller-owned *dynamodb.Client 또는 narrow Client interface
    "worker-leases",
    leader.Options{
        Group: "billing-workers", MemberID: "worker-1",
        Lease: 30 * time.Second, RenewInterval: 10 * time.Second,
    },
)
if err != nil { return err }
if err := elector.Campaign(ctx); err != nil { return err }
defer elector.Resign(cleanupCtx)
```

`Client`는 `PutItem`, `UpdateItem`, `DeleteItem`, `GetItem` 네 method만
포함합니다. AWS client 생성·종료, credential/region, retry와 timeout policy,
table/IAM provisioning은 caller가 소유합니다. 이 package는 client를 만들거나
migration을 실행하지 않습니다.

공통 `leader.Options.KeyPrefix`는 provider 호환성을 위해 검증하지만 DynamoDB
item key에는 인코딩하지 않습니다. 이 provider에서 table 자체가 namespace
경계이므로 서로 다른 key-prefix 정책을 하나의 table에서 공유하려면 `Group`이
전역적으로 충돌하지 않도록 caller가 보장해야 합니다. namespace별 table 분리나
group 이름에 prefix를 넣는 정책은 operator/caller의 책임입니다.

## Item schema

기본 attribute는 다음과 같습니다.

| Attribute | Type | 의미 |
|---|---|---|
| `group` | `S` | partition key이자 `leader.Options.Group` |
| `owner_token` | `S` | 이 elector instance의 opaque member token |
| `lease_until_ms` | `N` | correctness에 사용하는 absolute epoch milliseconds |
| `expires_at` | `N` | DynamoDB TTL cleanup 전용 epoch seconds |

다른 schema는 `WithAttributeNames`로 지정하세요. Expression에는 alias를
사용하며 이름을 trim하거나 조용히 변경하지 않습니다. Lease deadline은
`WithClock`(기본 `time.Now`) 기준이고 `RenewInterval < Lease`, 두 값 모두
최소 1ms여야 합니다. TTL은 다음 초로 올림해 asynchronous cleanup이 active
lease를 일찍 지우지 않게 하지만, correctness 판단에는 사용하지 않습니다.

## Lifecycle과 consistency

Campaign은 먼저 `attribute_not_exists(#key)`로 최초 획득을 시도합니다.
Conditional failure 뒤에는 `attribute_not_exists(#lease) OR #lease <= :now`
조건으로 expired owner만 교체합니다. Renewal은 owner token과
`lease_until_ms > :now`를 함께 검사하며 conditional failure이면
`IsLeader=false`가 되고 다른 owner를 삭제하지 않습니다.

`Leader`는 항상 `ConsistentRead=true`로 읽고 injected clock과 deadline을
비교합니다. Item이 없거나 만료되면 `"", nil`을 반환하고 active item이
malformed이면 `ErrMalformedItem`을 반환합니다. DynamoDB TTL 삭제 시점은
비동기이므로 correctness signal이 아닙니다. Provisioned/on-demand capacity,
hot key와 throttling은 caller/operator가 관리합니다.

## Error와 cleanup

Provider failure는 안전한 operation label을 가진 `leader.OperationError`로
반환합니다. AWS message, table name, group, owner token은 `Error()`나 `%+v`에
포함되지 않습니다. Cause와 `leader.ErrCommitUnknown`은
`errors.Is`/`errors.As`로 확인하세요.

Write response가 유실되거나 bounded attempt context 뒤에 늦은 response가
도착해도 response만 믿지 않고 bounded strongly consistent probe를 한 번
수행합니다. Own active token이면 commit을 복구하고, empty/다른 owner이면
새 campaign attempt를 허용합니다. Probe도 실패하면
`leader.ErrCommitUnknown`과 cleanup pending을 반환하므로 fresh cleanup
context로 같은 elector의 `Resign`을 재시도하세요. Takeover deadline은
conditional update 직전에 다시 계산하며 Resign probe도 같은 attempt budget으로
제한합니다. Conditional delete failure는 item이 이미 사라졌거나 교체된
상태이므로 idempotent success입니다.

Lifecycle/failure log는 caller가 선택한 `log/slog` logger(`WithLogger`)로만
기록합니다. Global logger 설정을 변경하지 않고 raw provider text도 기록하지
않습니다.

## IAM과 live test

Table role에는 caller가 선택한 table에 대해 `dynamodb:GetItem`,
`dynamodb:PutItem`, `dynamodb:UpdateItem`, `dynamodb:DeleteItem` action이
필요합니다. Least-privilege resource policy, encryption, network, credential
rotation은 package 범위 밖입니다. 일반 CI는 deep-copy fake만 사용하므로 AWS
credential이 필요 없습니다. Floci/live AWS test는 명시적 opt-in일 때만
실행합니다.

Compile-checked fake 구성은 `ExampleNew`를 참고하세요.
