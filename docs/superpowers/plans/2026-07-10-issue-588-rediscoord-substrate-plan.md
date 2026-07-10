# Redis Cache Coordinator Substrate Migration Plan

**Goal:** Standardize `cache/rediscoord` direct Redis provider diagnostics on
the shared substrate without changing cache-coordination behavior or stored
data.

**Architecture:** Keep local key formatting, opaque result envelopes, existing
duration handling, and the migrated `lock/redis` lease boundary. Add one small
operation-error helper that joins a late caller context with the provider cause
and constructs a redacted `redis.OpError`.

## Task 1: Failing Regression Coverage

- [x] Add closed-client tests for `readOwnerResult`, `ownerToken`,
  `ensureOwner`, and `storeResult`.
- [x] Assert original cause, typed operation error, stable redacted key ID, and
  no raw key/token/payload marker in formatted errors.
- [x] Add a direct unit-level late-context test. It deterministically verifies
  the error boundary without relying on an unreliable network race.
- [x] Run the focused test set and capture the pre-implementation failure.

## Task 2: Minimal Error-Boundary Migration

- [x] Import the shared `redis` package as `btredis`.
- [x] Add a private operation-error helper with stable family/operation labels.
- [x] Replace only direct provider error returns from result read, owner read,
  owner check, and result write.
- [x] Preserve `redis.Nil` branches and all preflight/sentinel behavior.
- [x] Format and run targeted serial tests.

## Task 3: Documentation And Validation

- [x] Update English and Korean package README only to describe the preserved
  `errors.Is`/`errors.As` cause contract and redacted diagnostics.
- [x] Do not refresh benchmark data; record N/A and #560 ownership in review.
- [x] Run normal/race package tests, shared dependency tests, static gates, and
  `make ci`.

## Task 4: Review And Publication

- [x] Record a local six-perspective 7-Tier review (native review lanes are
  unavailable in this session) and require P0=0/P1=0.
- [x] Add a focused lesson about compatibility boundaries for shared helpers.
- [ ] Commit with Lore trailers, create a PR closing #588, verify CI, then
  rebase-merge, sync `develop`, and clean the worktree after approved merge.

## Rollback

Revert the migration commit. The shared substrate and `lock/redis` migration
remain independently usable; no key, payload, token, or schema migration has
occurred.
