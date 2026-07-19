# leader

[English](README.md) | [한국어](README.ko.md)

`leader`는 bluetape-go backend에서 사용하는 leader-election contract를 정의합니다.
이 패키지는 option validation, sentinel error, shared API shape를 제공하고,
backend 구현은 `leader/redis`, `leader/mongo`, `leader/sql`, `leader/etcd`가 담당합니다.

## 다이어그램

![leader contract overview](../docs/images/readme-diagrams/leader-contract-overview.png)

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/leader"
```

## 사용 예

```go
opts, err := leader.Options{
    Group:         "billing-workers",
    MemberID:      "worker-1",
    Lease:         30 * time.Second,
    RenewInterval: 10 * time.Second,
}.Normalize()
if err != nil {
    return err
}
```

## 동작

- `Elector`는 하나의 leader group 안에 있는 하나의 member를 나타냅니다.
- `Campaign`은 leadership 획득을 시도하고, `Resign`은 caller의 current ownership만 해제합니다.
- `GroupElector`는 하나의 group 안에서 최대 `MaxLeaders`개의 live member를 허용합니다.
- `StrategicElector`는 candidate registry와 deterministic `ElectionStrategy`를 사용해 모든 node가 같은 live candidate list에서 같은 winner를 계산할 수 있게 합니다.
- Built-in strategy에는 FIFO, seed-stable random, scored election이 포함됩니다.
- Built-in scorer에는 idle time, success rate, candidate weight, weighted composition이 포함됩니다.
- Public error는 sentinel error이며 `errors.Is`로 비교해야 합니다.
- Backend renewal failure가 발생하면 `IsLeader`는 false를 반환해야 합니다.

## Backend 참고

- `leader/redis`는 단일, group, strategic leader election을 지원합니다.
- `leader/mongo`는 단일 `Elector`, bounded-slot `GroupElector`, candidate-registry
  `StrategicElector`를 지원합니다. 단일 elector는 MongoDB group마다 하나의 lease
  document를 저장하고, group elector는 정확한 `MaxLeaders` 보장을 위해 slot마다
  하나의 lease document를 저장하며, strategic elector는 node마다 하나의 candidate
  document를 저장합니다. TTL index는 cleanup 용도로만 취급합니다.
- `leader/sql`은 caller-owned row lease와 caller-owned `*sql.DB`를 사용하는 PostgreSQL
  전용 단일 `Elector`입니다. Group/strategic election은 제공하지 않습니다.
- `leader/etcd`는 caller-owned `*clientv3.Client`, 공식 Session/Election primitive,
  server-granted TTL, bounded Proclaim, exact-key ownership monitoring을 사용하는 단일
  `Elector`입니다. Fencing token, group/strategic election은 제공하지 않습니다.

## 테스트

```bash
go test -count=1 ./leader
go test -count=1 ./leader/mongo
go test -p 1 -count=1 ./leader/sql
go test -p 1 -count=1 ./leader/etcd
```

## Single-Elector Conformance

Single elector는 이제 leadership 획득 또는 caller context 종료까지 기다립니다. Local
duplicate, in-progress, cleanup-pending, nil-context 상태는 서로 다른 sentinel을 사용합니다.
Bare context error보다 typed `OperationError`와 `ErrCommitUnknown`을 먼저 확인하고,
commit-indeterminate이면 bounded `Resign` 후 TTL fallback을 사용합니다.
`leader/leadertest.Run`은 Redis, Mongo, PostgreSQL, etcd에 같은 contract를 적용합니다.
Group과 strategic elector는 이 single-elector suite 범위 밖입니다. Etcd의 불확실한
cleanup은 revoke 성공 또는 linearizable exact-key proof가 필요하며 TTL 경과만으로는
증명되지 않습니다.
