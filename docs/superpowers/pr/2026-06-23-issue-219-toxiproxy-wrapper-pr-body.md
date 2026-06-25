Part of #219

## Summary

- Added the #219 messaging/HTTP/fault-injection prioritization matrix and
  selected Toxiproxy as the first high-value wrapper slice.
- Added `testcontainers/toxiproxy` with `Start`, `StartServer`,
  `StartContainer`, `ProxiedEndpoint`, and the documented
  `toxiproxy.control_uri` connection detail key.
- Added Redis-through-Toxiproxy integration coverage proving usable failure
  injection without latency sleeps.
- Added English and Korean README pages with control URI export, dynamic-port
  guidance, bounded timeout notes, serial Docker test guidance, and explicit
  deferrals.

## Scope Decisions

- Toxiproxy is in scope because it directly supports resilience/failure-mode
  testing without selecting a messaging adapter.
- RabbitMQ, Redpanda, and Pulsar remain deferred to #58 until outbox adapter
  semantics select a broker.
- WireMock, Nginx, and Mailpit remain deferred to #224 or a concrete recipe
  issue that proves `httptest` is insufficient.
- ElasticMQ/SQS/SNS-compatible fixtures remain coordinated with #220 and
  #61-#64.

## Validation

- `go test -p 1 -count=1 ./testcontainers/toxiproxy`
- `go test -race -p 1 -count=1 ./testcontainers/toxiproxy`
- `go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy`
- `go test -race -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy`
- `rg -n "toxiproxy.control_uri|BLUETAPE_TOXIPROXY_CONTROL_URI|dynamic|bounded|#58|#224|#220" testcontainers/toxiproxy/README.md testcontainers/toxiproxy/README.ko.md`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `make ci`
- `git diff --check`

## Review Evidence

- Step 2-R spec review:
  `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-2r-spec-review.md`
  with `P0=0 P1=0`.
- Step 3-R plan review:
  `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-3r-plan-review.md`
  with `P0=0 P1=0`.
- Step 6-R code review:
  `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-6r-code-review.md`
  with `P0=0 P1=0`.
- Main integration fallback was used for the 7-Tier review lanes per session
  instruction; the six perspectives are recorded in the tracked review
  artifacts.

## DoD Status

| Step | Status | Evidence |
|---|---|---|
| Step 1-R issue classification | PASS | #219 classified as Type A full-feature because it adds a new Testcontainers package and dependency slice. |
| Step 2 spec | PASS | `docs/superpowers/specs/2026-06-23-issue-219-toxiproxy-wrapper-design.md`. |
| Step 2-R spec review | PASS | `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-2r-spec-review.md`, `P0=0 P1=0`. |
| Step 3 plan | PASS | `docs/superpowers/plans/2026-06-23-issue-219-toxiproxy-wrapper-plan.md`. |
| Step 3-R plan review | PASS | `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-3r-plan-review.md`, `P0=0 P1=0`. |
| Step 4 TDD | PASS | Initial package test failed before implementation because Toxiproxy dependencies/package were absent; implementation then made targeted tests pass. |
| Step 5 implementation | PASS | `testcontainers/toxiproxy`, README pair, and `go.mod`/`go.sum` updates. |
| Step 6 validation | PASS | Targeted tests, race tests, README grep, `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, `make ci`, and `git diff --check`. |
| Step 6-R code review | PASS | `docs/superpowers/reviews/2026-06-23-issue-219-toxiproxy-wrapper-step-6r-code-review.md`, `P0=0 P1=0`. |
| Step 7 PR readiness | PASS | Assignee/milestone/labels copied from #219; this PR body ends with `## DoD Status`. |
