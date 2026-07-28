# Issue #180 FastMoney 평가

## 결정

#180에서는 public long-backed `FastMoney` type을 추가하지 않는다.

측정한 local benchmark snapshot은 `money` public API를 복제하기 위한 승인 threshold를
넘지 않는다. 이 machine에서 `Money.NewMinor`는 direct `govalues/money` minor-unit
constructor보다 약 1.15x 느리지만, 두 path 모두 operation당 object allocation이 0이고
기존 `Money.MinorUnits`, `Money.Add`, `Money.Sum` hot path도 operation당 object
allocation이 0이다.

기존 public API는 그대로 유지한다.

- integer minor-unit input에는 `NewMinor`.
- integer minor-unit extraction에는 `MinorUnits`.
- decimal-backed immutable amount behavior에는 `Money`.

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

raw output은 다음 위치에 저장한다.

```text
docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt
```

이를 numeric source of truth로 다룬다. chart는 같은 row를 scan-friendly하게 시각화한
것일 뿐이다. 낮은 `ns/op`, 낮은 `B/op`, 낮은 `allocs/op`가 더 좋다.

## 결과

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

## 해석

public `FastMoney` follow-up에 대한 승인 threshold는 다음과 같았다.

- `NewMinor`, `MinorUnits`, `Add`, `Sum` 중 하나가 같은 operation family의 direct
  `govalues` reference보다 최소 3x 느린 경우, 또는
- 가장 단순한 minor-unit path가 5 allocs/op를 초과하고 reference path는 near zero에
  머무는 경우, 또는
- public type boundary로 long-backed minor-unit storage가 필요한 caller workflow가
  문서화된 경우.

benchmark는 이 조건을 충족하지 않는다.

- `Money.NewMinor(USD)`는 `9.249 ns/op`이고 direct `govalues` minor construction은
  `8.021 ns/op`다. 이는 약 `1.15x`이지 `3x`가 아니다.
- `Money.NewMinor`, `Money.MinorUnits`, `Money.Add`, `Money.Sum`은 모두
  `0 allocs/op`다.
- #180에는 public type boundary로 long-backed minor-unit storage가 필요한 caller
  workflow가 없다.

JSON serialization과 text parsing은 serialization 및 parsing 작업상 예상대로
allocation이 발생한다. 이것만으로 parallel long-backed public amount type을 정당화할
수 없다.

## 비교

### JVM FastMoney Intent

JVM `FastMoneySupport.kt` reference는 minor-unit-heavy operation에 long-backed helper가
유용할 수 있는 Moneta/JVM ecosystem에 존재한다. 그 intent는 JVM package에서는
타당하지만, 같은 public surface를 Go로 자동 복제할 근거는 아니다.

### Current Go Money

Go `Money` type은 `github.com/govalues/money` 위의 bluetape-go wrapper다. immutable,
decimal-backed, currency-aware, serialization-friendly이며 minor-unit input/output을
위해 이미 `NewMinor`와 `MinorUnits`를 노출한다.

### Direct govalues/money

direct `govalues/money` benchmark row는 wrapper overhead만 보기 위한 reference row다.
upstream concrete type을 bluetape-go public API로 leak하자는 recommendation이 아니다.

### Rhymond/go-money

`Rhymond/go-money`는 active MIT Fowler-style integer minor-unit Go package로 남아
있다. integer-minor-unit API에 대한 유용한 prior art지만, 현재 bluetape-go `money`
package보다 decimal 및 provider-backed exchange-rate story가 좁다.

## Follow-Up

이 benchmark에서 `FastMoney` implementation follow-up은 필요하지 않다.

future work는 production caller가 measured hot-path gap을 기록하거나 `Money`,
`NewMinor`, `MinorUnits`로 충족할 수 없는 public contract need를 기록한 경우에만 새
issue를 열어야 한다.
