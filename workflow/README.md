# workflow

[English](README.md) | [한국어](README.ko.md)

`workflow` provides lightweight runners for ordinary Go work functions. Each
work item receives a `context.Context` and returns a `workreport.Report`.

The package is intentionally small: it has sequential, conditional, and
all-branches parallel runners. It does not provide a durable workflow engine,
retry scheduler, Kotlin-style DSL, or mutable shared `WorkContext` map.

## Diagram

![workflow runner flow](../docs/images/readme-diagrams/workflow-runner-flow.png)

## Example

The snippet assumes standard imports such as `context` and `log/slog`. The
compile-checked version lives in [`workflow_example_test.go`](workflow_example_test.go).

```go
runner := workflow.Sequential(
    "import",
    workreport.ContinueOnFailure,
    func(context.Context) workreport.Report { return workreport.Completed("read") },
    func(context.Context) workreport.Report { return workreport.Failed("write", err) },
)

report := runner.Run(ctx)
if report.IsPartial() {
    slog.InfoContext(ctx, "workflow produced partial report",
        slog.String("report", report.Name),
        slog.Int("children", len(report.Children)),
    )
}
```

## Runnable Examples

Compile-checked examples for sequential, conditional, and parallel runners live
in [`workflow_example_test.go`](workflow_example_test.go). Run them with:

```bash
go test ./workflow
```

## Runners

- `Sequential` runs work in input order. `StopOnFailure` stops at the first
  failed or partial child, while `ContinueOnFailure` keeps going after ordinary
  failures. Aborted and cancelled child reports always stop the sequence.
- `Conditional` evaluates one predicate and runs exactly one selected branch.
  A false predicate with no false branch returns a completed report.
- `Parallel` starts every work item with a shared cancellable context and
  preserves child reports in input order. `StopOnFailure`, aborted, and
  cancelled child reports cancel siblings and wait for started goroutines.

## External execution bridge

[`workflow/stepfunctions`](stepfunctions/README.md) is intentionally separate
from these in-process runners. It provides a caller-owned AWS Step Functions
execution bridge for start, describe, optional stop, and bounded wait/polling.
It does not define, deploy, retry, or own a durable workflow engine. See the
package README for payload limits, idempotency, cancellation, and fake-first
verification contracts.

## Contracts

- Nil caller contexts are treated as `context.Background()`.
- Nil work, nil predicates, too many false branches, unknown report statuses,
  and unknown failure policies return failed reports with checkable errors.
- Predicate cancellation and caller cancellation return cancelled reports with
  the caller context error.
- Use ordinary closures and explicit inputs for shared data. The package does
  not own a mutable workflow context.
