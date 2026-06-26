# Issue #277 Statistics Research Review

Issue: #277
Branch: `research/issue-277-stats-scope`
Date: 2026-06-26

## Scope

Docs-only research boundary for focused statistics, numeric helpers,
histograms, regression, interpolation, precision, and special functions.

## 7-Tier Review

| Tier | Lens | P0 | P1 | Verdict | Evidence |
|---|---:|---:|---:|---|---|
| 1 | Security | 0 | 0 | PASS | No runtime code, secrets, IO, or unsafe numeric parser surface added. |
| 2 | Performance | 0 | 0 | PASS | Avoids generic helpers and broad Gonum dependency without benchmark/data-volume evidence. |
| 3 | Stability | 0 | 0 | PASS | Keeps NaN/Inf/overflow rules package-local instead of centralizing unclear semantics. |
| 4 | Operator/Ops | 0 | 0 | PASS | No build, native, CI, runtime, or deployment changes. |
| 5 | Developer/API | 0 | 0 | PASS | Rejects Kotlin/Apache-Commons-shaped convenience APIs until caller demand exists. |
| 6 | User/Caller | 0 | 0 | PASS | Prevents broad API promises around population/sample, weighting, sorting, and precision behavior. |
| 7 | Evidence | 0 | 0 | PASS | Grounded in #223, `utils/math`, current repo math usage, standard `math`, and Gonum candidate docs. |

P0=0 P1=0

## Residual P2/P3

- P2: If a future analytics/data package appears, evaluate Gonum with a
  package-specific dependency and test plan instead of reviving a broad utility
  port.
