# Lessons Learned - PostgreSQL Rate Limiter (2026-07-13)

**Related issue:** #529
**Affected modules:** `ratelimit`, `ratelimit/ratelimittest`, `ratelimit/redis`, `ratelimit/sql`

## L1: Pool acquisition is part of the commit-unknown boundary

### Problem

Calling `DB.QueryRowContext` directly does not let a provider distinguish a
context canceled while waiting for a pooled connection from a context canceled
after PostgreSQL dispatch. Treating both as commit-unknown is safe but loses the
strong no-dispatch result and weakens caller diagnostics.

### Decision

Acquire a dedicated caller-owned `*sql.Conn` first. Return the original context
error when acquisition is canceled, emit a typed determinate provider error for
other acquisition failures, and classify only query/scan failures as possibly
dispatched.

### Future guard

Distributed `database/sql` providers that expose commit-unknown semantics must
test pool-wait cancellation separately from row-lock or response-loss
cancellation.

## L2: `IF NOT EXISTS` needs an exact hostile-catalog preflight

### Problem

`CREATE TABLE/INDEX IF NOT EXISTS` does not prove that an existing object has the
required owner, relation kind, columns, constraints, index target, RLS policy,
or trigger state. A same-name hostile object can make migration appear complete
and fail only after runtime traffic begins.

### Decision

Keep migration caller-owned, but require a pre-traffic catalog proof for the
entire supported relation. Test both the exact schema and hostile mutations,
including a same-name expiry index on another relation.

### Future guard

Every fixed-schema SQL provider should pair its migration constant with an
operator-visible catalog checklist and hostile-object integration cases.

## L3: Conformance must isolate scheduling latency from refill semantics

### Problem

At 100 tokens per second, a missing token refills in 10 ms. Race instrumentation
and database adapter latency can cross that window, so debit-preservation cases
can legitimately observe a refilled bucket and falsely report a contract
failure. A fixed refill sleep has the inverse problem: it assumes scheduler and
server clocks have advanced enough.

### Decision

Use a refill interval longer than the whole case timeout for assertions that
require no refill. For the refill contract, retry only rejected results within a
bounded deadline and honor the provider's positive `RetryAfter`.

### Future guard

Timing tests should make time irrelevant to negative assertions and use
condition-based bounded waits for positive eventual behavior. Reproduce a
suspected flaky test repeatedly before labeling it baseline noise.

## L4: Cleanup bounds and scan bounds are different claims

### Problem

Adding primary-key tie breakers to expiry ordering forced PostgreSQL to sort a
large expired backlog even though an expiry index existed. Removing the Sort
does not mean `SKIP LOCKED` scans at most `limit` rows; locked rows may still be
visited and skipped.

### Decision

Order by the indexed expiry column only, prove the large-backlog plan has an
expiry-index scan and no Sort, and document that `limit` bounds locks/deletes
while caller timeout and pressure budgets bound the scan.

### Future guard

Performance documentation must state exactly which resource is bounded and use
an execution-plan regression for the access path it relies on.
