# Issue #421 Rules Benchmark Review

Issue: [#421](https://github.com/bluetape4k/bluetape-go/issues/421)
Branch: `feat/issue-421-rules-bench`
Baseline: `d29b86e`

## Scope

- `Makefile`
- `rules/rules_benchmark_test.go`
- `docs/research/outputs/issue-421/environment.md`
- `docs/research/outputs/issue-421/rules-benchmark.txt`

## Evidence

- Issue #421 requires benchmark command/raw output preservation, measured evidence for future optimization follow-ups, and existing rules behavior remaining covered by tests.
- `Makefile:11`, `Makefile:28`, and `Makefile:82` expose `bench-rules` as a phony, help-listed, opt-in benchmark target.
- `rules/rules_benchmark_test.go:9`, `rules/rules_benchmark_test.go:55`, `rules/rules_benchmark_test.go:97`, `rules/rules_benchmark_test.go:147`, and `rules/rules_benchmark_test.go:215` cover composite activation, unit, conditional, bounded inference, and sequential engine hot paths.
- `docs/research/outputs/issue-421/environment.md:27` records the benchmark command and `docs/research/outputs/issue-421/rules-benchmark.txt:5` preserves the raw benchmark rows.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | P0=0 P1=0. Noted P3 that composite rows are steady-state `Facts` reuse, now documented in `environment.md`. |
| Stability | PASS after evidence | P0=0 P1=0. Fresh `go test -count=1 ./rules` and `go test -race -count=1 ./rules` recorded by main session. |
| Security | PASS after hygiene repair | P0=0 P1=0. Hostname fingerprint was removed from `environment.md`; artifact scan found no local paths or secret-like tokens. |
| Operator/Ops | PASS after repair | P0=0 P1=0. `bench-rules` added to `.PHONY`, `make help`, and aligned with `-count=5`. |
| Developer/API | PASS after repair | P0=0 P1=0. Mixed inference row split into stable `Count0` and `Count1` sub-benchmarks with expected `Applied` counts. |
| User/Caller | PASS after repair | P0=0 P1=0. `bench-rules` is discoverable and reproduces the preserved evidence command. |
| Integration | PASS | Main-session review accepted the benchmark-only scope and confirmed no production API behavior changes. |

## Validation

| Command | Status | Evidence |
|---|---|---|
| `git diff --check` | PASS | No whitespace errors. |
| `make fmt-check` | PASS | Go files are formatted. |
| `make vet` | PASS | Vet completed after benchmark helper format-string repair. |
| `make lint` | PASS | GolangCI-Lint completed with 0 issues after the benchmark helper parameter-order repair. |
| `go test -count=1 ./rules` | PASS | Rules behavior test suite passed after benchmark addition. |
| `go test -race -count=1 ./rules` | PASS | Rules race gate passed after benchmark addition. |
| `make help \| rg 'bench-rules\|bench-id\|Targets'` | PASS | `bench-rules` appears in help output. |
| `make -n bench-rules` | PASS | Dry-run prints `go test -run '^$' -bench '^BenchmarkRules' -benchmem -count=5 ./rules`. |
| `make bench-rules` | PASS | Raw `-count=5` benchmark rows preserved under `docs/research/outputs/issue-421/rules-benchmark.txt`. |
| Artifact hygiene scan | PASS | `rg -n "/Users\|debop\|TOKEN\|SECRET\|PASSWORD\|GITHUB_\|sk-\|ghp_\|github_pat\|BEGIN .*PRIVATE\|PRIVATE KEY" docs/research/outputs/issue-421` returned no matches. |

## Findings

P0=0 P1=0

- P2 resolved: `bench-rules` missing from `.PHONY` and `make help`.
- P2 resolved: `bench-rules` did not reproduce the preserved `-count=5` command shape.
- P2 resolved: `BenchmarkRulesInferenceConverges` mixed `Count0` and `Count1` workloads in one row.
- P3 resolved: `environment.md` exposed local hostname/kernel fingerprint.
- P3 documented: composite benchmark rows measure steady-state `Facts` reuse, not fresh request construction.

## Residual Risk

The retained benchmark evidence is a local comparable snapshot, not a production ranking. Future optimization work should compare baseline and candidate output with `benchstat`, and should add fresh-`Facts` variants if request-style allocation cost becomes the optimization target.
