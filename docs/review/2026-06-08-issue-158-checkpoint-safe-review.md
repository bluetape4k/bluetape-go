# Issue #158 7-Tier Review: Checkpoint-safe writer skip

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- Issue: #158 `fix(batch): make skipped writer chunks checkpoint-safe`
- Branch: `fix-issue-158-checkpoint-safe`
- Base: `origin/develop`
- Changed files:
  - `batch/errors.go`
  - `batch/policy.go`
  - `batch/step.go`
  - `batch/policy_test.go`

## 요약

`Step.flush` no longer advances a checkpoint after a skipped writer chunk when
`CheckpointStore` is configured. `Writer` does not expose an atomic commit
boundary for a failed chunk, so checkpointing after a skipped writer error could
drop items that were not safely committed. The new behavior fails that unsafe
path with `ErrUnsafeWriterSkipCheckpoint` and preserves the original writer
error with `%w`. Writer skip behavior without checkpointing is unchanged.

## 7-Tier 발견 사항

### Tier 1: Security

- Finding: none.
- Evidence: No auth, secret, network boundary, serialization, or input trust
  boundary changed.
- Gate: P0=0, P1=0.

### Tier 2: Ops and SRE Reliability

- Finding: none.
- Evidence: Checkpointed writer-skip failure now keeps the last durable
  checkpoint instead of advancing reader progress beyond a partially failed
  writer chunk.
- Operational impact: restart replays the failed chunk from the last safe
  checkpoint.

### Tier 3: Structural Impact

- Finding: none.
- Evidence: Scope is limited to `batch.Step.flush`, policy documentation, a
  package-level sentinel error, and regression tests.
- Public API impact: `ErrUnsafeWriterSkipCheckpoint` is added as an exported
  sentinel error. Existing no-checkpoint writer skip behavior remains intact.

### Tier 4: Go Code Quality

- Finding: none.
- Evidence: The implementation uses an idiomatic sentinel error and `%w`
  wrapping. No new dependencies, goroutines, timers, global mutable state, or
  context detachment were added.
- Production scan: `rg` found only intentional test `context.Background()` use
  and the existing `normalizeContext` fallback in `batch/step.go`.

### Tier 5: Tests, Types, and Silent Failure

- Finding: none.
- Evidence: `TestStepRunDoesNotCheckpointSkippedWriterChunk` covers a writer
  that commits part of a chunk before returning an error. It verifies the step
  fails with `ErrUnsafeWriterSkipCheckpoint`, preserves the writer cause,
  leaves `SkipCount` at zero, keeps checkpoint `2`, and replays items `3,4` on
  restart before saving checkpoint `4`.

### Tier 6: Performance and Stability

- Finding: none.
- Evidence: The new error allocation occurs only on the failure path after
  retry exhaustion and skip matching. The success path and normal checkpoint
  path are unchanged. Race tests passed for `./batch`.

### Tier 7: Docs, Release, and Evidence

- Finding: none.
- Evidence: `SkipPolicy` documentation now states the checkpointed writer-skip
  restriction. This review artifact records the gate result and validation
  commands for PR review.
- Release note: This is a patch-safe behavior fix for milestone `0.5.0`; it
  does not require retagging by itself.

## 검증

- PASS: `go test -count=1 ./batch -run 'TestStepRunDoesNotCheckpointSkippedWriterChunk|TestStepRunComposesRetryAndSkipPolicies|TestStepRunRestartsFromCheckpoint'`
- PASS: `go test -count=1 ./batch`
- PASS: `go test -race -count=1 ./batch`
- PASS: `golangci-lint run ./batch`
- PASS: `git diff --check`
- TRANSIENT FAIL: first `make ci` failed in `TestStartKafka` with
  `find kafka controller: EOF`.
- PASS: `go test -count=1 ./testcontainers/kafka -run TestStartKafka -v`
- PASS: rerun `make ci`

## 그래프 검토

- `code-review-graph.detect_changes_tool` analyzed the four changed files
  against `origin/develop` and returned risk score `0.00`, affected flows `0`,
  and test gaps `0`.
- Limitation: the graph reported zero changed functions for this Go diff, so
  source-level review and focused tests are the primary evidence.

## 게이트 판정

P0=0 P1=0

Gate passes for PR creation.
