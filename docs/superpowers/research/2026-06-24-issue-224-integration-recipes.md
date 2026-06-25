# Issue 224 Integration Recipe Research

Issue: #224
Branch: issue-224-integration-recipes
Date: 2026-06-24

## Scope Decision

#224 asks for runnable recipes proving corrected `0.6.x` packages work together.
The smallest durable shape is a new `examples/integration` package, not a new
library API. Existing `examples/s3` and `examples/sqs-sns` use compile-checked
examples plus env-gated Docker smoke tests, so this work follows that pattern.

## Local Evidence

- Existing example packages provide `example_test.go`, `doc.go`, `README.md`,
  `README.ko.md`, and explicit smoke env vars.
- `batch` already supports checkpointed `Step` execution with retry and skip
  policies.
- `workflow` already provides sequential orchestration over `workreport`.
- `cache.Memory` supports context-aware `GetOrLoad` with same-key stampede
  protection.
- `resilience` composes retry and timeout policies around context-aware
  operations.
- `leader/redis`, `lock/redis`, and `testcontainers/redis` already provide the
  Redis contracts needed for a Docker-backed smoke recipe.
- `id` and `jwt` have public examples for UUID v7 generation and fixed HMAC JWT
  compose/parse.

## Routing

Classified as Type E maintenance because the issue title starts with `docs:`,
with Type B-style Go validation because the documentation includes runnable Go
examples and a Testcontainers smoke test.

## Constraints

- Default `go test ./...` must not require Docker.
- Redis/Testcontainers coverage must be opt-in through an explicit env var.
- Root English and Korean READMEs must link the new package.
- Cross-package recipes must remain examples and avoid adding new first-party
  abstractions.
