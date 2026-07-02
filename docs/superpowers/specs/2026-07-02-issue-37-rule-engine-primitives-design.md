# Issue #37 Rule Engine Primitives Design

## Goal

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

## Validation Handoff

The implementation issues should verify with:

```bash
go test ./...
go test -race ./rules
```

They must also include stress coverage using `testing/concurrency`:
`GoroutineStressTester` for concurrent facts/registration pressure and
`AsyncJobTester` for cancellation-heavy engine execution.
