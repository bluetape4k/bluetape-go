# Roadmap Reorder and Floci Review

Date: 2026-06-08 KST
Scope: milestone `0.8.0` through `0.12.0` ordering, related issue metadata,
README/WIP/research documentation, and AWS emulator direction.

## Decision Reviewed

The post-`0.7.0` order should be:

1. `0.8.0` relational SQL DSL and repository helpers.
2. `0.9.0` AWS helper packages and Floci-backed examples.
3. `0.10.0` text search and tokenizer packages.
4. `0.11.0` audit and event packages.
5. `0.12.0` graph packages and examples.

Rationale: SQL is foundational backend infrastructure and should precede graph
work. AWS belongs immediately after SQL because service integration examples
are broadly useful. Graph remains valuable, but backend selection, graph I/O,
and domain examples carry more uncertainty, so it is deferred.

## Floci Position

AWS planning should be Floci-first, not LocalStack-first.

Evidence:

- Floci documents LocalStack-compatible AWS SDK/CLI endpoint conventions:
  port `4566`, test credentials, and endpoint override flow.
- Floci documents Testcontainers modules for Java, Node.js, and Python, while
  the Go module is still in progress.
- For `bluetape-go`, the first fixture should therefore wrap the `floci/floci`
  Docker image with `testcontainers-go` and expose endpoint, region, access key,
  and secret key values directly.

LocalStack remains a compatibility reference, but not the default open-CI
fixture target because its current product path introduces account/auth-token
and plan-fit review.

## GitHub Metadata Checked

- `0.8.0`: #101.
- `0.9.0`: #47, #60, #61, #62, #63, #64.
- `0.10.0`: #45, #52, #53, #54, #55.
- `0.11.0`: #46, #56, #57, #58, #59.
- `0.12.0`: #44, #48, #49, #50, #51.

Milestone descriptions were also checked through the GitHub API and match the
same order.

## Review Findings

- P0: 0.
- P1: 0.
- P2: 0.
- P3: 0.

Verdict: pass. The roadmap, issue metadata, and research docs now describe the
same ordering and AWS emulator direction.

## Validation

- `gh api repos/bluetape4k/bluetape-go/milestones --paginate`.
- `gh issue list --state open --milestone ...` for `0.8.0` through `0.12.0`.
- `rg` stale-reference checks over README, WIP, and `docs/research`.
- `git diff --check`.
