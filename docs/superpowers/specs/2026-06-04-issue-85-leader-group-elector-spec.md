# Issue 85 LeaderGroupElector Spec

## Context

Issue #85 adds a Go-native multi-leader election API. The existing
`leader.Elector` allows one leader per group. `LeaderGroupElector` allows up to
`MaxLeaders` concurrent leaders for bounded batch, shard, or worker-slot
coordination.

This is a Go implementation, not a Redis compatibility adapter for
`bluetape4k-leader`. The Kotlin Redis group slot-token design is reference
evidence for the ZSET model and server-side time handling.

## Goals

- Add `leader.GroupElector` and `leader.GroupOptions`.
- Implement Redis-backed group election in `leader/redis`.
- Use Redis ZSET slot tokens with expiry scores and server-side `TIME`.
- Keep acquisition bounded by caller `context.Context`.
- Renew an owned slot until resign, renewal loss, or backend failure.
- Expose `ActiveCount` and `AvailableSlots` based on live non-expired slots.
- Add Testcontainers-backed tests, runnable examples, and README locale updates.

## Non-Goals

- Do not mix Go and Kotlin/JVM Redis leader participants on the same group.
- Do not implement min-lease semantics in this issue.
- Do not add new third-party runtime dependencies.
- Do not add a local in-memory group elector.

## Public API

```go
type GroupOptions struct {
    Options
    MaxLeaders int
}

type GroupElector interface {
    Campaign(ctx context.Context) error
    Resign(ctx context.Context) error
    IsLeader() bool
    ActiveCount(ctx context.Context) (int, error)
    AvailableSlots(ctx context.Context) (int, error)
}
```

`GroupOptions.Normalize` validates `Options.Normalize` first and then requires
`MaxLeaders > 0`.

## Redis Contract

- Key: `bluetape:leader-group:<group>` by default.
- Type: Redis ZSET.
- Member: `memberID:random` slot token.
- Score: expiry time in Unix milliseconds from Redis server time.
- Acquire script:
  - prune expired scores using Redis `TIME`;
  - if `ZCARD < MaxLeaders`, add the token with `now + lease`;
  - set key TTL to `lease + small buffer`;
  - return success/failure atomically.
- Renew script:
  - extend only if this elector still owns the token;
  - return false when the token disappeared or expired.
- Release script:
  - remove only this elector's token.
- Status script:
  - prune expired slots and return active count.

## Behavior

- Duplicate `Campaign` on the same elector returns `leader.ErrAlreadyLeader`.
- When all slots are occupied, `Campaign` waits by polling until a slot opens or
  `ctx` is done. If the context expires first, return the wrapped context error.
- `Resign` is idempotent and never removes another member's slot.
- Renewal loss flips `IsLeader()` to false.
- `ActiveCount` and `AvailableSlots` never return negative values.
- Backend and context errors are wrapped so callers can use `errors.Is`.

## Tests

- Unit/API tests for `GroupOptions.Normalize`.
- Redis Testcontainers tests:
  - `MaxLeaders` electors can campaign concurrently;
  - `MaxLeaders + 1` blocks until context timeout and preserves `context` error;
  - duplicate campaign returns `ErrAlreadyLeader`;
  - repeated resign is idempotent;
  - active/available slot counts update after campaign, resign, and expiry;
  - renewal loss after token deletion flips `IsLeader` false;
  - release does not delete another elector's slot;
  - expired leaked slots are reclaimed on later acquire/status.
- Example: N workers with at most `MaxLeaders` concurrent batch runners.

## Documentation

Update `README.md` and `README.ko.md` with:

- single-leader vs group-leader distinction;
- Redis group key format;
- copy-paste Redis group elector example;
- smoke test command for the batch worker example.

## Risks

| Risk | Mitigation |
|---|---|
| Client clock skew corrupts slot expiry | Use Redis server `TIME` in Lua. |
| Slot leak after client crash | Expired ZSET entries are pruned during acquire/status. |
| Split brain when lease is shorter than work | Document caller responsibility and renew while owned. |
| Mixed Kotlin/Go Redis participants | Use Go-owned `bluetape:leader-group:<group>` key and document non-interop. |
| Poll loop load under contention | Poll at `RenewInterval` or a bounded minimum interval, never busy-spin. |

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Issue requirements captured | Done | Issue #85 acceptance criteria mapped. |
| Existing implementation inspected | Done | `leader` and `leader/redis` single elector patterns reviewed. |
| External/reference evidence scoped | Done | Kotlin slot-token design used as reference, not compatibility target. |
| Public API contract specified | Done | `GroupOptions` and `GroupElector` defined. |
| Tests and docs acceptance listed | Done | Testcontainers, examples, README locale pair included. |
