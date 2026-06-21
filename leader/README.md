# leader

[English](README.md) | [한국어](README.ko.md)

`leader` defines the leader-election contracts used by bluetape-go backends.
The package owns option validation, sentinel errors, and the shared API shape;
backend-specific Redis behavior lives in `leader/redis`.

## Diagram

![leader contract overview](../docs/images/readme-diagrams/leader-contract-overview.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/leader"
```

## Usage

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

## Behavior

- `Elector` represents one member inside one leader group.
- `Campaign` attempts leadership acquisition; `Resign` releases only the
  caller's current ownership.
- `GroupElector` allows up to `MaxLeaders` live members in one group.
- `StrategicElector` uses a candidate registry plus a deterministic
  `ElectionStrategy` so every node can compute the same winner from the same
  live candidate list.
- Built-in strategies include FIFO, seed-stable random, and scored election.
- Built-in scorers include idle time, success rate, candidate weight, and
  weighted composition.
- Public errors are sentinel errors and should be compared with `errors.Is`.
- Backend renewal failures should cause `IsLeader` to return false.

## Test

```bash
go test -count=1 ./leader
```
