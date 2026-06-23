# Issue #62 S3 Examples Plan

Issue: [#62](https://github.com/bluetape4k/bluetape-go/issues/62)  
Date: 2026-06-24

## Tasks

1. Create stacked worktree from `issue-60-aws-helper-boundaries`.
2. Inspect current Floci fixture, #60 boundary docs, and AWS SDK for Go v2 S3
   documentation.
3. Add `examples/s3` with compile-checked S3 examples and opt-in Floci smoke.
4. Update README pairs and root package index.
5. Run validation:
   - `go test -count=1 ./examples/s3`
   - `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3`
   - `go test -race -count=1 ./examples/s3`
   - `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/s3`
   - `make fmt-check`
   - `make tidy-check`
   - `make vet`
   - `make lint`
   - `git diff --check`
6. Run Step 6-R 7-tier review with main integration fallback if subagents are
   unavailable or unstable.
7. Commit, push, and create a stacked PR against `issue-60-aws-helper-boundaries`
   with #62 assignee, milestone, and labels.
8. Run Step 7-R review and CI gate. Do not merge.

## Stop Condition

Stop when the stacked PR is open, metadata mirrors #62, local validation and CI
are recorded, P0/P1 review findings are zero, and the PR remains unmerged.
