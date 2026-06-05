# Issue 86 Strategic Leader Elector

Issue: #86
Milestone: 0.3.0

## Summary

#86 is the remaining 0.3.0 implementation slice for pluggable leader election.
It adds a candidate-registry model where each node registers metadata, all nodes
apply the same deterministic strategy, and only the elected node runs the guarded
action.

## Decision

Implement a Go-owned `leader.StrategicElector` API and Redis-backed
`leader/redis` implementation. Use `bluetape4k-leader` as reference evidence,
but do not make Redis keys or serialized candidates compatible with the JVM
implementation.

## Scope

- Candidate metadata and result counters.
- FIFO, seed-stable random, and scored strategies.
- Idle-time, success-rate, candidate-weight, and weighted scorers.
- Redis shared candidate registry.
- Testcontainers, stress, cancellation, examples, and README locale updates.

## Related Artifacts

- Superpowers research:
  `docs/superpowers/research/2026-06-05-issue-86-strategic-leader-elector-research.md`
- Spec:
  `docs/superpowers/specs/2026-06-05-issue-86-strategic-leader-elector-spec.md`
- Plan:
  `docs/superpowers/plans/2026-06-05-issue-86-strategic-leader-elector-plan.md`
