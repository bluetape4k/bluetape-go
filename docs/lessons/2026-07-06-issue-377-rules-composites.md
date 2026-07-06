# Issue #377 Rules Composite and Inference Primitives

Date: 2026-07-06

Composite groups can stay Go-native by implementing the existing `Rule`
interface instead of introducing a parallel DSL. Activation, conditional, and
unit groups should all reuse `RuleSet` ordering so priority/name/registration
tie behavior remains consistent with the core engine.

Lesson: do not store per-run child selection state on composite rule structs.
Rule values may be reused concurrently, so composite `Execute` should
re-evaluate child predicates and document that predicates must be side-effect
free. If re-evaluation no longer finds an executable child, fail closed with a
typed error instead of letting the outer engine count the composite as applied.

Lesson: forward-chaining-style behavior must be bounded. `InferenceEngine`
requires a positive `MaxCycles` and reports `ErrInferenceNonConverged` when
matching rules keep firing after the configured limit.

Lesson: inference convergence cannot use engine options that stop after the
first non-triggered rule. `StopOnFirstNotTriggered` can hide later matching rules
and produce false convergence, so bounded inference rejects that configuration.

Lesson: inference mutates `Facts` in place and is not transactional. Keep
non-convergence docs and tests explicit so callers know partial writes remain
visible after a bounded run fails.

Prevention: future expression/YAML/JSON reader work should compile into these
same first-party rule contracts instead of adding a second execution model.
