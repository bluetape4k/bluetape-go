# core

[English](README.md) | [한국어](README.ko.md)

`core` contains narrow shared helpers used by bluetape-go packages. Prefer the
Go standard library when it already expresses the operation clearly; this
package is for repeated validation, pointer, zero/default, string, and small
numeric checks.

## Import

```go
import "github.com/bluetape4k/bluetape-go/core"
```

## Usage

```go
name := core.BlankToDefault(input.Name, "anonymous")
limit, err := core.Clamp(input.Limit, 1, 100)
if err != nil {
    return err
}

owner := core.Ptr("worker-1")
_ = core.ValueOr(owner, "fallback")
```

## Behavior

- Validation helpers return errors instead of panicking.
- `Zero`, `IsZero`, `DefaultIfZero`, and `FirstNonZero` keep generic fallback
  behavior explicit.
- `TruncateUTF8Bytes` truncates at rune boundaries and rejects negative limits.
- Hex helpers validate prefixed `0x` / `0X` strings without decoding them.

## Test

```bash
go test -count=1 ./core
```
