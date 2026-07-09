# leader

[English](README.md) | [한국어](README.ko.md)

`leader`는 bluetape-go backend에서 사용하는 leader-election contract를 정의합니다.
이 패키지는 option validation, sentinel error, shared API shape를 제공하고,
Redis에 특화된 동작은 `leader/redis`가 담당합니다.

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

## 테스트

```bash
go test -count=1 ./leader
go test -count=1 ./leader/mongo
```
