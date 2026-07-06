# rules

[English](README.md) | [한국어](README.ko.md)

`rules`는 일반 Go application을 위한 dependency-free deterministic rule-engine
core를 제공합니다. Scope는 의도적으로 작습니다. Facts, rule contract, rule set,
engine config, sequential execution, result detail, typed error, context
cancellation만 다룹니다.

Expression language, YAML/JSON reader, annotation-style registration, composite
rule group, forward chaining은 포함하지 않습니다. 이 기능들은 이 core 위에 쌓는
후속 범위입니다.

Rule은 trusted Go code입니다. Context cancellation은 cooperative합니다. Engine은
각 evaluation과 execution 전에 context를 확인하며, rule body도 전달받은
context를 존중해야 합니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/rules"
```

## 예제

Compile-checked 예제는 [`rules_example_test.go`](rules_example_test.go)에
있습니다.

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

`Facts`는 blank-key validation, deletion, sorted key listing, clone helper,
shallow snapshot을 제공하는 key/value container입니다.

- Container operation은 concurrent access에 안전합니다.
- 저장된 value는 caller-owned입니다. `Clone`과 `Snapshot`은 map만 복사하고 내부
  value는 deep copy하지 않습니다.
- Blank key는 `ErrBlankKey`로 거부합니다.

## Rules

Rule은 plain Go interface입니다.

```go
type Rule interface {
    Name() string
    Description() string
    Priority() int
    Evaluate(context.Context, *Facts) (bool, error)
    Execute(context.Context, *Facts) error
}
```

간단한 functional rule은 `NewRule`을 사용하세요. Reflection, annotation,
JVM-shaped DSL을 사용하지 않습니다.

## RuleSet Ordering

`RuleSet`은 duplicate rule name을 거부하고 deterministic order로 rule을 반환합니다.

1. Priority ascending.
2. Rule name ascending.
3. 완전히 같은 tie에서는 registration sequence.

## Engine

`Engine`은 rule을 sequential하게 실행하고, evaluated/applied/skipped/failed rule
마다 `Detail`을 담은 `Result`를 반환합니다.

`EngineConfig`는 다음 정책을 제공합니다.

- `StopOnFirstApplied`
- `StopOnFirstFailed`
- `StopOnFirstNotTriggered`
- `UsePriorityThreshold`와 `PriorityThreshold`

Engine은 각 evaluation 전과 execution 전에 `context.Context`를 확인합니다.
Cancellation error는 `errors.Is`로 `context.Canceled` 또는
`context.DeadlineExceeded`를 보존합니다.

Evaluation/execution failure는 result detail에 기록됩니다. `StopOnFirstFailed`가
true이면 run을 중단하고 `ErrRuleEvaluation` 또는 `ErrRuleExecution`과 호환되는
`RuleError`를 반환합니다. `StopOnFirstFailed`가 false이면 engine은 이후 rule을
계속 실행하지만 rule failure가 하나라도 있으면 joined error를 반환합니다. Rule별
failure는 `Result.Details`에서 확인하세요.

## 테스트

```bash
go test -count=1 ./rules
go test -race -count=1 ./rules
```
