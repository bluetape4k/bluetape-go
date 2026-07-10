# Redis Lock Substrate Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reuse the shared Redis substrate in `lock/redis` without changing its public API, stored key bytes, or legacy custom-token behavior.

**Architecture:** `Mutex.TryLock` uses `btredis.NewOwnerToken` only for default-generated ownership and records a compatible `btredis.Lease` when the selected token is canonical. `Lease.Unlock` dispatches `btredis.CompareAndDelete` for that canonical lease; legacy custom tokens retain the existing Lua command but convert provider failures to a redacted `btredis.OpError`. TTL validation remains local because the legacy package permits any positive duration, while the shared helper requires at least one millisecond.

**Tech Stack:** Go 1.26, `github.com/redis/go-redis/v9`, existing Redis Testcontainers fixture, `github.com/bluetape4k/bluetape-go/redis`.

---

## File Map

| File | Responsibility |
|---|---|
| `lock/redis/mutex.go` | Select generated/shared versus custom compatibility ownership path and redact acquire/unlock provider errors. |
| `lock/redis/mutex_test.go` | Lock contract regression cases using one serial Testcontainers Redis instance per test. |
| `lock/redis/README.md` | Explain error-cause preservation and redacted Redis diagnostics. |
| `lock/redis/README.ko.md` | Korean parity for the same operational guarantee. |

### Task 1: Lock Contract Regression Tests

**complexity:** medium

**Files:**
- Modify: `lock/redis/mutex_test.go`

- [ ] **Step 1: Write failing generated-token substrate coverage**

Add a test that acquires with an empty `Options.Token`, parses the public
`Lease.Token()` through the shared package, and unlocks normally:

```go
func TestMutexGeneratedTokenUsesSharedOwnerToken(t *testing.T) {
    ctx := context.Background()
    client := redisClient(ctx, t)
    mutex := newMutex(t, client, testLockKey(t), "", time.Second)

    lease, err := mutex.TryLock(ctx)
    if err != nil {
        t.Fatalf("try lock: %v", err)
    }
    if _, err := btredis.ParseOwnerToken(lease.Token()); err != nil {
        t.Fatalf("generated token should be canonical: %v", err)
    }
    released, err := lease.Unlock(ctx)
    if err != nil || !released {
        t.Fatalf("unlock generated lease: released=%t err=%v", released, err)
    }
}
```

- [ ] **Step 2: Run the new test before implementation**

Run: `go test -p 1 -count=1 ./lock/redis -run TestMutexGeneratedTokenUsesSharedOwnerToken`

Expected: FAIL because the legacy generator emits a 32-character token.

- [ ] **Step 3: Write failing custom-token compatibility coverage**

Add a test with `Token: " owner-a "` that asserts the existing normalized
`lease.Token() == "owner-a"`, reads the same Redis value, and performs a
successful unlock. This protects the legacy non-canonical script path.

- [ ] **Step 4: Add redacted provider-error coverage**

Use a closed `*redis.Client`, a key containing a unique secret marker, and a
custom token containing a second marker. Assert `TryLock` and `Unlock` retain
`errors.Is(err, redis.ErrClosed)` while neither error string contains either
marker.

- [ ] **Step 5: Run focused regression tests**

Run: `go test -p 1 -count=1 ./lock/redis -run 'GeneratedToken|CustomToken|Redacted'`

Expected: generated-token and redaction tests fail before Task 2; the custom
token test passes against the current compatibility behavior.

### Task 2: Minimal Shared-Substrate Adoption

**complexity:** high

**Files:**
- Modify: `lock/redis/mutex.go`

- [ ] **Step 1: Add the shared substrate import and lease field**

Import `btredis "github.com/bluetape4k/bluetape-go/redis"` and add a private
optional `sharedLease *btredis.Lease` to `Lease`. Keep `key` and `token` fields
and all exported methods unchanged.

- [ ] **Step 2: Replace only default token generation**

Replace `randomToken()` with:

```go
ownerToken, err := btredis.NewOwnerToken()
if err != nil {
    return nil, fmt.Errorf("generate redis lock token: %w", err)
}
token = ownerToken.RedisValue()
```

After choosing either generated or caller token, create the optional shared
lease only for canonical values:

```go
var sharedLease *btredis.Lease
if ownerToken, err := btredis.ParseOwnerToken(token); err == nil {
    lease, err := btredis.NewLease(m.opts.key, ownerToken)
    if err != nil {
        return nil, err
    }
    sharedLease = &lease
}
```

Do not reject a non-canonical caller token.

- [ ] **Step 3: Redact acquire provider failures**

Wrap a `SetNX` command failure as:

```go
return nil, btredis.NewOpError(
    btredis.OpLabels{Family: "lock", Operation: "acquire"},
    m.opts.key,
    err,
)
```

Do not wrap `ctx.Err()` or `ErrNotAcquired`; their current caller-visible
identity remains unchanged.

- [ ] **Step 4: Route canonical unlocks through the shared helper**

After preserving the existing nil/canceled-context checks, use:

```go
if l.sharedLease != nil {
    return btredis.CompareAndDelete(ctx, l.mutex.client, *l.sharedLease, "lock")
}
```

For a custom non-canonical token, retain the private `unlockScript` execution
and replace only its error return with `btredis.NewOpError` using family `lock`
and operation `compare-delete`. Owner mismatch must still return `(false, nil)`.

- [ ] **Step 5: Preserve TTL compatibility explicitly**

Leave `options.normalize` validation as `o.TTL <= 0`. Do not call
`btredis.ValidateTTL`: the shared package rejects sub-millisecond durations and
would change the current public option contract.

- [ ] **Step 6: Format and run focused tests**

Run:

```bash
gofmt -w lock/redis/mutex.go lock/redis/mutex_test.go
go test -p 1 -count=1 ./lock/redis
go test -p 1 -race -count=1 ./lock/redis
```

Expected: PASS. Redis Testcontainers execution remains serial (`-p 1`).

### Task 3: Documentation And Verification

**complexity:** low

**Files:**
- Modify: `lock/redis/README.md`
- Modify: `lock/redis/README.ko.md`

- [ ] **Step 1: Document sanitized operational errors**

Add one behavior bullet in each locale: Redis command failures retain their
cause for `errors.Is`/`errors.As`, while diagnostic messages redact raw keys
and owner tokens. Do not claim a new public lock feature.

- [ ] **Step 2: Verify source/document parity and full local contract**

Run:

```bash
go test -p 1 -count=1 ./redis ./lock/redis
go test -p 1 -race -count=1 ./lock/redis
go test -count=1 ./lock/redis -run Example
git diff --check
rg -n 'redact|redacted|오류|에러' lock/redis/README.md lock/redis/README.ko.md
```

- [ ] **Step 3: Run repository verification before review**

Run: `make ci`

Expected: PASS. No benchmark is run because this issue does not alter an
algorithm or provider throughput; issue #560 owns benchmark measurement,
table, chart, and analysis obligations.

### Task 4: Review, Lessons, And Publication

**complexity:** medium

**Files:**
- Create: `docs/review/2026-07-10-issue-579-redis-lock-substrate-review.md`
- Create: `docs/lessons/2026-07-10-issue-579-redis-lock-substrate.md`

- [ ] **Step 1: Verify implementation against this spec and plan**

Confirm each invariant is covered by an implementation assertion and fresh
test evidence. Record any intentional no-op, especially local TTL validation.

- [ ] **Step 2: Run the mandatory six-perspective 7-Tier review**

Review the `develop...HEAD` diff for performance, stability, security,
operator/Ops, developer/API, and user/caller concerns. Normalize findings and
do not publish with P0/P1 findings. Record `P0=0 P1=0` in the review artifact.

- [ ] **Step 3: Commit with Lore trailers and create a PR closing #579**

Use an English intent-first commit message with Constraint, Rejected,
Confidence, Scope-risk, Directive, Tested, and Not-tested trailers. The PR
body ends with `## DoD Status` and includes the benchmark N/A rationale.

## Rollback

Revert the migration commit. The shared `redis` package remains independently
usable because it was introduced and merged in #578; reverting #579 restores
the original local token/script/error implementation without any data or key
migration.
