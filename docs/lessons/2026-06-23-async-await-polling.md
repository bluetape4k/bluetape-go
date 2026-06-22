# Lessons: Async Await Polling Helpers

## What changed

Issue #211 added context-aware await/polling helpers to the root `testing`
package:

- `CheckAwait` and `RequireAwait`
- `CheckAwaitValue` and `RequireAwaitValue`
- `CheckAwaitError` and `RequireAwaitError`
- `AwaitResult`, `AwaitStatus`, `AwaitProbe`, and `AwaitCheck`

The helpers complement the existing Gomega-backed `Eventually` and
`Consistently` wrappers instead of replacing them.

## What surprised us

The repository lint rule requires `context.Context` to be the first argument
even for `Require*` test helpers. The initial `RequireAwait(tb, ctx, ...)`
shape was more familiar for test assertions, but revive rejected it. The final
API is `RequireAwait(ctx, tb, ...)` and the README examples follow that order.

## What to repeat

- Keep await/polling helpers cooperative and synchronous. A probe that ignores
  its context can still block its own test; helpers should document this rather
  than hide a goroutine leak behind a timeout.
- Do not retry caller-owned `context.Canceled` or `context.DeadlineExceeded`.
- Return the final observed value/error and attempt count so timeout diagnostics
  are useful without requiring logging inside the probe.
- Use `testing/concurrency` for repeated bounded goroutine stress; keep root
  `testing` helpers small and assertion-focused.

## Verification

- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `make fmt-check && make vet && make lint`: PASS
- `make test`: PASS
- `make race`: PASS
