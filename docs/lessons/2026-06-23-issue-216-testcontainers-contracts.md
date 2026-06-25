# Issue #216 Testcontainers contract hardening lessons

Date: 2026-06-23
Issue: #216

## What changed

- Existing Testcontainers wrappers already had bounded cleanup, but the public
  failure text was too generic for callers and CI operators.
- Connection detail names were implied by return values only; downstream
  examples had no stable key names for maps, reports, or fixture metadata.
- Docker smoke tests used `t.Parallel()`, which conflicts with milestone 0.6.5's
  serial resource-containment rule.

## Evidence that resolved it

- `go test -p 1 -count=1 ./testcontainers/...` passed before changes, proving
  the wrappers worked on the happy path.
- New synthetic start-error tests proved the desired diagnostic categories
  without forcing Docker daemon/image failures locally.
- Cleanup tests now cover skipped subtests and repeated terminate calls.
- Repo-wide `make test` and `make race` both passed after removing Testcontainers
  `t.Parallel()` calls.

## Next time

- For Testcontainers helper issues, add synthetic diagnostic tests first and use
  real Docker only for happy-path smoke and race gates.
- Keep README examples aligned with actual API keys and serial test commands.
- If `golangci-lint` reports paths from removed worktrees, clean its cache before
  treating the report as a current-branch failure.
