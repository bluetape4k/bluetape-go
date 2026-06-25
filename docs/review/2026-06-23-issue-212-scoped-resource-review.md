# Issue 212 scoped test resource helpers review

Date: 2026-06-23
PR: #259
Issue: #212
Scope: `testing` package scoped temp output, environment, stdout/stderr capture helpers, examples, and README updates.

## Verdict

P0=0 P1=0

## Evidence

- `go test -count=1 ./testing/...`
- `go test -race -count=1 ./testing`
- `make fmt-check tidy-check vet lint`
- `make test`
- `make race`
- GitHub CI for initial PR commit: success before review-artifact repair commit.

## 7-Tier Notes

- Performance: output capture uses a package mutex and short-lived OS pipes only around the caller function; no production hot path is touched.
- Stability: env restoration, output restoration, nil callback diagnostics, and panic restoration are covered by tests; full race gate passed.
- Security: temp output path helpers reject empty roots, absolute path parts, and parent traversal after cleaning.
- Operator/Ops: README and Go doc explicitly state process-global stdout/stderr and environment caveats.
- Developer/API: helpers use `testing.TB`, keep `t.TempDir` and `t.Setenv` as the preferred simple-case APIs, and avoid broad test framework abstractions.
- User/caller: examples cover path validation and output capture without suggesting unsafe parallel use.
- Integration: change is scoped to `testing` package docs/tests/helpers and does not alter other package behavior.

## Follow-Up Risk

None blocking. Do not use env or stdout/stderr capture helpers from parallel tests unless the implementation no longer mutates process-global state.
