# leader/mongo

[English](README.md) | [한국어](README.ko.md)

`leader/mongo`는 단일 `leader.Elector` contract를 구현하는 MongoDB backend입니다.
하나의 leader key마다 하나의 lease document를 저장하고, acquire, renew,
release, observation 모두 owner-token predicate로 보호합니다.

이 package는 `leader.GroupElector`나 `leader.StrategicElector`를 구현하지 않습니다.

## 가져오기

```go
import mongoleader "github.com/bluetape4k/bluetape-go/leader/mongo"
```

## 사용 예

```go
collection := client.Database("coordination").Collection("leader_leases")
if err := mongoleader.EnsureIndexes(ctx, collection); err != nil {
    return err
}

elector, err := mongoleader.New(collection, leader.Options{
    Group:         "billing-workers",
    MemberID:      "worker-1",
    Lease:         30 * time.Second,
    RenewInterval: 10 * time.Second,
})
if err != nil {
    return err
}
if err := elector.Campaign(ctx); err != nil {
    return err
}
defer elector.Resign(context.Background())
```

## 저장소 계약

각 leader group은 `_id`로 식별되는 하나의 document를 사용합니다.

| Field | 목적 |
|---|---|
| `_id` | 정규화된 leader key, `<keyPrefix>:<group>`. |
| `group` | Leader group name. |
| `member_id` | 현재 owner member ID. |
| `token` | `Leader`가 반환하는 opaque owner token. |
| `lease_until` | 권위 있는 lease expiry. |
| `created_at` / `updated_at` | 진단용 timestamp. |

`EnsureIndexes`는 cleanup용 `lease_until` TTL index를 만듭니다. MongoDB TTL
monitor는 비동기이므로 leadership correctness는 TTL 삭제 timing에 의존하지
않습니다. `Campaign`과 `Leader`는 일반 `lease_until` predicate로 판단합니다.

## 운영 경계

- MongoDB client, database, collection, index, write concern, cleanup은 caller가
  소유합니다.
- 운영 환경에서는 caller-owned client나 collection에 적절한 write concern,
  일반적으로 majority를 설정하세요.
- 첫 구현은 process clock으로 `lease_until`을 계산합니다. contender 사이의 clock을
  동기화하고, 예상 clock skew와 MongoDB operation latency보다 충분히 긴 lease를
  사용하세요.
- `Campaign(ctx)`은 leadership을 얻거나 context가 취소될 때까지 대기합니다.
- Renewal 실패, token 교체, backend renewal error가 발생하면 `IsLeader`는 false가
  됩니다.

## 테스트

```bash
go test -count=1 ./leader ./leader/mongo
go test -race -count=1 ./leader ./leader/mongo
go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb
```

