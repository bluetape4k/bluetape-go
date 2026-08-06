# Issue #62 S3 Examples Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

Issue: [#62](https://github.com/bluetape4k/bluetape-go/issues/62)  
Date: 2026-06-24

## 작업

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
