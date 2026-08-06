# Issue #376 Expression-Backed Rule Reader Research

Date: 2026-07-06

## 결정

follow-up issue에서 좁고 optional한 `expr-lang/expr` backed reader를 채택한다. 단, 첫 implementation slice는
expression-backed predicate에만 한정하고 expression-backed action은 추가하지 않는다.

reader는 YAML/JSON definition을 기존 first-party `rules.Rule` contract로 compile해야 한다. 두 번째 execution engine,
JVM-style annotation model, arbitrary Go code execution, HOCON support, unbounded forward chaining은 도입하지 않는다.

## Repo-Local Constraints

- `rules.Rule`은 이미 `Evaluate(context.Context, *Facts)`와 `Execute(context.Context, *Facts) error`를 소유한다.
- `RuleSet`, composite group, `InferenceEngine`은 deterministic ordering, fail-closed composite drift behavior,
  max-cycle bounded inference, context-aware execution을 이미 정의한다.
- future reader는 이 contract를 우회하지 말고 compile target으로 사용해야 한다.
- inference는 `Facts`를 in-place mutate하며 transactional하지 않다. reader docs는 reader가 inference와 함께 쓰일 때 이 점을
  계속 보이게 해야 한다.

## Candidate Matrix

| Candidate | Decision | 이유 |
|---|---|---|
| `github.com/expr-lang/expr` | Prefer for follow-up implementation | active, MIT, Go-centric expression language다. README는 memory-safe, side-effect-free, always-terminating expression을 설명한다. `Env`, `AsBool`, builtin disabling, custom function, `MaxNodes`, `WithContext`가 predicate adapter에 잘 맞는다. |
| `github.com/PaesslerAG/gval` | fallback/reference | active enough이고 BSD-3-Clause다. `EvaluateWithContext`, composable grammar가 있지만 static type checking이 `expr`보다 약하고 runtime/type-conversion driven behavior가 많다. |
| `github.com/Knetic/govaluate` | Reject | archived이며 `ARCHIVED.md`가 active alternative로 `expr`를 권한다. 새 bluetape-go surface에는 적합하지 않다. |
| `github.com/hyperjumptech/grule-rule-engine` | reference only | active full rule engine이지만 GRL DSL, knowledge base, inference model을 따로 소유하므로 `rules.Rule` reader로는 너무 넓다. |
| `github.com/rulego/rulego` | reference only | component orchestration/rule-chain framework이며 protocol/action component와 plugin concept가 넓다. first-party rules predicate reader보다 크다. |
| `github.com/gorules/zen-go` | reference only | JDM/BRMS에는 강하지만 native binding lifecycle과 `Dispose`가 필요하다. first-party predicate reader에는 과하다. |
| `github.com/bytedance/arishem` | reference only | JSON-compatible DSL rule engine이지만 global initialization과 별도 execution model을 가진다. docs도 주로 Chinese다. |

## Proposed Reader Scope

follow-up issue에서 `rules/exprreader` 또는 `rules/readers/exprjson` 같은 optional package를 만들고
`github.com/expr-lang/expr`에 explicit dependency를 둔다.

첫 implementation은 다음을 지원한다.

- 같은 schema의 YAML 및 JSON document.
- predicate expression only.
- `Facts`를 predefined operation으로 mutate하는 아주 작은 whitelist의 built-in action.
- compile-on-load 및 run-many execution.
- 모든 predicate에 `expr.AsBool()`.
- `Facts`에서 좁은 map/struct projection을 사용하는 `expr.Env`.
- conservative default를 가진 `expr.MaxNodes`.
- 기본적으로 `expr.DisableAllBuiltins()`를 사용하고 필요할 때만 explicit `EnableBuiltin` 또는 whitelisted `Function` 등록.
- `context.Context`를 받는 allowed custom function에는 `expr.WithContext("ctx")`로 context propagation.

지원하지 않는 항목:

- YAML/JSON에 선언된 arbitrary Go callback.
- unbounded loop, script block, reflection registration, annotation, external DSL action.
- side-effectful expression predicate.
- 첫 Go reader의 HOCON.

## Minimal YAML/JSON Schema

`expr`가 predicate evaluation에 적합하므로 schema를 의도적으로 작게 둔다.

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
engine:
  stopOnFirstApplied: false
  stopOnFirstFailed: true
  usePriorityThreshold: false
```

schema rule:

- `version`은 required이며 `1`이어야 한다.
- `rules`는 required이며 non-empty여야 한다.
- `name`은 required이며 기존 `rules` blank-key validation을 따른다.
- `priority` default는 `0`이다.
- `when`은 required이며 bool로 compile되어야 한다.
- `then`은 optional이다. 있으면 whitelisted declarative action만 허용한다. 첫 slice는 `set`만 지원한다.
- `engine`은 optional이며 safe `rules.EngineConfig` field에만 매핑한다.
- reader-backed inference는 later issue가 명시적으로 추가할 때만 별도 top-level `inference` block에 둔다. 그때도
  `NewInferenceEngine`과 맞춰 `stopOnFirstNotTriggered`는 거부해야 한다.

## Error Policy

`errors.Is` / `errors.As`와 호환되는 typed reader error를 추가한다.

- `ErrInvalidRuleDocument`: malformed YAML/JSON 또는 schema-level validation.
- `ErrInvalidRuleExpression`: expression parse/compile/type-check failure.
- `ErrInvalidRuleAction`: unsupported action 또는 invalid action payload.
- `ReaderError{RuleName, Field, Err}`: rule-local wrapping.

rule 하나라도 invalid이면 compilation은 전체 document를 fail한다. runtime predicate error는 `Rule.Evaluate`로 반환하고,
action error는 `Rule.Execute`로 반환한다. engine behavior는 기존 `ErrRuleEvaluation` 및 `ErrRuleExecution` wrapping을 재사용한다.

## Context-Cancellation Policy

- loading/parsing은 `context.Context`를 받고 decode, schema validation, expression compile, rule construction 각 phase 전에
  context를 확인한다.
- expression evaluation은 generated `Rule.Evaluate`를 통해 caller context를 받는다.
- `expr.Run` 자체는 `context.Context`를 받지 않으므로 adapter가 `expr.Run` 호출 전에 `ctx.Err()`를 확인해야 한다.
- block 또는 I/O 가능성이 있는 whitelisted custom function은 반드시 `context.Context`를 받고 `expr.WithContext("ctx")`로 연결한다.
- 첫 implementation은 I/O custom function을 피한다. reader loading은 caller가 넘긴 `io.Reader` / `[]byte`에서만 읽는다.

## Implementation Issues

follow-up implementation issue 하나를 만든다.

- `feat: add expr-backed YAML and JSON rules reader`

acceptance에는 schema test, compile-time type test, runtime predicate/action error test, context cancellation test,
`go test -race -count=1 ./rules ./rules/...`가 포함되어야 한다.

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
