# Issue #37 Rule Engine Primitives Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 목표

Convert the `bluetape4k-rule-engine` source inventory into Go-native work that
can start in milestone `0.12.0` without adopting a broad third-party rule
engine.

## Package Direction

Create a first-party `rules` package for the small, deterministic core:

```go
type Facts struct { /* protected key/value store */ }

type Rule interface {
    Name() string
    Description() string
    Priority() int
    Evaluate(context.Context, *Facts) (bool, error)
    Execute(context.Context, *Facts) error
}

type EngineConfig struct {
    SkipOnFirstAppliedRule      bool
    SkipOnFirstFailedRule       bool
    SkipOnFirstNonTriggeredRule bool
    PriorityThreshold           int
}
```

The exact exported API belongs to #375, but the design constraints are fixed:
context-first calls, deterministic ordering, explicit error results, and no
reflection/annotation-shaped registration.

## Implementation Boundaries

- #375 owns `Facts`, `Rule`, `RuleSet`, `EngineConfig`, default sequential
  execution, functional rule helpers, docs, unit tests, race tests, and
  stress/cancellation tests.
- #377 owns activation groups, conditional groups, unit groups, and bounded
  inference on top of the core package.
- #376 owns expression/YAML/JSON reader research and may create a later
  implementation issue if `expr-lang/expr` or another evaluator is justified.

## Required Semantics

- Sort rules by priority ascending, then name, then registration sequence for
  exact ties.
- Reject blank fact keys and blank rule names.
- Preserve cancellation with `errors.Is(err, context.Canceled)` and
  `errors.Is(err, context.DeadlineExceeded)`.
- Report evaluation and execution errors in structured results.
- Do not swallow rule errors by default; continuation must be an explicit
  configured policy.
- Keep `Facts` map operations safe for ordinary concurrent access, but document
  that mutation of stored values remains caller-owned unless values are copied
  by the caller.
- Keep rule execution sequential until separate evidence justifies parallel
  execution.

## Rejected Options

- Adopt `grule` or another full DSL engine as the default package: too broad for
  source core parity and pulls callers into a DSL surface before the Go contract
  is proven.
- Extend `workflow` into a rule engine: it has useful context patterns, but its
  current package purpose is composition of work reports, not mutable facts and
  rule ordering.
- Recreate JVM annotations or script engines: those are not idiomatic Go and
  add unnecessary dependency/security surface for the first pass.

## 검증 Handoff

The implementation issues should verify with:

```bash
go test ./...
go test -race ./rules
```

They must also include stress coverage using `testing/concurrency`:
`GoroutineStressTester` for concurrent facts/registration pressure and
`AsyncJobTester` for cancellation-heavy engine execution.
