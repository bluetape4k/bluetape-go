# Issue 32 ID Generator Concurrency Notes

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #32
Plan task: T5

## Stress Coverage

`id/id_concurrency_test.go` uses `testing/concurrency.GoroutineStressTester` to
exercise concurrent UUID v4, UUID v7, monotonic ULID, and Snowflake generation.
The mixed-generator smoke test checks that concurrent calls do not duplicate IDs
in the exercised stress window.

`TestGUIDGeneratorsStayUniqueAcrossGoroutines` separately stress-tests UUID v4,
UUID v7, random ULID, and monotonic ULID through the same generator instance from
a 64-worker goroutine pool. Each subtest generates `512 * 16 = 8192` IDs and
asserts the generated count, map cardinality, and duplicate-free completion.

Commands passed:

```bash
go test -count=1 ./id -run 'TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v
go test -race -count=1 ./id -run 'TestGUIDGeneratorsStayUniqueAcrossGoroutines|TestGeneratorsAreConcurrentSafe' -v
GOMAXPROCS=1 go test -count=100 ./id -run TestGUIDGeneratorsStayUniqueAcrossGoroutines
```

## Cancellation Applicability

AsyncJobTester N/A: single generation has no caller-observable cancellation boundary.

The `id` package does not add a context-aware batch helper in this issue.
Generation is local CPU/entropy work and each public single-ID method returns
directly with either a generated ID or an error. If a future batch helper is
added, it should own `context.Context` cancellation and use `AsyncJobTester`.
