# Leader Conformance Test

## Contract

`Run` applies one mandatory single-elector contract to every provider. The
factory and control are required; there are no capability skips.

## Usage

Use a caller-owned backend fixture, normalize one `leader.Options` identity,
construct provider electors in `Harness.New`, and expose only deterministic
owner probes, operation counts, replacement, and post-linearization failure
injection through `Control`.

```go
func TestProviderConformance(t *testing.T) {
    leadertest.Run(t, providerHarness(t))
}
```

## Timeout Containment

Use `RunWithConfig` when a provider needs a hard-stop callback. `CaseTimeout`
starts cancellation and containment; it does not cap the final join. `Abort`
must unblock provider work, and the test command must set an outer
`go test -timeout` so a provider that ignores cancellation and cannot be joined
fail-stops the process. See `ExampleRunWithConfig` for the compile-checked shape.

## Commit-Unknown Recovery

A provider must return a typed `leader.OperationError` for dispatched failures.
When commit cannot be determined it also matches `leader.ErrCommitUnknown`.
Callers use type-first handling, bounded `Resign`, and lease TTL fallback before
starting another campaign.

## Diagnostics

Runner failures report the stable case name and a safe reason category such as
`context`, `state`, `provider`, or `contract`. They never render adapter errors,
owner values, keys, endpoints, or tokens. Inspect provider-local logs separately.

## Test

```bash
go test -race -count=1 -timeout=10m ./leader/leadertest
```
