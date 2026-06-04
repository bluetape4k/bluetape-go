# Issue 24 Redis Distributed Lock Spec

Issue: #24
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## Problem

`bluetape-go` needs a small Redis lock package for service coordination tasks
that are narrower than leader election. The lock must acquire with an owner
token and TTL, unlock only the current owner, and prove contention, expiration,
cleanup, and stress behavior with Redis Testcontainers.

## Goals

- Add public package `lock/redis` with package name `redislock`.
- Use Redis `SET key token NX PX ttl` semantics through `go-redis` `SetNX`.
- Generate an owner token for each successful lock attempt unless a test token
  is explicitly supplied.
- Release by Lua compare-and-delete so a stale owner cannot delete another
  owner's lock.
- Preserve context cancellation and Redis errors through wrapped errors.
- Add unit, Testcontainers, stress, cancellation, example, README, CHANGELOG,
  research, spec, plan, review, and lessons artifacts.

## Non-Goals

- Redlock or quorum locking across multiple Redis nodes.
- Blocking lock acquisition, retry policy, or backoff loops.
- TTL renewal/extension.
- Fencing tokens.
- Kotlin/JVM Redis lock interoperability.
- Generic top-level lock abstraction before a second backend exists.

## Public API

Package path:

- `github.com/bluetape4k/bluetape-go/lock/redis`

Package name:

- `redislock`

```go
type Options struct {
    Key   string
    TTL   time.Duration
    Token string
}

type Mutex struct { ... }
type Lease struct { ... }

var ErrNotAcquired = errors.New("redis lock not acquired")

func New(client redis.Cmdable, options Options) (*Mutex, error)
func (m *Mutex) TryLock(ctx context.Context) (*Lease, error)
func (m *Mutex) Key() string
func (l *Lease) Key() string
func (l *Lease) Token() string
func (l *Lease) Unlock(ctx context.Context) (bool, error)
```

### Option Rules

- `client` is required.
- `Key` is required and trimmed only for validation.
- `TTL` must be positive.
- `Token` is optional. If provided, it must be non-empty after trimming.

### Acquire

`TryLock` normalizes nil context to `context.Background()` and checks
`ctx.Err()` before mutation. It then chooses an owner token and calls
`SetNX(ctx, key, token, ttl)`.

- Redis returns true: return a `Lease`.
- Redis returns false: return `ErrNotAcquired`.
- Redis/context error: return a wrapped error preserving `errors.Is`.

### Unlock

`Lease.Unlock` runs:

```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
```

- Script returns `1`: return `true, nil`.
- Script returns `0`: return `false, nil`; the caller no longer owns the key or
  the lock has expired.
- Redis/context error: return `false` with a wrapped error preserving
  `errors.Is`.

Returning `false` instead of a hard error makes `defer lease.Unlock(...)` usable
when the lease expired naturally, while still preserving owner safety.

## Consistency and Safety Model

- This is a single Redis instance lock.
- Mutual exclusion is bounded by Redis availability and TTL.
- Owner token protects unlock from deleting another owner's lock.
- TTL recovers locks from crashed clients.
- If a client continues work after TTL expiry, this package cannot stop it or
  provide fencing protection.
- Callers should choose TTLs larger than the protected operation and keep the
  protected operation idempotent where possible.

## Tests

Testcontainers tests:

- successful acquire writes token with positive TTL;
- second contender returns `ErrNotAcquired`;
- owner unlock removes the key;
- non-owner unlock does not delete current owner;
- expired lock can be acquired by another mutex;
- context cancellation is preserved for acquire and unlock;
- examples compile against Redis client construction.

Stress/cancellation tests:

- `GoroutineStressTester` runs many contenders against one key and proves each
  round admits at most one owner before cleanup.
- `AsyncJobTester` runs canceled acquisition attempts and verifies no leaked key
  remains.

## Documentation

- README.md and README.ko.md package table row and Redis lock section.
- CHANGELOG.md Unreleased entry.
- `docs/research/2026-06-04-issue-24-redis-distributed-lock.md`.
- `docs/lessons/2026-06-04-redis-distributed-lock.md`.

## Step 1 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Target repository confirmed | Done | `bluetape4k/bluetape-go`, branch `feat/issue-24-redis-lock`. |
| Worktree created | Done | `.worktrees/feat-issue-24-redis-lock` from `origin/develop`. |
| Issue inspected | Done | #24 acceptance criteria and stress requirement checked. |
| User intent clear | Done | Work one issue at a time, PR, then request merge approval. |
| Review-only boundary | N/A | User requested implementation work. |

## Step 1-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Official docs checked | Done | Redis `SET`, distributed lock pattern, and `EVAL` docs. |
| Current repo checked | Done | `leader/redis`, Redis Testcontainers fixture, README/CHANGELOG/package layout. |
| Third-party API checked | Done | Local `go doc` for `go-redis/v9` `SetNX`, `Eval`, and `Script`. |
| Adopt/borrow/skip decisions recorded | Done | Borrow single-instance owner-token pattern; skip Redlock, blocking wait, renew, and generic abstraction. |
| Technical constraints identified | Done | Testcontainers tests serial; stress/cancellation required; docs and GNO research required. |
