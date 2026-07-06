# Issue #375 Rules Core Primitives

Date: 2026-07-05

The first `rules` slice intentionally stayed dependency-free: `Facts`, plain Go
`Rule` contracts, deterministic `RuleSet`, sequential `Engine`, typed errors,
and context-aware result reporting. Expression readers, composites, and forward
chaining remain follow-up scope.

Lesson: rule engines fail open easily when failures are recorded only in details.
Even when `StopOnFirstFailed` is false, return an error if any rule fails, and
teach callers to inspect `Result.Failed` before trusting mutated facts.

Lesson: public `Rule` implementations are caller code. Capture rule name and
priority at registration and never call metadata methods while holding internal
locks or sorting an engine run.

Prevention: future rules work should add regression tests for failure-after-fact
mutation, rule-returned context cancellation, zero-value typed errors, and cached
registration metadata before expanding into composites or readers.
