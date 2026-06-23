# Issue #60 AWS Helper Boundary Plan

Issue: [#60](https://github.com/bluetape4k/bluetape-go/issues/60)  
Date: 2026-06-23

## Tasks

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
