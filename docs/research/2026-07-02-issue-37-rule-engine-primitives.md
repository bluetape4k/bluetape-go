# Issue #37 Rule Engine Primitives Research

## Context

Issue #37 asks how much of `bluetape4k-rule-engine` should become Go code and
which parts should remain dependency-backed or deferred. The source module
contains core facts/rules/rule sets, priority-based execution, Kotlin DSL and
annotation registration, coroutine-aware execution, composite groups, forward
chaining, script-backed rules, and YAML/JSON/HOCON readers.

The Go repository has no current rule-engine package. The closest local
reference is `workflow`, which already models `context.Context` cancellation
and conditional execution, but it intentionally avoids mutable shared state and
durable rule-engine semantics.

## Source Concept Matrix

| Source concept | Go decision | Rationale |
|---|---|---|
| `Facts` | Implement first-party | Small map-like surface; blank-key validation, clone/snapshot helpers, and deterministic key listing fit Go without dependencies. |
| `Rule` | Implement first-party | A context-aware interface is clearer than Kotlin annotations or reflection. |
| `RuleSet` | Implement first-party | Deterministic priority/name ordering is core behavior and easy to test locally. |
| Engine parameters | Implement first-party | Skip-on-first applied/failed/non-triggered and priority threshold map directly to an engine config. |
| Coroutine/suspend support | Translate to `context.Context` | Go cancellation should be explicit in `Evaluate` and `Execute`, not hidden behind async wrappers. |
| Composite rules | Split to follow-up | Activation, conditional, and unit groups are useful but should build on a proven core package. |
| Forward chaining | Split to follow-up | The JVM inference engine can loop until no rules match; Go should add a max-cycle guard and typed non-convergence error. |
| Kotlin DSL | Replace with functional options/builders | Go callers should use constructors and functional rule helpers rather than a JVM-shaped DSL. |
| Annotation rules | Defer | Reflection-based annotation parity is not idiomatic Go. |
| Script engines | Defer | MVEL, SpEL, Kotlin Script, Janino, and Groovy are JVM-specific and would broaden the dependency/security surface. |
| YAML/JSON/HOCON readers | Split to research | YAML/JSON may be useful with a safe expression engine; HOCON is not a first-pass Go need. |

## Dependency Candidates

| Candidate | License | Maintenance signal | Fit | Decision |
|---|---|---|---|---|
| `hyperjumptech/grule-rule-engine` | README reports Apache-2.0; GitHub API license is `NOASSERTION` | Not archived, pushed 2026-02-10, about 2.5k stars | Full Drools-like engine and DSL | Reference only; too broad for core primitives. |
| `rulego/rulego` | Apache-2.0 | Not archived, pushed 2026-06-26, about 1.5k stars | Component orchestration and IoT-style rule chains | Reference only; not the small in-process library shape needed here. |
| `gorules/zen-go` | MIT | Not archived, pushed 2026-03-15 | JSON Decision Model with Rust-backed native binding | Reference for decision models; native/Rust binding is too heavy for core. |
| `bytedance/arishem` | Apache-2.0 | Not archived, pushed 2025-03-24 | DSL/config-driven business rules | Reference only; DSL adoption needs separate evaluation. |
| `expr-lang/expr` | MIT | Not archived, pushed 2026-06-04, about 7.9k stars | Maintained Go expression language | Best later candidate for expression-backed rules/readers. |
| `PaesslerAG/gval` | BSD-3-Clause | Not archived, pushed 2025-08-04 | Smaller expression evaluator | Secondary expression candidate. |
| `Knetic/govaluate` | MIT | Archived | Dynamic expression evaluator | Reject for new code because the repository is archived. |

## Decision

Implement the minimal rule-engine core as a first-party Go package. The core is
small enough to keep dependency-free while preserving the important source
contracts: facts, rule ordering, rule sets, engine config, error policy, and
context cancellation. Do not adopt a full Go rule engine for the default package
because the available engines bring larger DSL, orchestration, native binding,
or decision-model surfaces than the source-parity core requires.

Use a new `rules` package for the implementation track. Keep `workflow` as a
separate execution-composition package; it should inform context handling but
should not become a mutable facts/rule engine.

## Follow-Up Issues

- #375: implement first-party rules core primitives.
- #377: add composite and bounded inference rule primitives.
- #376: evaluate expression-backed YAML and JSON rule readers.

Annotation and JVM script parity are intentionally deferred with no immediate
implementation issue. If #376 selects an expression engine, create a narrower
implementation issue for YAML/JSON readers only.

## Error And Cancellation Policy

The core package should check context before each rule evaluation and execution.
Cancellation should short-circuit with `context.Canceled` or
`context.DeadlineExceeded` preserved for `errors.Is`. Evaluation/execution
errors should appear in the engine result and obey the configured stop/continue
policy. This avoids the source engine's log-and-continue default becoming an
invisible failure path for Go callers.

## Determinism Policy

Rules should execute by priority ascending, then rule name, then registration
sequence only for exact ties. Duplicate names should be rejected unless the
registration API explicitly replaces an existing rule. Forward chaining must be
bounded by a configured max-cycle limit to avoid non-converging rule sets.

## Verification Notes

This is a research/design issue. No Go package code was added. The follow-up
implementation issues require unit, race, cancellation, and
`testing/concurrency` stress coverage.
