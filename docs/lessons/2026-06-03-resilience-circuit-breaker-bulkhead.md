# Resilience Circuit Breaker and Bulkhead Workflow

Issue #19 confirms the right Go shape for resilience state machines: keep the
runtime first-party, use explicit `context.Context` boundaries, and make timing
deterministic through injected time sources rather than sleeps.

For graph-aware review, stage or otherwise register new files before asking
code-review-graph for review context. Untracked Go files may not appear in the
initial changed-file set even after the graph itself can parse them.

For circuit breaker and bulkhead work, tests should control concurrency with
channels and fake clocks. Avoid tests that depend on arbitrary sleep intervals
for half-open transitions or permit release.
