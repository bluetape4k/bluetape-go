# Issue 30 Batch Policies Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- Baseline: `origin/develop` at `60d8591f68380d5526de79e7b221c18863987352`.
- Diff: retry, skip, checkpoint, and restart support in the `batch` package.
- Files: `batch/checkpoint.go`, `batch/policy.go`, `batch/policy_test.go`, `batch/job.go`, `batch/report.go`, `batch/step.go`.

## 수용 기준 증거

- Retry and skip policies are composable through `StepOptions.RetryPolicy` and `StepOptions.SkipPolicy`.
- `CheckpointStore` is a pluggable interface, and `MemoryCheckpointStore` provides the local/test implementation.
- `CheckpointReader` lets readers restore and save progress without forcing checkpoint methods onto every `Reader`.
- Restart behavior is covered by `TestStepRunRestartsFromCheckpoint`.
- Context cancellation is never retried or skipped; `TestRetrySkipPoliciesDoNotHandleContextCancellation` verifies this with `AsyncJobTester`.
- New policy/checkpoint code has stress coverage through `TestMemoryCheckpointStoreWithGoroutineStressTester`.

## 로컬 검토

- P0: none.
- P1: none.
- P2: none.
- P3: `code-review-graph detect-changes --base origin/develop --brief` reports interface-declaration test gaps for newly exported `CheckpointReader`, `CheckpointStore`, and `MemoryCheckpointStore` symbols. Concrete restart, save/load, cancellation, retry, skip, and stress behavior is covered by package tests.

## 검증

```text
go test -count=1 ./batch
ok github.com/bluetape4k/bluetape-go/batch

go test -race -count=1 ./batch
ok github.com/bluetape4k/bluetape-go/batch

golangci-lint run ./batch
0 issues.

make ci
exit code 0

git diff --check
exit code 0

code-review-graph build --repo .
Full build: 194 files, 1234 nodes, 10716 edges

code-review-graph detect-changes --base origin/develop --brief
0 affected flow(s), overall risk score 0.65
```

## 판정

PASS. `P0=0 P1=0`.
