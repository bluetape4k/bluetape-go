# Step 6-R Review: Issue 224 Integration Recipes

Issue: #224
Branch: issue-224-integration-recipes
Date: 2026-06-24

Subagent lanes were not used due to current subagent/runtime instability; main
integration fallback performed with required lane separation.

## Performance Lane

P0: 0
P1: 0

- Service-free recipes run in-process and avoid sleeps.
- Redis smoke is env-gated and uses a single Testcontainers Redis instance.
- Retry examples use `resilience.NoBackoff()` and bounded `MaxAttempts`.

## Stability Lane

P0: 0
P1: 0

- Every runnable recipe uses a timeout context.
- Redis lock, leadership, client, and container cleanup are registered with
  `t.Cleanup`.
- Batch failure paths cover temporary write retry and invalid-item skip.

## Security Lane

P0: 0
P1: 0

- JWT examples use explicit HS256 and a 32-byte HMAC secret.
- Parse examples require expected subject, audience, and expiration where the
  recipe signs an access token.
- Redis lock cleanup only unlocks with the owner token.

## Operator/Ops Lane

P0: 0
P1: 0

- Docker-backed Redis smoke is opt-in with
  `BLUETAPE_INTEGRATION_RECIPE_SMOKE=1`.
- README documents serial execution for Testcontainers-backed packages.
- Package docs describe cleanup, timeouts, and failure-path behavior.

## Developer/API Lane

P0: 0
P1: 0

- Recipes use existing public APIs only.
- No helper abstraction was promoted from example code into a library package.
- Root README and README.ko.md link the new example package.

## User/Caller Lane

P0: 0
P1: 0

- The package gives callers copyable commands for service-free tests, smoke
  tests, and race tests.
- README links point to maintained package-level docs.

## Main Integration Verdict

P0: 0
P1: 0

The change is scoped to examples and documentation. It should proceed if the
example package, Redis smoke, and standard repository gates pass.

## Validation Evidence

- PASS `go test -count=1 ./examples/integration`
- PASS `BLUETAPE_INTEGRATION_RECIPE_SMOKE=1 go test -p 1 -count=1 ./examples/integration`
- PASS `go test -race -count=1 ./examples/integration`
- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `make test`
- PASS `make race`
