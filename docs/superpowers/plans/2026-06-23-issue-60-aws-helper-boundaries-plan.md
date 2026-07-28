# Issue #60 AWS Helper Boundary Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

Issue: [#60](https://github.com/bluetape4k/bluetape-go/issues/60)  
Date: 2026-06-23

## 작업

1. Confirm repository and GitHub state.
   - Verify #60 assignee, labels, and milestone.
   - Verify the branch is stacked on `issue-61-floci-service-smoke`.
2. Gather boundary evidence.
   - Read 0.9.0 AWS research.
   - Read #220 and #61 Floci fixture decisions.
   - Inspect `bluetape4k-aws` service coverage and emulator policy.
3. Write decision artifacts.
   - Add the #60 candidate matrix and follow-up routing.
   - Add spec and plan artifacts for review traceability.
4. Run 7-tier review using main integration fallback.
   - Record Step 2-R, Step 3-R, and Step 6-R with P0/P1 verdicts.
5. Verify docs-only diff.
   - Run `git diff --check`.
   - Run `make fmt-check`.
   - Run `make tidy-check`.
   - Run `go test -p 1 -count=1 ./testcontainers/floci`.
6. Publish stacked PR.
   - Commit with Lore trailers.
   - Push `issue-60-aws-helper-boundaries`.
   - Create PR against `issue-61-floci-service-smoke`.
   - Assign `debop`; apply milestone `0.9.0`; mirror #60 labels.
   - Verify the live PR body final `##` heading is `## DoD Status`.
   - Do not merge.

## Stop Condition

Stop when the stacked PR is open, metadata is correct, local verification
evidence is recorded, CI status is known or explicitly pending, and #60 has a
comment linking the PR and routing decision.
