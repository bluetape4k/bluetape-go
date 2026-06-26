# Issue #277 Statistics And Math Scope

Issue: #277
Parent: #7
Date: 2026-06-26

## Decision

Close #277 as research-only. Do not add a generic `math`, `stats`,
`statistics`, `histogram`, `regression`, `interpolation`, or special-functions
package during the 0.7.0 research gate.

The current Go-native direction is:

- keep package-local numeric logic inside the package that owns the domain
  contract, such as `money`, `ratelimit`, `measure`, and pagination helpers;
- use the standard library `math` package for scalar numeric functions and
  explicit NaN/Inf checks;
- revisit `gonum` only when a concrete data/science/statistics package needs a
  broad numerical toolkit;
- avoid Apache Commons Math shaped convenience APIs until repeated bluetape-go
  callers prove the package boundary.

No implementation follow-up is filed from this issue.

## Source Inventory

| Source module | Capability | Go decision |
|---|---|---|
| `bluetape4k-projects/utils/math` descriptives | descriptive statistics, comparable statistics, histograms, variance, skewness, kurtosis | Defer. No current bluetape-go caller needs a shared statistics package. |
| `utils/math/commons` | moving average/sum, arithmetic helpers, precision, approximate equality, primes, combinations, roots, norms | Reject as broad utility port. Keep small numeric validation package-local. |
| `utils/math/interpolation` | linear, spline, Akima, Loess, Neville interpolation over Apache Commons Math | Defer to a future data/science package. This is not a general infrastructure helper. |
| `utils/math/special` | gamma, beta, factorial, harmonic, stability helpers | Defer to a future numerical/science consumer. Do not hand-port special functions. |

## Go Ecosystem Candidates

| Candidate | Use if future consumer appears | Current decision |
|---|---|---|
| Standard `math` | Scalar functions, NaN/Inf checks, rounding, min/max, powers | Keep as current default. |
| `gonum.org/v1/gonum/stat` | Means, variance, covariance, correlation, histogram, quantiles, regression, ROC/TOC | Candidate only for a concrete statistics/data package. |
| `gonum.org/v1/gonum/floats` | Slice operations over float64 data | Candidate only when a caller already accepts Gonum-style slice contracts. |
| `gonum.org/v1/gonum/mat` / broader Gonum | Matrix, linear algebra, distribution, optimization use cases | Reject until a domain package proves the broad dependency is justified. |

## Rejected

- Porting Kotlin extension-style math helpers for convenience.
- Exposing approximate equality or precision helpers without domain-specific
  tolerance rules.
- Adding histogram/regression/interpolation APIs without data shape, weighting,
  sorting, NaN/Inf, and empty-input contracts.
- Adding Gonum as a repo-wide dependency before a concrete package owns the
  numerical problem.
- Replacing existing `money` decimal-backed calculations or `ratelimit`
  package-local math with shared helpers.

## Required Pattern For Future Math Work

Any future issue must define:

1. the owning package and real caller;
2. input type, mutability, sorting, weighting, and allocation expectations;
3. empty input, NaN, Inf, overflow, underflow, and precision behavior;
4. whether population versus sample statistics are required;
5. benchmark or data-volume evidence for hot-path helpers;
6. dependency decision, especially whether standard `math` is enough or Gonum
   is justified.

## Validation

- Current bluetape-go search shows only package-local standard `math` use.
- Existing `money`, `measure`, `ratelimit`, and pagination code already own
  their numeric contracts directly.
- The JVM source module is broad and Apache Commons Math shaped; no current Go
  consumer proves an equivalent shared package boundary.
