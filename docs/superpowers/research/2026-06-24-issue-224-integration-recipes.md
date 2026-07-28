# Issue 224 Integration Recipe Research

> 한국어 연구 요약: 이 문서는 사용자 협업용 조사/결정 기록이다. 아래 표와 목록의 URL, package name, command, issue number, version, source path는 evidence이므로 그대로 보존한다. 의사결정, 선택/보류/거절 사유, 후속 이슈 경계는 한국어 독자가 바로 이해할 수 있도록 이 요약을 우선 적용한다.
> 추가 한국어 해석: 이 문서에서 영어로 남은 표의 값은 원문 근거이며, 실제 채택 여부는 한국어 결정 문장을 따른다. 후속 작업자는 보류와 거절 항목을 새 구현 범위로 착각하지 않아야 한다.\n

Issue: #224
Branch: issue-224-integration-recipes
Date: 2026-06-24

## 범위 Decision

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
