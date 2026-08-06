# Issue #35 Money Concurrency Notes

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

- `GoroutineStressTester`: `money/money_concurrency_test.go` repeatedly parses,
  serializes, adds same-currency values, and rejects cross-currency operations
  across goroutines with explicit worker, round, and timeout options.
- Race gate: `go test -race -count=1 ./money`.
- AsyncJobTester N/A: #35 money core has no context-aware async, provider, IO, or cancellation boundary.
