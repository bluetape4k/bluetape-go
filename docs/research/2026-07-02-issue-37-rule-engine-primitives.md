# Issue #37 Rule Engine Primitives Research

## 맥락

Issue #37은 `bluetape4k-rule-engine` 중 어느 범위가 Go code가 되어야 하고,
어느 부분을 dependency-backed 또는 deferred로 남겨야 하는지 묻는다. Source
module은 core facts/rules/rule sets, priority-based execution, Kotlin DSL과
annotation registration, coroutine-aware execution, composite groups, forward
chaining, script-backed rules, YAML/JSON/HOCON readers를 포함한다.

Go repository에는 현재 rule-engine package가 없다. 가장 가까운 local
reference는 `workflow`이며, 이미 `context.Context` cancellation과 conditional
execution을 모델링한다. 하지만 `workflow`는 의도적으로 mutable shared state와
durable rule-engine semantics를 피한다.

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
| `hyperjumptech/grule-rule-engine` | README reports Apache-2.0; GitHub API license is `NOASSERTION` | Not archived, pushed 2026-02-10, about 2.5k stars | Full Drools-like engine and DSL | Reference only; core primitive에는 너무 넓다. |
| `rulego/rulego` | Apache-2.0 | Not archived, pushed 2026-06-26, about 1.5k stars | Component orchestration and IoT-style rule chains | Reference only; 여기서 필요한 small in-process library shape가 아니다. |
| `gorules/zen-go` | MIT | Not archived, pushed 2026-03-15 | JSON Decision Model with Rust-backed native binding | Decision model reference로만 본다. Native/Rust binding은 core에 너무 무겁다. |
| `bytedance/arishem` | Apache-2.0 | Not archived, pushed 2025-03-24 | DSL/config-driven business rules | Reference only; DSL adoption은 별도 평가가 필요하다. |
| `expr-lang/expr` | MIT | Not archived, pushed 2026-06-04, about 7.9k stars | Maintained Go expression language | Expression-backed rules/readers의 later candidate로 가장 적합하다. |
| `PaesslerAG/gval` | BSD-3-Clause | Not archived, pushed 2025-08-04 | Smaller expression evaluator | Secondary expression candidate. |
| `Knetic/govaluate` | MIT | Archived | Dynamic expression evaluator | Repository가 archived이므로 new code에서는 기각한다. |

## 결정

Minimal rule-engine core를 first-party Go package로 구현한다. Core는
dependency-free로 유지할 만큼 작고, source의 중요한 contract인 facts, rule
ordering, rule sets, engine config, error policy, context cancellation을 보존할
수 있다. Available Go engine은 source-parity core가 요구하는 것보다 큰 DSL,
orchestration, native binding, decision-model surface를 가져오므로 default
package로 채택하지 않는다.

Implementation track에는 새 `rules` package를 사용한다. `workflow`는 별도의
execution-composition package로 유지한다. `workflow`는 context handling에는
참고가 되지만 mutable facts/rule engine이 되면 안 된다.

## 후속 이슈

- #375: implement first-party rules core primitives.
- #377: add composite and bounded inference rule primitives.
- #376: evaluate expression-backed YAML and JSON rule readers.

Annotation과 JVM script parity는 immediate implementation issue 없이 의도적으로
보류한다. #376이 expression engine을 선택하면 YAML/JSON reader만을 위한 더
좁은 implementation issue를 만든다.

## Error And Cancellation Policy

Core package는 각 rule evaluation과 execution 전에 context를 확인해야 한다.
Cancellation은 `errors.Is`에서 보존되는 `context.Canceled` 또는
`context.DeadlineExceeded`로 short-circuit해야 한다. Evaluation/execution
error는 engine result에 나타나고 configured stop/continue policy를 따라야
한다. 이렇게 해야 source engine의 log-and-continue default가 Go caller에게
보이지 않는 failure path가 되는 일을 피할 수 있다.

## Determinism Policy

Rule은 priority ascending, 그다음 rule name, exact tie일 때만 registration
sequence 순서로 실행해야 한다. Registration API가 명시적으로 existing rule을
replace하지 않는 한 duplicate name은 거부해야 한다. Forward chaining은
non-converging rule set을 피하기 위해 configured max-cycle limit으로 제한해야
한다.

## 검증 메모

이 이슈는 research/design issue다. Go package code는 추가하지 않았다. Follow-up
implementation issue는 unit, race, cancellation, `testing/concurrency` stress
coverage를 요구한다.
