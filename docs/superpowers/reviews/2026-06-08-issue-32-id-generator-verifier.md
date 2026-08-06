# Issue 32 ID Generator Verifier

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #32
Milestone: 0.6.0
범위: `id` package foundation, UUID v4/v7, random/monotonic ULID, Snowflake,
docs, tests, benchmarks, and release notes.

## 구현 범위

| Plan task | Status | Evidence |
|---|---|---|
| T0 dependency/API decision | PASS | `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-preimplementation-risk.md`. |
| T1 common API/errors | PASS | `id/generator.go`, `id/errors.go`, `id/errors_test.go`; final API exposes narrow `StringGenerator` and `Int64Generator` contracts, not an unused generic `Generator[T]`. |
| T2 UUID v4/v7 | PASS | `id/uuid.go`, `id/uuid_test.go`. |
| T3 random/monotonic ULID | PASS | `id/ulid.go`, `id/ulid_test.go`. |
| T4 Snowflake | PASS | `id/snowflake.go`, `id/snowflake_test.go`. |
| T5 stress/race/cancellation applicability | PASS | `id/id_concurrency_test.go`; `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-concurrency-notes.md`. |
| T6 benchmarks | PASS | `id/id_benchmark_test.go`. |
| T7 package docs/examples | PASS | `id/README.md`, `id/README.ko.md`, `id/id_example_test.go`, `id/doc.go`. |
| T8 root docs/release notes | PASS | `README.md`, `README.ko.md`, `CHANGELOG.md`, `WIP.md`. |
| T9 validation | PASS | Commands below. |

## 검증 증거

Commands passed:

```bash
gofmt -w id
go mod tidy
go test -count=1 ./id
go test -count=1 ./id -run Example
go test -race -count=1 ./id
go test -count=1 ./id -run 'TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v
go test -race -count=1 ./id -run 'TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v
GOMAXPROCS=1 go test -count=100 ./id -run TestGUIDGeneratorsStayUniqueAcrossGoroutines
go test -run '^$' -bench . -benchmem ./id
go test -count=1 ./...
make ci
git diff --check
git diff --cached --check
```

Benchmark smoke evidence from `go test -run '^$' -bench . -benchmem ./id`:

| Benchmark | Result |
|---|---:|
| `BenchmarkUUIDV4-10` | 223.1 ns/op, 88 B/op, 3 allocs/op |
| `BenchmarkUUIDV7-10` | 240.9 ns/op, 88 B/op, 3 allocs/op |
| `BenchmarkULIDRandom-10` | 97.03 ns/op, 48 B/op, 2 allocs/op |
| `BenchmarkULIDMonotonicParallel-10` | 190.3 ns/op, 48 B/op, 2 allocs/op |
| `BenchmarkSnowflakeNextInt64-10` | 11.19 ns/op, 0 B/op, 0 allocs/op |
| `BenchmarkSnowflakeNextInt64Parallel-10` | 88.07 ns/op, 0 B/op, 0 allocs/op |

Post-review rerun after Snowflake overflow guard and API/doc polish:

| Benchmark | Result |
|---|---:|
| `BenchmarkUUIDV4-10` | 257.3 ns/op, 88 B/op, 3 allocs/op |
| `BenchmarkUUIDV7-10` | 286.5 ns/op, 88 B/op, 3 allocs/op |
| `BenchmarkULIDRandom-10` | 99.64 ns/op, 48 B/op, 2 allocs/op |
| `BenchmarkULIDMonotonicParallel-10` | 208.1 ns/op, 48 B/op, 2 allocs/op |
| `BenchmarkSnowflakeNextInt64-10` | 11.45 ns/op, 0 B/op, 0 allocs/op |
| `BenchmarkSnowflakeNextInt64Parallel-10` | 98.22 ns/op, 0 B/op, 0 allocs/op |

Post-review Snowflake same-millisecond sequence benchmark:

| Benchmark | Result |
|---|---:|
| `BenchmarkSnowflakeNextInt64-10` | 11.35 ns/op, 0 B/op, 0 allocs/op |
| `BenchmarkSnowflakeNextInt64SameMillisecond-10` | 10.47 ns/op, 0 B/op, 0 allocs/op |
| `BenchmarkSnowflakeNextInt64Parallel-10` | 93.95 ns/op, 0 B/op, 0 allocs/op |

Final pre-PR benchmark smoke:

| Benchmark | Result |
|---|---:|
| `BenchmarkUUIDV4-10` | 229.0 ns/op, 88 B/op, 3 allocs/op |
| `BenchmarkUUIDV7-10` | 256.0 ns/op, 88 B/op, 3 allocs/op |
| `BenchmarkULIDRandom-10` | 101.7 ns/op, 48 B/op, 2 allocs/op |
| `BenchmarkULIDMonotonicParallel-10` | 205.5 ns/op, 48 B/op, 2 allocs/op |
| `BenchmarkSnowflakeNextInt64-10` | 11.79 ns/op, 0 B/op, 0 allocs/op |
| `BenchmarkSnowflakeNextInt64SameMillisecond-10` | 10.42 ns/op, 0 B/op, 0 allocs/op |
| `BenchmarkSnowflakeNextInt64Parallel-10` | 96.37 ns/op, 0 B/op, 0 allocs/op |

Repository-wide `go test -count=1 ./...` passed, including Testcontainers-backed
packages.

`make ci` passed after staging the intended `go.mod`/`go.sum` dependency
changes so `tidy-check` could compare the tidied working tree against the staged
source.

GUID uniqueness stress evidence:

- `TestGUIDGeneratorsStayUniqueAcrossGoroutines` covers UUID v4, UUID v7,
  random ULID, and monotonic ULID.
- Each subtest generates `512 * 16 = 8192` IDs from one shared generator through
  a 64-worker goroutine pool and asserts generated count, map cardinality, and
  duplicate-free completion.
- Targeted normal, `-race`, and `GOMAXPROCS=1 -count=100` runs passed. The
  stress test intentionally avoids scheduler-dependent `MaxConcurrent`
  assertions.

## Documentation Checks

Commands passed:

```bash
rg -n "GoroutineStressTester|AsyncJobTester N/A: single generation has no caller-observable cancellation boundary" id docs/superpowers/reviews
rg -n "UUID v7|ULID|Snowflake|authentication|authorization|secret|standalone security boundary|unique.*machine ID|duplicate.*machine ID|wall-clock rollback|KSUID.*#166|Flake|Hashids|zero-value|name-based" id/README.md id/README.ko.md id/doc.go
rg -n "UUID parsing accepts only canonical|Base62 deferred|snowflakeMaxTimestamp|RejectsTimestampOverflow|RejectsInvalidOptions|RejectsNonCanonical" id docs/superpowers/reviews README.md README.ko.md WIP.md CHANGELOG.md
rg -n "id|0.5.1|0.6.0|v0.5.1|v0.5.0" README.md README.ko.md CHANGELOG.md WIP.md
```

## 의존성 증거

`go.mod` now lists direct dependencies:

- `github.com/google/uuid v1.6.0`
- `github.com/oklog/ulid/v2 v2.1.1`

The public `id` API returns strings, int64 values, and repo-owned structs or
interfaces. It does not expose dependency UUID/ULID concrete types as stable API.

## Step 6 Verifier Verdict

PASS. Implementation and validation evidence are ready for Step 6-R subagent
7-Tier code review.
