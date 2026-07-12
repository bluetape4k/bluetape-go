# Leader Conformance Test

## Contract

`Run` applies one mandatory single-elector contract to every provider. The
factory and control are required; there are no capability skips.

## Usage

Use a caller-owned backend fixture, normalize one `leader.Options` identity,
construct provider electors in `Harness.New`, and expose only deterministic
owner probes, operation counts, replacement, and post-linearization failure
injection through `Control`.

## Commit-Unknown Recovery

A provider must return a typed `leader.OperationError` for dispatched failures.
When commit cannot be determined it also matches `leader.ErrCommitUnknown`.
Callers use type-first handling, bounded `Resign`, and lease TTL fallback before
starting another campaign.

## Diagnostics

Runner failures report only the stable case name and never render adapter errors,
owner values, keys, endpoints, or tokens. Inspect provider-local logs separately
when a case fails.

## Test

```bash
go test -race -count=1 ./leader/leadertest
```
