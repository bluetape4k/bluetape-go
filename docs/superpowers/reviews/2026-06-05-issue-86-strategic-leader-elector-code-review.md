# Issue 86 Strategic Leader Elector Code Review

Issue: #86
Milestone: 0.3.0
Branch: `feat/issue-86-strategic-leader-elector`

## Verdict

P0=0 P1=0

No blocking issues found in the local review pass.

## Seven-Tier Review

1. Contract and API
   - `leader.StrategicElector` exposes register, unregister, list, result update,
     and guarded execution without binding callers to Redis.
   - Strategies operate on `leader.CandidateInfo` and return an optional winner.

2. Determinism
   - FIFO uses registration time with node ID tie-break.
   - Random sorts node IDs before seed-stable PCG selection.
   - Scored strategy uses FIFO ordering as a deterministic tie-break.

3. Redis correctness
   - Candidate registration stores JSON plus a live ZSET index in one Lua script.
   - Listing uses Redis server time, prunes expired ZSET entries, and removes
     missing candidate references.
   - Result updates increment counters atomically with Lua while preserving TTL.

4. Concurrency
   - Stress test registers and elects across multiple Redis-backed electors.
   - Exact concurrent result update test verifies no lost success increments.
   - Race test passed for the leader and Redis strategic suites.

5. Cancellation and errors
   - Context cancellation is preserved through Redis operations.
   - Missing or expired candidate result updates return `leader.ErrNotLeader`.
   - `RunIfLeader` joins action and update errors without hiding action failure.

6. Documentation and examples
   - Package docs, README files, root README locale pair, changelog, and WIP
     describe the strategic elector.
   - `ExampleNewStrategic` compile-checks the scored idle-time election path.

7. Scope and maintainability
   - No new dependencies.
   - Implementation stays in `leader` and `leader/redis`.
   - Research, spec, plan, lesson, and review artifacts are linked to #86.

## Verification Evidence

- `go test -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress|ExampleNewStrategic'`: 14 passed in 2 packages.
- `go test -race -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'`: 14 passed in 2 packages.

## Residual Risk

`RunIfLeader` intentionally uses strategy selection over the current candidate
snapshot; it does not implement a separate Redis lease lock. This matches the
#86 strategy-registry scope and should be considered if future work needs
exclusive execution under clock skew or delayed candidate refresh.
