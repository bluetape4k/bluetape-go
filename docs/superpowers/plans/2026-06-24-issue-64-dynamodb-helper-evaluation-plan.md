# Issue #64 DynamoDB Helper Evaluation Plan

Issue: [#64](https://github.com/bluetape4k/bluetape-go/issues/64)
Date: 2026-06-24

## Classification

Type: Research / decision record.
Selected lane: docs-only fast-track with 7-tier review evidence.

## Tasks

1. Confirm tracker state.
   - Verify #64 assignee, labels, milestone, and comments.
   - Check duplicate DynamoDB helper issues before creating follow-ups.
2. Gather source evidence.
   - Read #60 AWS helper boundary decision.
   - Inspect current `testcontainers/floci` DynamoDB smoke coverage.
   - Inspect `bluetape4k-aws` DynamoDB batch, mapper, repository, schema, and
     framework integration surfaces.
   - Confirm AWS SDK for Go v2 DynamoDB official docs for direct client,
     expression builder, paginator, and batch write behavior.
3. Decide scope.
   - Classify each candidate as package code, examples/workshop, direct SDK, or
     defer.
   - Create implementation issues only for helpers with clear value beyond the
     AWS SDK.
4. Record artifacts.
   - Add research decision document.
   - Add review and lesson artifacts.
   - Link follow-up issue #270 and workshop issue.
5. Verify docs-only diff.
   - Run `git diff --check`.
   - Run `make fmt-check`.
   - Run `make tidy-check`.
6. Publish PR.
   - Commit with Lore trailers.
   - Push `issue-64-dynamodb-helper-evaluation`.
   - Create PR against `develop`.
   - Assign `debop`; mirror #64 labels and milestone.
   - Verify live PR body final `##` heading is `## DoD Status`.

## Stop Condition

Stop when the decision PR is open with correct metadata, local validation is
recorded, #64 links the PR and follow-up routing, and CI status is known or
explicitly pending.
