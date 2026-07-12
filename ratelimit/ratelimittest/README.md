# Rate Limit Conformance Test

## Contract

`Run` validates burst, refill, isolation, cancellation, one-debit lost responses,
and exact concurrent admission with a parent-independent neutral `Result`.

## Usage

Convert provider results field by field. Gates and failure injection must be
attached to the actual debit boundary; public-call wrapper gates are insufficient.

## Commit-Unknown Recovery

An indeterminate provider returns a zero `Result` and a typed error. The request
may have debited once, so never replay automatically. Wait a conservative full
refill interval (`Burst / RatePerSecond`) or absorb one debit in the caller budget.

## Test

```bash
go test -race -count=1 ./ratelimit/ratelimittest
```
