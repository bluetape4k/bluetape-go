# Issue #24 Redis Distributed Lock Research

Issue: #24
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## Research Question

How should `bluetape-go` expose a small Redis distributed lock package that uses
owner tokens and TTL while staying idiomatic to Go and testable with
Testcontainers?

## Decision

Implement a single-Redis-instance lock package at `lock/redis` with package name
`redislock`.

The lock acquisition primitive is:

```text
SET key token NX PX ttl
```

The unlock primitive is a Lua script that deletes the key only when the stored
value still equals the owner token.

## Source Evidence

| Source | Relevant point | Decision impact |
|---|---|---|
| Redis `SET` command docs, https://redis.io/docs/latest/commands/set/ | `SET` supports `NX` and millisecond expiration (`PX`). | Use `go-redis` `SetNX(ctx, key, token, ttl)` for atomic acquire with TTL. |
| Redis distributed lock docs, https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/ | Single-instance acquire uses a random value and release must compare that value before deleting. | Store an owner token and use compare-before-delete Lua unlock. |
| Redis `EVAL` docs, https://redis.io/docs/latest/commands/eval/ | Scripts receive keys through `KEYS` and additional arguments through `ARGV`; accessed keys should be explicit. | Unlock script takes one key and the token as `ARGV[1]`. |
| Local `leader/redis` package | Existing leader election uses `SetNX` plus Lua release/renew with random `memberID:token`. | Reuse the same owner-token and Lua safety pattern without depending on leader semantics. |
| `go-redis/v9` local docs | `Client.SetNX` returns `*BoolCmd`; `Client.Eval` returns `*Cmd`; `redis.Cmdable` already covers both. | Accept `redis.Cmdable` directly, matching existing `leader/redis`. |

## API Direction

```go
package redislock

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
func (l *Lease) Token() string
func (l *Lease) Unlock(ctx context.Context) (bool, error)
```

`Options.Token` is optional. When empty, each successful `TryLock` attempt uses
a freshly generated random token. When provided, it is useful for deterministic
tests and interoperability experiments.

## Non-Goals

- Do not implement Redlock quorum over multiple Redis nodes.
- Do not implement blocking wait/retry loops.
- Do not implement TTL renewal/extend in #24.
- Do not expose a generic lock abstraction before more backends exist.
- Do not make Kotlin/JVM Redis lock key/value interop a supported contract.

## Test Requirements

- Two clients contending for the same key: one succeeds, the other returns
  `ErrNotAcquired`.
- Unlock deletes only the owner token.
- Unlock after expiration or after another owner acquired the key returns
  `false, nil` without deleting the new owner.
- TTL expiration lets another owner acquire the same key.
- Context cancellation is preserved with `errors.Is`.
- `GoroutineStressTester` proves concurrent contention does not admit multiple
  owners at once.
- `AsyncJobTester` proves canceled attempts return cleanly and do not leave
  keys behind.

## Operational Boundary

This lock is a small Redis primitive for mutual exclusion on one Redis instance.
It is useful for low/medium risk service coordination where TTL leakage
recovery is sufficient. It does not provide fencing tokens and does not protect
external systems from a client that keeps working after its TTL expires.
