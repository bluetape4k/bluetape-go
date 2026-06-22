# core

[English](README.md) | [한국어](README.ko.md)

`core` contains narrow shared helpers used by bluetape-go packages. Prefer the
Go standard library when it already expresses the operation clearly; this
package is for repeated validation, pointer, zero/default, string, and small
numeric checks. It also contains small ordered range values when callers need
explicit open/closed boundary semantics, plus wildcard and XXH64 helpers for
repeated filter/key use cases, and small calendar helpers for quarter and
date-iteration workflows.

![core helper boundary map](../docs/images/readme-diagrams/core-helper-boundary-map.png)

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

window, err := core.ClosedOpenRange(10, 20)
if err != nil {
    return err
}
_ = window.Contains(10) // true

matched, err := core.MatchWildcard("order-*.json", "order-2026.json")
if err != nil {
    return err
}
_ = matched // true

pathMatched, err := core.MatchWildcardPath("configs/**/*.yaml", `configs\prod\app.yaml`)
if err != nil {
    return err
}
_ = pathMatched // true

cacheKeyHash := core.XXH64String("customer:42")
_ = cacheKeyHash

reportingQuarter, err := core.ParseYearQuarter("2026-Q3")
if err != nil {
    return err
}
periodStart, err := reportingQuarter.Start(time.UTC)
if err != nil {
    return err
}
periodEnd, err := reportingQuarter.End(time.UTC)
if err != nil {
    return err
}

for reportDate := range core.DatesUntil(periodStart, periodStart.AddDate(0, 0, 3)) {
    _ = reportDate.Format(time.DateOnly)
}
_ = periodEnd
```

## Behavior

- Validation helpers return errors instead of panicking.
- `Range` constructors support `[lower, upper]`, `[lower, upper)`,
  `(lower, upper]`, and `(lower, upper)` notation through `ClosedRange`,
  `ClosedOpenRange`, `OpenClosedRange`, and `OpenOpenRange`.
- Invalid ranges and NaN float endpoints are rejected. The zero-value `Range`
  is safe and empty; use constructors for non-empty ranges.
- `Zero`, `IsZero`, `DefaultIfZero`, and `FirstNonZero` keep generic fallback
  behavior explicit.
- `TruncateUTF8Bytes` truncates at rune boundaries and rejects negative limits
  or invalid UTF-8 input.
- Hex helpers validate prefixed `0x` / `0X` strings without decoding them.
- `MatchWildcard` is case-sensitive and supports `*`, `?`, consecutive stars,
  and escaped literals such as `\*`, `\?`, and `\\`. A trailing escape returns
  `ErrMalformedWildcardPattern`.
- `MatchWildcardPath` is lexical and portable: it accepts `/` and `\` as input
  separators, treats a full `**` segment as zero or more path segments, supports
  escaped `*`, `?`, and `\` inside slash-separated pattern segments, stays
  case-sensitive on every OS, and never reads or cleans the filesystem.
- `XXH64Bytes` and `XXH64String` return deterministic XXH64 values with seed 0.
  XXH64 is non-cryptographic; use `crypto/*` or a keyed MAC for signatures,
  passwords, tokens, or attacker-resistant integrity checks.
- `Quarter` and `YearQuarter` helpers validate quarter values, reject year `0`,
  and use exclusive quarter ends so reporting windows can be expressed as
  `[start, end)`.
- `DatesUntil` and `DatesInclusive` iterate calendar dates in the start
  location and yield midnight values. End boundaries are converted into the
  start location before date comparison.
- Date iteration advances with `time.Time.AddDate`, so daylight-saving
  transitions preserve calendar dates instead of assuming every day is exactly
  24 hours.
- Kotlin operator overloads and DSL-style range constructors are intentionally
  not part of this Go API.
- Kotlin numeric duration DSLs, broad duration parser/formatter wrappers,
  Java-time temporal type mirrors, period/calendar frameworks, JVM classpath
  resource loading, system-property wrappers, shutdown hooks, generic object
  hashing, temp/output/env helpers, and broad string/byte aliases are
  intentionally excluded. Go callers should use `time`, `os`, `io/fs`,
  `runtime`, `context`, and explicit encodings directly.

## Test

```bash
go test -count=1 ./core
```
