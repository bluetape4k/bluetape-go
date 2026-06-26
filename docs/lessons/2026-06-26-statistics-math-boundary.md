# Issue #277 Lessons

- Numeric helpers need domain ownership. Empty input, NaN, Inf, overflow,
  weighting, and precision rules are different for money, rate limiting,
  benchmarks, and data analysis.
- A broad Apache Commons Math source module should not become a broad Go
  utility package without repeated Go callers.
- Treat Gonum as a strong future candidate for real statistics/data work, not
  as a repo-wide dependency for convenience helpers.
