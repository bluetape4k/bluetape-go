# rules

[English](README.md) | [한국어](README.ko.md)

`rules` provides a dependency-free, deterministic rule-engine core for ordinary
Go applications. It is intentionally small: facts, rule contracts, rule sets,
engine configuration, sequential execution, result details, typed errors, and
context cancellation.

The package does not include expression languages, YAML/JSON readers,
annotation-style registration, composite rule groups, or forward chaining.
Those surfaces are separate follow-up concerns built on this core.

Rules are trusted Go code. Context cancellation is cooperative: the engine
checks the context before each evaluation and execution, and rule bodies should
also respect the context they receive.

## Import

```go
import "github.com/bluetape4k/bluetape-go/rules"
```

## Example

Compile-checked examples live in [`rules_example_test.go`](rules_example_test.go).

```go
import (
    "context"
    "fmt"

    "github.com/bluetape4k/bluetape-go/rules"
)

func applyDiscount(ctx context.Context) error {
    facts := rules.NewFacts()
    _ = facts.Set("amount", 120)

    discount, err := rules.NewRule(
        "discount",
        func(_ context.Context, facts *rules.Facts) (bool, error) {
            value, ok := facts.Get("amount")
            if !ok {
                return false, nil
            }
            amount, ok := value.(int)
            return ok && amount >= 100, nil
        },
        func(_ context.Context, facts *rules.Facts) error {
            return facts.Set("discount", 10)
        },
        rules.WithPriority(10),
        rules.WithDescription("apply a threshold discount"),
    )
    if err != nil {
        return err
    }

    set, err := rules.NewRuleSet(discount)
    if err != nil {
        return err
    }
    result, err := rules.NewEngine(set, rules.EngineConfig{
        StopOnFirstFailed: true,
    }).Run(ctx, facts)
    if err != nil {
        return err
    }
    if result.Failed > 0 {
        return fmt.Errorf("rules failed: %d", result.Failed)
    }
    return nil
}
```

## Facts

`Facts` is a key/value container with blank-key validation, deletion, sorted
key listing, clone helpers, and shallow snapshots.

- Container operations are safe for concurrent access.
- Stored values are caller-owned. `Clone` and `Snapshot` copy the map, not the
  values inside it.
- Blank keys are rejected with `ErrBlankKey`.

## Rules

Rules are plain Go interfaces:

```go
type Rule interface {
    Name() string
    Description() string
    Priority() int
    Evaluate(context.Context, *Facts) (bool, error)
    Execute(context.Context, *Facts) error
}
```

Use `NewRule` for simple functional rules. It avoids reflection, annotations,
and JVM-shaped DSLs.

## RuleSet Ordering

`RuleSet` rejects duplicate rule names and returns rules in deterministic order:

1. Priority ascending.
2. Rule name ascending.
3. Registration sequence for exact ties.

## Engine

`Engine` runs rules sequentially and returns a `Result` with one `Detail` per
evaluated, applied, skipped, or failed rule.

`EngineConfig` supports:

- `StopOnFirstApplied`
- `StopOnFirstFailed`
- `StopOnFirstNotTriggered`
- `UsePriorityThreshold` with `PriorityThreshold`

The engine checks `context.Context` before each evaluation and before each
execution. Cancellation returns an error that preserves `context.Canceled` or
`context.DeadlineExceeded` for `errors.Is`.

Evaluation and execution failures are recorded in result details. When
`StopOnFirstFailed` is true, the run stops and returns a `RuleError` compatible
with `ErrRuleEvaluation` or `ErrRuleExecution`. When `StopOnFirstFailed` is
false, the engine continues through later rules but still returns a joined
error when any rule fails; inspect `Result.Details` for per-rule failures.

## Test

```bash
go test -count=1 ./rules
go test -race -count=1 ./rules
```
