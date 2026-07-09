# redis

`github.com/bluetape4k/bluetape-go/redis` provides small Redis safety
primitives shared by Redis-backed bluetape-go packages. The package name is
`btredis`.

## Scope

This package is not a generic Redis client facade. It does not own Redis
connections, retries, logging, metrics, tenant isolation, or package-specific
key authorization. Callers keep their `go-redis` clients and deadlines.

Issue #569 only adds the foundation package. Existing packages such as
`lock/redis`, `leader/redis`, `ratelimit/redis`, and `probabilistic/redis` are
not migrated here. Follow-up migrations must add old/new key parity tests and
benchmark evidence before replacing package-local helpers.

## Keys

`KeyBuilder` separates package-owned structural parts from one caller-owned
logical key segment.

- Prefixes may be colon-delimited package strings such as
  `bluetape:probabilistic:bloom:v1`.
- Structural parts reject empty values, braces, and `:` delimiters.
- `LogicalKey` preserves caller-owned key bytes verbatim, including spaces,
  braces, and colons.
- `WithHashTag` preserves the hash tag verbatim while rejecting empty or braced
  tags. Colons are allowed because existing probabilistic Redis namespaces use
  them.
- Hash tags are Redis Cluster same-slot helpers, not tenant isolation or
  authorization boundaries.

Use `Key.RedactedID` or `RedactedKeyID` in diagnostics. The id is a stable
correlation handle for trusted operational logs, not anonymization; low-entropy
keys can still be guessed by recomputing candidate IDs. `Key.Value` is Redis
command input and may contain caller key material.

## Owner Tokens

`OwnerToken` values are 256-bit random lowercase-hex Redis comparison
credentials. `String`, `GoString`, and `slog.LogValuer` formatting are redacted.
Only pass `RedisValue()` to Redis command arguments; do not log it.

## Lease Scripts

`CompareAndDelete` and `CompareAndExtend` use package-level Lua scripts through
`redis.NewScript`. They validate nil contexts, canceled contexts, nil clients,
leases, and TTLs before Redis dispatch.

Always use a caller-owned timeout:

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()
ok, err := btredis.CompareAndDelete(ctx, client, lease, "redis lock")
```

`(false, nil)` means ownership drift: the key no longer contains the lease
owner token. It is not an infrastructure error.

After a command has been dispatched, cancellation or deadline errors can leave
the commit state indeterminate from the caller's point of view. Inspect Redis
state or retry through an idempotent owner workflow before assuming the delete
or extend did or did not commit.

## Errors And Runbook

Redis script/client failures return `OpError`. `OpError.Error()` is sanitized:
it includes low-cardinality family/operation labels and a redacted key id, but
not raw keys, owner tokens, or provider error text. Use `errors.Is`,
`errors.As`, or `errors.Unwrap` to inspect the cause.

Operational checks:

- `false, nil`: owner drift; reacquire or stop acting as owner.
- `context.Canceled` / `context.DeadlineExceeded`: caller timeout path; inspect
  Redis state if the command may have been dispatched.
- `OpError`: Redis script/client path; inspect the unwrapped cause and redacted
  key id.
- Partial failures: the redacted key id is correlation-only unless the caller
  stores a safe lookup handle. For cleanup, enumerate the caller-owned namespace
  with bounded `SCAN` / `MATCH` / `COUNT`, recompute `RedactedKeyID` locally to
  match the failing id, dry-run the candidate set before delete, and avoid
  blocking broad `KEYS` scans in production. Do not log raw keys or tokens.
- Rollback: #569 does not migrate existing Redis packages, so rollback is
  removing consumers of the new package, not changing existing Redis behavior.

## Verification

```bash
go test -count=1 ./redis
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
go test -count=1 ./redis -run Example
```
