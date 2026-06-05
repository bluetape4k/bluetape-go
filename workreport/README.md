# workreport

[English](README.md) | [한국어](README.ko.md)

`workreport` provides status, failure-policy, and report-tree values for
lightweight workflow code. It is independent from runner execution so ordinary
Go functions and future `workflow` runners can share the same result model.

## Example

```go
report, err := workreport.Aggregate(
    "import",
    workreport.ContinueOnFailure,
    workreport.Completed("read"),
    workreport.Failed("write", errors.New("disk full")),
)
if err != nil {
    return err
}

if report.IsPartial() {
    log.Printf("%s finished with %d child reports", report.Name, len(report.Children))
}
```

## Statuses

- `StatusCompleted`: work finished without failed children.
- `StatusFailed`: work failed with a caller-visible error.
- `StatusPartial`: aggregated work has at least one non-completed child.
- `StatusAborted`: work stopped for a policy or caller-defined reason.
- `StatusCancelled`: caller cancellation or deadline stopped the work.

A zero-value `Report` has an unknown status. It is not successful, failed,
partial, aborted, cancelled, or terminal.

## Failure Policies

- `StopOnFailure` keeps child reports through the first non-completed child and
  returns that child status as the parent status.
- `ContinueOnFailure` keeps every child report and returns `StatusPartial` when
  any child is not completed.

Unknown policy values return an `ErrUnknownFailurePolicy`-compatible error from
`Aggregate`.

## Contracts

- Constructors preserve caller-visible errors in `Report.Err`.
- Aggregation preserves child report order.
- Child reports are copied into parent reports so caller slice mutations do not
  rewrite existing report trees.
- `workreport` does not start goroutines, retry work, own cancellation, or run
  workflow branches. Runner behavior belongs to the `workflow` package.
