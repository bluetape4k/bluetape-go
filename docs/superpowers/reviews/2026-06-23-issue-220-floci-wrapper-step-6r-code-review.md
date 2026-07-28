# Issue #220 Floci Wrapper Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: [#220](https://github.com/bluetape4k/bluetape-go/issues/220)
브랜치: `issue-220-aws-graph-infra-fixtures`
기준: `origin/develop` at `1c4f5d4`
날짜: 2026-06-23

## 범위

Reviewed the Floci fixture implementation slice:

- `testcontainers/floci/doc.go`
- `testcontainers/floci/floci.go`
- `testcontainers/floci/floci_test.go`
- `testcontainers/floci/README.md`
- `testcontainers/floci/README.ko.md`
- `go.mod`
- `go.sum`
- Issue #220 Floci spec and plan amendments

This gate used main integration fallback for all six 7-tier perspectives, per
the current user instruction. No subagent verdict is treated as required
evidence for this gate.

## 검토 관점

| Lane | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Normal `go test ./...` skips the Docker-backed S3 smoke; opt-in smoke remains explicit through `BLUETAPE_FLOCI_SMOKE=1`. No hot-path code is added. |
| Stability | 0 | 0 | 0 | 0 | `StartContainer` registers bounded cleanup through `context.WithoutCancel(ctx)` plus `testcleanup.DefaultTerminateTimeout`; S3 response bodies are closed; sequential Docker reruns passed after a parallel smoke procedure error. |
| Security | 0 | 0 | 0 | 0 | `LoadConfig` uses Floci-local static test credentials from `Details`; no production AWS credential lookup is introduced by the helper contract. |
| Operator/Ops | 0 | 0 | 0 | 0 | README pair documents Docker runtime, dynamic endpoint, serial Testcontainers execution, pseudo-version/default-image drift, and opt-in smoke expectations. |
| Developer/API | 0 | 0 | 0 | 0 | API stays narrow: `Start`, `StartContainer`, `DetailsFromContainer`, `Details.ConnectionDetails`, and `LoadConfig`; service-specific AWS clients remain caller-owned. |
| User/Caller | 0 | 0 | 0 | 0 | README pair includes import path, `UsePathStyle`, env export keys, opt-in test command, and deferrals to #61/#62/#63/#64/#50/#44. |
| Main integration | 0 | 0 | 0 | 0 | Diff is scoped to the #220 first Floci slice; PR remains stackable and unmerged by current policy. |

Final gate: `P0=0 P1=0`.

## 발견 사항

No blocking findings remain.

Resolved during Step 6-R:

- Stability: cleanup originally used the caller's start context directly for
  `Stop(ctx)`. Fixed to use a bounded cleanup context that preserves values but
  ignores parent cancellation.
- Procedure: one local verification command incorrectly ran the opt-in Floci
  smoke in parallel with adjacent Docker packages, producing Kafka/MariaDB
  connection refused failures. Re-ran all affected Docker packages sequentially
  and recorded the passing evidence below.

## 검증

RED evidence:

- `go test -p 1 -count=1 ./testcontainers/floci` failed before dependencies and
  implementation with missing AWS SDK module errors.

Targeted validation:

- `go test -p 1 -count=1 ./testcontainers/floci ./testcontainers/kafka ./testcontainers/mariadb` passed before the cleanup repair.
- `BLUETAPE_FLOCI_SMOKE=1 go test -p 1 -count=1 ./testcontainers/floci` passed before and after the cleanup repair.
- `BLUETAPE_FLOCI_SMOKE=1 go test -race -p 1 -count=1 ./testcontainers/floci` passed before the cleanup repair.
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/floci` passed before and after the cleanup repair.
- After the procedure-error parallel smoke failure, sequential rerun passed:
  `go test -p 1 -count=1 ./testcontainers/kafka ./testcontainers/mariadb &&
  go test -p 1 -count=1 ./testcontainers/floci ./testcontainers/kafka ./testcontainers/mariadb &&
  BLUETAPE_FLOCI_SMOKE=1 go test -p 1 -count=1 ./testcontainers/floci &&
  go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/floci`.

Repository validation:

- `make fmt-check` passed.
- `make vet` passed.
- `make lint` passed with `0 issues.`
- `git diff --check` passed.
- `make test && make race` passed on the latest diff after the cleanup repair.

Documentation checks:

- `rg -n "floci.endpoint|BLUETAPE_FLOCI_ENDPOINT|UsePathStyle|S3|#61|#62|#63|#64" testcontainers/floci/README.md testcontainers/floci/README.ko.md` found the required README coverage.
- Go quick scan over `testcontainers/floci` found only intentional
  `context.Background()` use in tests and README snippets.

## Step 6 Checklist Completion Report

| 항목 | 상태 | Notes |
|---|---|---|
| Implemented diff reviewed | Done | Floci package, README pair, dependency changes, spec/plan amendments. |
| 7-tier review run | Done | Main integration fallback for six perspectives plus integration lane. |
| P0/P1 convergence | Done | `P0=0 P1=0`. |
| Targeted tests | Done | Normal, opt-in smoke, and race-targeted commands passed. |
| Repo checks | Done | `make fmt-check`, `make vet`, `make lint`, `make test && make race`, `git diff --check`. |
| Testcontainers serial discipline | Done | Parallel smoke procedure error corrected by sequential rerun. |
