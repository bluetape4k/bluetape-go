# Issue #277 Statistics And Math Scope

Issue: #277
Parent: #7
Date: 2026-06-26

## 결정

#277은 research-only로 닫는다. 0.7.0 research gate에서는 generic `math`,
`stats`, `statistics`, `histogram`, `regression`, `interpolation`,
special-functions package를 추가하지 않는다.

현재 Go-native 방향은 다음과 같다.

- numeric logic은 `money`, `ratelimit`, `measure`, pagination helper처럼
  domain contract를 소유한 package 안에 package-local로 둔다.
- scalar numeric function과 explicit NaN/Inf check에는 standard library
  `math` package를 사용한다.
- `gonum`은 구체적인 data/science/statistics package가 broad numerical
  toolkit을 필요로 할 때만 재검토한다.
- 반복되는 bluetape-go caller가 package boundary를 증명하기 전까지 Apache
  Commons Math shaped convenience API는 피한다.

이 이슈에서 구현 follow-up은 만들지 않는다.

## 소스 인벤토리

| Source module | Capability | Go decision |
|---|---|---|
| `bluetape4k-projects/utils/math` descriptives | descriptive statistics, comparable statistics, histograms, variance, skewness, kurtosis | Defer. 현재 bluetape-go caller는 shared statistics package를 필요로 하지 않는다. |
| `utils/math/commons` | moving average/sum, arithmetic helpers, precision, approximate equality, primes, combinations, roots, norms | Broad utility port로 기각한다. 작은 numeric validation은 package-local로 유지한다. |
| `utils/math/interpolation` | linear, spline, Akima, Loess, Neville interpolation over Apache Commons Math | 향후 data/science package로 보류한다. general infrastructure helper가 아니다. |
| `utils/math/special` | gamma, beta, factorial, harmonic, stability helpers | 향후 numerical/science consumer로 보류한다. special function을 hand-port하지 않는다. |

## Go 생태계 후보

| Candidate | Use if future consumer appears | Current decision |
|---|---|---|
| Standard `math` | Scalar functions, NaN/Inf checks, rounding, min/max, powers | 현재 default로 유지한다. |
| `gonum.org/v1/gonum/stat` | Means, variance, covariance, correlation, histogram, quantiles, regression, ROC/TOC | concrete statistics/data package가 있을 때만 candidate이다. |
| `gonum.org/v1/gonum/floats` | Slice operations over float64 data | caller가 이미 Gonum-style slice contract를 받아들일 때만 candidate이다. |
| `gonum.org/v1/gonum/mat` / broader Gonum | Matrix, linear algebra, distribution, optimization use cases | domain package가 broad dependency의 필요성을 증명할 때까지 기각한다. |

## 기각

- 편의를 위해 Kotlin extension-style math helper를 port하는 것.
- domain-specific tolerance rule 없이 approximate equality 또는 precision
  helper를 노출하는 것.
- data shape, weighting, sorting, NaN/Inf, empty-input contract 없이
  histogram/regression/interpolation API를 추가하는 것.
- 구체 package가 numerical problem을 소유하기 전에 Gonum을 repo-wide
  dependency로 추가하는 것.
- 기존 `money` decimal-backed calculation이나 `ratelimit` package-local
  math를 shared helper로 대체하는 것.

## 향후 Math 작업 필수 패턴

향후 이슈는 다음을 정의해야 한다.

1. owning package와 real caller.
2. input type, mutability, sorting, weighting, allocation expectations.
3. empty input, NaN, Inf, overflow, underflow, precision behavior.
4. population versus sample statistics가 필요한지 여부.
5. hot-path helper라면 benchmark 또는 data-volume evidence.
6. dependency decision, 특히 standard `math`만으로 충분한지 또는 Gonum이
   정당화되는지.

## 검증

- 현재 bluetape-go search에서는 package-local standard `math` 사용만 보인다.
- 기존 `money`, `measure`, `ratelimit`, pagination code는 각자의 numeric
  contract를 직접 소유한다.
- JVM source module은 broad하고 Apache Commons Math shaped이다. 현재 Go
  consumer는 동등한 shared package boundary를 증명하지 않는다.
