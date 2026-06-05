# Go Coverage Artifacts Lessons

## Context

- Issue #125 adds coverage reporting to CI and Nightly for `bluetape-go`.
- The reference pattern from bluetape4k JVM repositories is Kover XML generation,
  artifact upload, and GitHub Step Summary aggregation.

## Decision

- Use Go native coverage before adding external coverage SaaS integration.
- Generate `coverage.out`, `coverage-packages.md`, `coverage.txt`, and
  `coverage.html` through one `make coverage` target.
- Upload raw local reports as workflow artifacts and write the text summary to
  `$GITHUB_STEP_SUMMARY`.
- Keep Step Summary focused on package-level subtotals; full function-level
  output belongs in the uploaded artifact.

## Learnings

- Go does not need a Kover-style aggregation script while this repository is a
  single Go module.
- Keeping race tests separate avoids mixing coverage instrumentation with race
  detector overhead.
- Workflow validation must include `actionlint` and a search for escaped single
  quotes before push.
- `go tool cover -func` output is file-path ordered, so `tail` shows arbitrary
  last-path functions instead of representative coverage.
- Package subtotals can be derived from the Go coverage profile by summing
  statement blocks per source package.

## Follow-up

- Revisit Coveralls/Codecov upload and coverage thresholds after a stable
  baseline exists for `0.3.0`.
