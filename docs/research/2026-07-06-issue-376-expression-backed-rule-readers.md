# Issue #376 Expression-Backed Rule Reader Research

Date: 2026-07-06

## Decision

Adopt a narrow, optional `expr-lang/expr` backed reader in a follow-up issue, but
only for expression-backed predicates. Do not add expression-backed actions in
the first implementation slice.

The reader should compile YAML/JSON definitions into the existing first-party
`rules.Rule` contract. It must not introduce a second execution engine, a
JVM-style annotation model, arbitrary Go code execution, HOCON support, or
unbounded forward chaining.

## Repo-Local Constraints

- `rules.Rule` already owns `Evaluate(context.Context, *Facts)` and
  `Execute(context.Context, *Facts) error`.
- `RuleSet`, composite groups, and `InferenceEngine` already define
  deterministic ordering, fail-closed composite drift behavior, max-cycle
  bounded inference, and context-aware execution.
- Future readers should compile to these contracts rather than bypassing them.
- Inference mutates `Facts` in place and is not transactional; reader docs must
  keep that visible when readers are used with inference.

## Candidate Matrix

| Candidate | Maintenance | License | Safety / sandbox model | Extensibility | Testability | Go API ergonomics | Decision |
|---|---|---|---|---|---|---|---|
| `github.com/expr-lang/expr` | Active; GitHub release `v1.17.8` published 2026-02-14, pushed 2026-06-04; `go list` shows `v1.17.8`. | MIT | Go-centric expression language; README describes memory-safe, side-effect-free, always-terminating expressions. API supports static type checks with `Env`, result type assertions such as `AsBool`, builtin disabling, custom functions, `MaxNodes`, and `WithContext` for context-aware function calls. | Good fit for a constrained predicate adapter: compile once, run many; custom functions can be whitelisted. | Strong: compile errors and runtime errors are ordinary Go errors; compiled programs can be unit-tested against `Facts` snapshots. | Best fit: small API, typed compile step, no standalone rule-engine lifecycle. | Prefer for follow-up implementation. |
| `github.com/PaesslerAG/gval` | Active enough; release `v1.2.3` in GitHub releases, `go list` also reports `v1.2.4`; pushed 2025-08-04. | BSD-3-Clause | Provides `EvaluateWithContext`, `NewEvaluableWithContext`, and `Evaluable(context.Context, ...)`; grammar is composable from language fragments. Less explicit static type checking than `expr`. | Very flexible language composition and custom selectors/functions. | Good for expression-unit tests, but more behavior is runtime/type-conversion driven. | Smaller and composable, but less directly aligned with typed rule predicates. | Keep as fallback/reference, not first choice. |
| `github.com/Knetic/govaluate` | Archived; README points to `ARCHIVED.md`; latest release `v3.0.0` from 2017. | MIT | Older arbitrary expression evaluator. Maintainer explicitly says the repo is preserved and will not receive updates; recommends active alternatives such as `expr`. | Legacy surface only. | Existing users can still test it, but no maintenance path. | Familiar API, but archived dependency is not acceptable for new bluetape-go surface. | Reject. |
| `github.com/hyperjumptech/grule-rule-engine` | Active; release `v1.20.4` in 2025, pushed 2026-02-10. | GitHub reports `NOASSERTION`; README badge says Apache 2.0. | Full rule engine with GRL DSL inspired by Drools. Too broad for a reader that should compile into `rules.Rule`. | High, but it owns rule syntax, knowledge base, and inference model. | Would test a separate engine rather than bluetape-go contracts. | Heavy lifecycle and DSL compared with first-party rules. | Reference only. |
| `github.com/rulego/rulego` | Active; release `v0.36.0` in 2026, pushed 2026-06-26. | Apache-2.0 | Component orchestration/rule-chain framework with many built-in protocol/action components and dynamic loading/plugin concepts. | Very high for orchestration systems. | Tests would exercise RuleGo chains, not `rules.Rule`. | Too broad and action-oriented for safe config readers. | Reference only. |
| `github.com/gorules/zen-go` | Active; pushed 2026-03-15, no GitHub release object; `go list` shows `v0.20.0`. | MIT | Native binding to Zen Engine / JSON Decision Model. Requires engine lifecycle and `Dispose`. | Strong for full BRMS/JDM models. | Testable, but brings native binding lifecycle and JDM model. | Good for BRMS users, not for first-party rules predicate readers. | Reference only. |
| `github.com/bytedance/arishem` | Active-ish; release `v1.1.0` in 2025, pushed 2025-03-24. | Apache-2.0 | JSON-compatible DSL rule engine with global initialization and custom execution model. README/docs are primarily Chinese. | Built for visual/configured rules. | Testable, but separate engine semantics. | Less ergonomic for narrow Go library integration. | Reference only. |

## Proposed Reader Scope

Create an optional package in a follow-up issue, for example
`rules/exprreader` or `rules/readers/exprjson`, with an explicit dependency on
`github.com/expr-lang/expr`.

The first implementation should support:

- YAML and JSON documents with the same schema.
- Predicate expressions only.
- Built-in actions limited to a tiny whitelist that mutates `Facts` through
  predefined operations.
- Compile-on-load and run-many execution.
- `expr.AsBool()` for every predicate.
- `expr.Env` using a narrow map/struct projection from `Facts`.
- `expr.MaxNodes` with a conservative default.
- `expr.DisableAllBuiltins()` plus explicit `EnableBuiltin` or whitelisted
  `Function` registrations only when needed.
- Context propagation through `expr.WithContext("ctx")` for allowed custom
  functions that accept `context.Context`.

Do not support:

- Arbitrary Go callbacks declared in YAML/JSON.
- Unbounded loops, script blocks, reflection registration, annotations, or
  external DSL actions.
- Side-effectful expression predicates.
- HOCON in the first Go reader.

## Minimal YAML/JSON Schema

The schema is justified because `expr` is suitable for predicate evaluation.
Keep it intentionally small:

```yaml
version: 1
rules:
  - name: high-value-discount
    description: Apply a discount for high-value orders.
    priority: 10
    when: amount >= 100 && customer.tier in ["gold", "platinum"]
    then:
      - set:
          key: discount
          value: 10
  - name: fraud-review
    priority: 20
    when: risk_score >= 80
    then:
      - set:
          key: review_required
          value: true
engine:
  stopOnFirstApplied: false
  stopOnFirstFailed: true
  usePriorityThreshold: false
```

Schema rules:

- `version` is required and must be `1`.
- `rules` is required and must be non-empty.
- `name` is required and follows the existing `rules` blank-key validation.
- `priority` defaults to `0`.
- `when` is required and must compile to bool.
- `then` is optional; when present, only whitelisted declarative actions are
  allowed. First slice should support `set` only.
- `engine` is optional and maps only to safe `rules.EngineConfig` fields.
- Inference configuration belongs in a separate top-level `inference` block only
  if a later issue explicitly adds reader-backed inference. It must reject
  `stopOnFirstNotTriggered`, matching `NewInferenceEngine`.

## Error Policy

Add typed reader errors that remain compatible with `errors.Is` /
`errors.As`:

- `ErrInvalidRuleDocument`: malformed YAML/JSON or schema-level validation.
- `ErrInvalidRuleExpression`: expression parse/compile/type-check failure.
- `ErrInvalidRuleAction`: unsupported action or invalid action payload.
- `ReaderError{RuleName, Field, Err}` for rule-local wrapping.

Compilation should fail the whole document if any rule is invalid. Runtime
predicate errors should return through `Rule.Evaluate`; action errors should
return through `Rule.Execute`. Engine behavior should then reuse the existing
`ErrRuleEvaluation` and `ErrRuleExecution` wrapping.

## Context-Cancellation Policy

- Loading/parsing accepts `context.Context` and checks it before each document
  phase: decode, schema validation, expression compile, rule construction.
- Expression evaluation receives the caller's context through the generated
  `Rule.Evaluate`.
- `expr` itself does not make `Run` take a `context.Context`; the adapter must
  check `ctx.Err()` before calling `expr.Run`.
- Any custom whitelisted function that can block or perform I/O must accept
  `context.Context` and be wired through `expr.WithContext("ctx")`.
- The first implementation should avoid I/O custom functions entirely. Reader
  loading should only read from `io.Reader` / `[]byte` supplied by the caller.

## Implementation Issues

Create one follow-up implementation issue:

- `feat: add expr-backed YAML and JSON rules reader`

Acceptance for that issue should include schema tests, compile-time type tests,
runtime predicate/action error tests, context cancellation tests, and
`go test -race -count=1 ./rules ./rules/...`.

## Sources

- `expr-lang/expr`: https://github.com/expr-lang/expr
- Expr docs: https://expr-lang.org/docs/Getting-Started
- `expr` release `v1.17.8`: https://github.com/expr-lang/expr/releases/tag/v1.17.8
- `PaesslerAG/gval`: https://github.com/PaesslerAG/gval
- `Knetic/govaluate` archive note: https://github.com/Knetic/govaluate/blob/master/ARCHIVED.md
- `hyperjumptech/grule-rule-engine`: https://github.com/hyperjumptech/grule-rule-engine
- `rulego/rulego`: https://github.com/rulego/rulego
- `gorules/zen-go`: https://github.com/gorules/zen-go
- `bytedance/arishem`: https://github.com/bytedance/arishem
