# Issue #180 FastMoney Evaluation

## Decision

Do not add a public long-backed `FastMoney` type in #180.

The measured local benchmark snapshot does not cross the approved threshold for
duplicating the `money` public API. `Money.NewMinor` is about 1.15x slower than
the direct `govalues/money` minor-unit constructor on this machine, both paths
allocate zero objects per operation, and the existing `Money.MinorUnits`,
`Money.Add`, and `Money.Sum` hot paths also allocate zero objects per operation.

The existing public API remains:

- `NewMinor` for integer minor-unit input.
- `MinorUnits` for integer minor-unit extraction.
- `Money` for decimal-backed immutable amount behavior.

## Benchmark Environment

- Commit: `3ec2844`
- Dirty state recorded in raw output: `money/money_benchmark_test.go` and `docs/research/outputs/issue-180/` were uncommitted during capture.
- Go: `go version go1.26.4 darwin/arm64`
- GOOS/GOARCH: `darwin/arm64`
- CPU: `Apple M4 Pro`
- Command: `go test -run '^$' -bench '^BenchmarkMoney' -benchmem ./money`

## Benchmark Chart

![Money FastMoney evaluation benchmark](../images/readme-charts/money-fastmoney-evaluation-benchmark.png)

Chart assets:

- SVG: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg`
- PNG: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`
- Vega-Lite data source: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.vl.json`
- Generator: `docs/images/readme-charts/generate-money-fastmoney-evaluation-benchmark.mjs`

## Raw Benchmark Output

Raw output is stored at:

```text
docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt
```

Treat this as the numeric source of truth. The chart is only a scan-friendly
visualization of the same rows. Lower `ns/op`, lower `B/op`, and lower
`allocs/op` are better.

## Results

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkMoneyNewMinorUSD` | 9.249 | 0 | 0 |
| `BenchmarkMoneyNewMinorJPY` | 9.236 | 0 | 0 |
| `BenchmarkMoneyMinorUnitsUSD` | 5.833 | 0 | 0 |
| `BenchmarkMoneyAddUSD` | 9.027 | 0 | 0 |
| `BenchmarkMoneySumUSD10` | 112.4 | 0 | 0 |
| `BenchmarkMoneyParseUSD` | 59.42 | 32 | 1 |
| `BenchmarkMoneyMarshalJSON` | 271.2 | 168 | 5 |
| `BenchmarkMoneyDirectGovaluesNewAmountFromMinorUnits` | 8.021 | 0 | 0 |

## Interpretation

The approved threshold for a public `FastMoney` follow-up was:

- `NewMinor`, `MinorUnits`, `Add`, or `Sum` at least 3x slower than the direct
  `govalues` reference for the same operation family, or
- the simplest minor-unit path above 5 allocs/op while the reference path stays
  near zero, or
- a documented caller workflow needing long-backed minor-unit storage as a
  public type boundary.

The benchmark does not meet those conditions:

- `Money.NewMinor(USD)` is `9.249 ns/op`; direct `govalues` minor construction
  is `8.021 ns/op`. That is about `1.15x`, not `3x`.
- `Money.NewMinor`, `Money.MinorUnits`, `Money.Add`, and `Money.Sum` are all
  `0 allocs/op`.
- No caller workflow in #180 requires long-backed minor-unit storage as a public
  type boundary.

JSON serialization and text parsing allocate as expected for serialization and
parsing work. They do not justify a parallel long-backed public amount type.

## Comparison

### JVM FastMoney Intent

The JVM `FastMoneySupport.kt` reference exists in a Moneta/JVM ecosystem where a
long-backed helper can be useful for minor-unit-heavy operations. That intent is
valid for the JVM package, but it does not automatically justify copying the
same public surface into Go.

### Current Go Money

The Go `Money` type is a bluetape-go wrapper over `github.com/govalues/money`.
It is immutable, decimal-backed, currency-aware, serialization-friendly, and
already exposes `NewMinor` plus `MinorUnits` for minor-unit input and output.

### Direct govalues/money

The direct `govalues/money` benchmark row is a reference row for wrapper
overhead only. It is not a recommendation to leak upstream concrete types into
bluetape-go public APIs.

### Rhymond/go-money

`Rhymond/go-money` remains an active MIT Fowler-style integer minor-unit Go
package. It is useful prior art for integer-minor-unit APIs, but it has a
narrower decimal and provider-backed exchange-rate story than the current
bluetape-go `money` package.

## Follow-Up

No `FastMoney` implementation follow-up is required from this benchmark.

Future work should open a new issue only if a production caller records a
measured hot-path gap or a public contract need that cannot be served by
`Money`, `NewMinor`, and `MinorUnits`.
