# leader

`leader` defines the leader-election contracts used by bluetape-go backends.
The package owns option validation, sentinel errors, and the shared API shape;
backend-specific Redis behavior lives in `leader/redis`.

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
- Public errors are sentinel errors and should be compared with `errors.Is`.
- Backend renewal failures should cause `IsLeader` to return false.

## Test

```bash
go test -count=1 ./leader
```
