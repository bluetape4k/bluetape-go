# Issue #571 Redis Streams Primitive Spec Review

## Scope

- Spec: `docs/superpowers/specs/2026-07-10-issue-571-redis-streams-spec.md`
- Test specification: `docs/superpowers/specs/2026-07-10-issue-571-redis-streams-test-spec.md`
- Existing provider: `audit/sqloutbox/redisstreams/publisher.go`
- Shared Redis dependency: `redis/errors.go`
- Review mode: local six-perspective equivalent. Native subagent spawning is
  not exposed in this session; the main session independently applied every
  required perspective and owns the integration verdict.

## Findings

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Helpers issue one caller-requested Redis command and own no polling loop, allocation-heavy envelope, or hidden trim/retry. Benchmark execution is correctly deferred to #560. |
| Stability | 0 | 0 | 0 | 0 | Pre-dispatch context checks, bounded Testcontainers tests, pending/recovery coverage, and a cancellation ambiguity rule are explicit. |
| Security | 0 | 0 | 0 | 0 | Raw stream keys and provider text remain outside formatted errors through `btredis.OpError`; payload values remain caller-owned and are not logged or encoded. |
| Operator/Ops | 0 | 0 | 0 | 0 | At-least-once, pending, replay, retention, trim, delete, and shutdown ownership are documented without claiming automatic recovery. |
| Developer/API | 0 | 0 | 0 | 0 | Narrow per-command interfaces preserve ordinary `go-redis` response types and keep scope bounded to `XAUTOCLAIM`, not a broad facade. |
| User/Caller | 0 | 0 | 0 | 0 | Names and payloads remain verbatim, and #533 migrates only the dispatch path while retaining its audit envelope and duplicate behavior. |

## Resolved Finding

| Priority | Finding | Resolution | Evidence |
|---|---|---|---|
| P1 | The first draft described `XREAD`/`XREADGROUP` arguments as interleaved `stream, id` pairs. go-redis expects all stream keys followed by their IDs. | Updated both specs to require an even list in all-streams-then-all-IDs order and to validate only the key half as stream names. | `go doc github.com/redis/go-redis/v9.XReadGroupArgs`, updated spec/test spec. |

## Integration Verdict

The specification is implementable without creating a generic Redis facade or
changing #533's public provider behavior. It deliberately exposes command
semantics rather than inventing delivery policy, and it makes the operational
cost of at-least-once delivery visible to callers.

P0=0 P1=0
