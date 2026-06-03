# Issue 20 HTTP resilience 7-tier lite review

## Scope

- Issue: #20 `Add HTTP resilience middleware and examples`
- Branch: `feat/issue-20-http-resilience-examples`
- Code: `resilience/http.go`, `resilience/http_test.go`,
  `resilience/http_example_test.go`
- Docs: `README.md`, `README.ko.md`, `resilience/doc.go`

## Findings

P0/P1: 0.

| Tier | Scope | Verdict | Evidence |
|---|---|---|---|
| 1 Security | request replay, status handling | PASS | Non-replayable request bodies fail before a second retry attempt; server retry caveat is documented. |
| 2 Architecture | API boundary | PASS | Adapter stays in `resilience` and uses only `net/http`; no framework or telemetry dependency added. |
| 3 Reliability | retry cleanup, timeout context | PASS | Retryable response bodies are closed before retry; handler receives policy child context. |
| 4 Correctness | policy composition | PASS | Client adapter composes retry, timeout, and circuit breaker through `Policy[*http.Response]`; server adapter composes `Policy[struct{}]`. |
| 5 Tests | behavior coverage | PASS | Tests cover retryable status close, replayable and non-replayable bodies, server error mapping, and handler timeout. |
| 6 Performance | hot path | PASS | Adapter allocates only per-attempt request clones and does not add background workers. |
| 7 Docs/Release | README/API docs | PASS | README locale pair and package doc describe HTTP adapter usage and retry caveats. |

## Validation Evidence

- `go test -count=1 ./resilience`: PASS
- `go vet ./...`: PASS
- `go test -count=1 ./...`: PASS
- `golangci-lint config verify`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `git diff --check`: PASS
- `actionlint`: PASS
- `make ci`: PASS, including lint, full tests, and race tests.

## Residual Risk

Server handlers cannot reliably undo a response that was already written before
a later policy error. The README explicitly warns against retrying server
handlers after response writes and recommends retry on outbound client calls.
