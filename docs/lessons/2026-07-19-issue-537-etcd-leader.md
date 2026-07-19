# Lessons Learned - etcd Leader Election (#537)

This note records the reusable implementation and verification lessons from the
etcd-backed `leader.Elector` provider delivered for milestone `v0.19.0`.

## L1: Official `Campaign` cleanup is bounded by the client context

### Problem

Canceling the caller context does not guarantee that
`concurrency.Election.Campaign` has returned. The official implementation may
still be removing its candidate through a transaction issued on the shared etcd
client context. A supervisor that only cancels the campaign can therefore leak
a blocked goroutine while believing shutdown is complete.

### Decision

Treat the caller context as the admission and waiting boundary, and the
caller-owned etcd client as the final coordinated hard-stop boundary. Normal
shutdown cancels campaigns, joins protected work, and attempts bounded
`Resign`. If an official campaign still does not return, the supervisor closes
the shared client, joins every campaign, retains unresolved cleanup inventory,
and uses a separate healthy client to prove the exact candidate range empty
before restart.

### Evidence

`TestBlockedOfficialCampaignCleanupRequiresClientHardStop` intercepts the
cleanup transaction, proves cancellation alone does not join the campaign,
then proves client close releases it. The provider preserves
`ErrCommitUnknown` and the generation inventory until a separate linearizable
read proves the exact candidate key absent. The shutdown example in
`leader/etcd/example_test.go` demonstrates the same ordering.

### Future Guard

Do not replace the client hard-stop and exact-range reconciliation sequence with
a caller-context timeout. Any etcd client upgrade must rerun the hostile cleanup
test because the upstream `Campaign` cleanup context is an operational
dependency even though it is not part of the provider's public API.

## L2: The server-granted TTL is the cleanup budget authority

### Problem

The requested lease duration is not proof of the lease duration etcd granted.
Using the request as an expiry or reconciliation authority can make cleanup too
early or too late, and it still cannot prove that a candidate key disappeared.

### Decision

Round the requested duration to integer seconds for `Grant`, validate the
server response, and publish the granted TTL through `EffectiveTTL`. Use that
value only to schedule reconciliation and operational deadlines. Clear cleanup
state only after successful revoke or a linearizable exact-generation absence
proof; elapsed TTL is never sufficient proof by itself.

### Evidence

`TestEffectiveTTLTransitionsAndRejectsInvalidGrant` verifies requested,
unpublished, active, and last-published transitions. Campaign tests reject
invalid grants and use the granted TTL for operation budgets. The public package
README and backend-neutral `leader.Elector` documentation explicitly preserve
the distinction between a timer and cleanup proof.

### Future Guard

Do not infer expiry from `leader.Options.Lease`, local wall-clock time, or one
missed keepalive. Future providers that expose an effective TTL should document
whether it is requested or server-authoritative and must keep proof of ownership
loss separate from scheduling.

## L3: Session, exact-key watch, and `Proclaim` prove different facts

### Problem

An etcd session staying alive only proves lease keepalive health. It does not
prove that the exact election candidate key still exists or still represents
the same generation. Likewise, winning `Campaign` does not prove that the
provider-specific token has been durably published for observers.

### Decision

Use the official `Session` for lease maintenance, an exact candidate-key watch
from the campaign header revision for generation loss, and a bounded
`Proclaim` to publish the provider token before exposing leadership. Leadership
is visible only after the watch is created and publication succeeds. Generation
identity includes key, lease, and creation revision so ABA replacement is not
mistaken for continued ownership.

### Evidence

`TestCampaignPublishesOnlyAfterWatchCreated` locks the publication boundary,
`TestCampaignBoundsProclaim` locks the bounded token update, monitor tests cover
watch cancellation and compaction, and
`TestCleanupReconciliationRejectsABA` proves creation-revision changes are
treated as replacement even when identity text is reused.

### Future Guard

Do not collapse session health, exact ownership, and observer publication into
one boolean or one RPC result. Changes to election observation must preserve the
three independent signals and the generation tuple.

## L4: Provider timing profiles must preserve cases and contain timed-out work

### Problem

A shared conformance suite can become either unrealistically slow for a real
server or unsafe when a timed-out case leaves a provider goroutine behind.
Increasing timeouts hides containment bugs; shortening them without an abort
path creates cross-case interference.

### Decision

Keep all 15 mandatory `leadertest` cases and supply an etcd-specific timing
profile: a 3-second lease, 1-second renewal interval, 12-second case timeout,
4-second wait timeout, and 2-second resign timeout. Give every case a dedicated
client and use client close as the bounded abort callback so timed-out work is
joined before the next case starts.

### Evidence

`TestEtcdElectorConformance` configures the profile and per-case client
registry. The full provider test completed in 28.24 seconds, and the race run of
`leader/leadertest` plus `leader/etcd` completed in 43.48 seconds. All 15 cases
passed, including lost response, renewal failure, owner loss, stale resign,
exact contention, and redaction.

### Future Guard

Do not delete slow cases, share a faulted client across cases, or raise the
global timing defaults to accommodate one backend. Tune the provider profile,
retain abort containment, and report normal plus race wall-clock evidence when
the profile changes.
