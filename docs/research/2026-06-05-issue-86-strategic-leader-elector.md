# Issue 86 Strategic Leader Elector 연구

Issue: #86
Milestone: 0.3.0

## 요약

#86은 pluggable leader election을 위한 남은 0.3.0 implementation slice다. 각 node가
metadata를 등록하고, 모든 node가 동일한 deterministic strategy를 적용하며, elected
node만 guarded action을 실행하는 candidate-registry model을 추가한다.

## 결정

Go가 소유하는 `leader.StrategicElector` API와 Redis-backed `leader/redis` 구현을
추가한다. `bluetape4k-leader`는 reference evidence로 사용하되, Redis key나
serialized candidate를 JVM implementation과 compatible하게 만들지는 않는다.

## 범위

- candidate metadata 및 result counters.
- FIFO, seed-stable random, scored strategies.
- idle-time, success-rate, candidate-weight, weighted scorers.
- Redis shared candidate registry.
- Testcontainers, stress, cancellation, examples, README locale updates.

## 관련 산출물

- Superpowers research:
  `docs/superpowers/research/2026-06-05-issue-86-strategic-leader-elector-research.md`
- Spec:
  `docs/superpowers/specs/2026-06-05-issue-86-strategic-leader-elector-spec.md`
- Plan:
  `docs/superpowers/plans/2026-06-05-issue-86-strategic-leader-elector-plan.md`
