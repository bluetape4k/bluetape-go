# Lock Conformance Test

## Contract

`Run` validates immediate acquire/release, expiry, cancellation, ownership,
lost responses, and exact contention through a provider-neutral function adapter.

## Usage

`Harness.New` returns an owner-bound `AcquireFunc`. `Control` must place gates at
the real mutation boundary and inject failures only after one mutation linearizes.

## Commit-Unknown Recovery

Acquire may return a non-nil release callback together with a typed provider
error. Clean it up immediately. A lost release returns false plus the typed
error; retry the same callback. Owner comparison protects replacements and TTL
is the final fallback.

## Test

```bash
go test -race -count=1 ./lock/locktest
```
