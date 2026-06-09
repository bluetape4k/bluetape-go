# Issue #36 Concurrency Notes

Scope: `probabilistic` in-memory Bloom filter.

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
