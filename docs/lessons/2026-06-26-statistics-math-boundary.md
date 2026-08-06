# Issue #277 교훈

- Numeric helper에는 domain ownership이 필요하다. Empty input, NaN, Inf,
  overflow, weighting, precision 규칙은 money, rate limiting, benchmark, data
  analysis마다 다르다.
- 넓은 Apache Commons Math source module을 반복되는 Go caller 없이 넓은 Go utility
  package로 바꾸지 않는다.
- Gonum은 실제 statistics/data 작업을 위한 강한 미래 후보로 본다. Convenience
  helper를 위해 repo-wide dependency로 넣지는 않는다.
