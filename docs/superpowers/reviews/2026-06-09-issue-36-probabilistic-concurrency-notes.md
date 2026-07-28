# Issue #36 Concurrency Notes

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

범위: `probabilistic` in-memory Bloom filter.

- `GoroutineStressTester`: used for concurrent `Put`, `MightContain`,
  metadata reads, reciprocal `PutAll`, self-merge, and `Clear` interaction.
- `go test -race -count=1 ./probabilistic`: required and passed during local
  implementation validation.
- `AsyncJobTester: N/A`

Reason: #36 exposes no `context.Context`, background goroutine, timer, external
I/O, Testcontainers, Redis, HTTP, file, or cancellation boundary. The package is
short CPU/memory work protected by a local mutex.

Verification:

```bash
rg -n "AsyncJobTester: N/A" docs/superpowers/reviews/2026-06-09-issue-36-probabilistic-concurrency-notes.md
```
