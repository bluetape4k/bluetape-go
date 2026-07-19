# leader/etcd

[English](README.md) | [한국어](README.ko.md)

`leader/etcd` implements the single-leader `leader.Elector` contract with the
official etcd v3 `Session` and `Election` primitives. It is designed for a
caller-owned, authenticated, TLS-verified client connected to one production
quorum.

## Preflight

Before starting contenders, verify every endpoint reports the expected member
and leader, then perform a bounded linearizable Put/Get/Delete roundtrip. Use an
odd-sized production quorum, confirm a writable majority, and pin the RBAC
range described below. The real-server suite targets etcd `v3.6.13`; other
minor versions require a separate compatibility run.

The repository Testcontainers fixture is plaintext and test-only. It pins an
immutable platform digest and must not be copied into production configuration.

## Import And Client Ownership

```go
import etcdleader "github.com/bluetape4k/bluetape-go/leader/etcd"
```

Construct every elector with `etcdleader.New`. The zero value of `Elector` is
unusable. `New` performs no network I/O and never takes ownership of the
`*clientv3.Client`; the caller closes that shared client only after all users
and campaign calls have joined. The coordinated hard-stop exception is a
blocked Campaign: first drain every other shared-client user, close the client
to unblock that call, then bounded-join it as shown below.

Production clients load a non-empty root CA pool, set a hostname-validating
`ServerName`, keep `InsecureSkipVerify=false`, and authenticate with a scoped
username/password or an equivalently scoped client certificate. See
`ExampleNew_productionTLS` for the compile-checked client shape.

## Usage

```go
elector, err := etcdleader.New(client, leader.Options{
    Group:         "billing-workers",
    MemberID:      "worker-1",
    Lease:         30 * time.Second,
    RenewInterval: 10 * time.Second,
})
if err != nil {
    return err
}
```

`Campaign` is synchronous. Even after it returns nil, inspect
`campaignCtx.Err()` and require `IsLeader()` immediately before starting
protected work. While work runs, sample `IsLeader` before every work unit and
at least every `min(RenewInterval, 1s)` during a long unit. Stop and join
protected work before reacquiring. The complete acquire/work/resign sequence is
in `ExampleNew`.

`New` accepts only the caller-owned `*clientv3.Client` and `leader.Options`,
then normalizes and validates those options. It intentionally exposes no raw
`concurrency.SessionOption`, session adoption, or restart-resume API. Every
Campaign creates a new provider-owned session. `Leader` returns an opaque
`<MemberID>:<random>` value; compare it as a whole string and never parse the
suffix as a fencing token or durable identity.

## Encoded Election Range

The provider encodes `KeyPrefix` and `Group` independently with unpadded URL
Base64 beneath `/bluetape4k/leader`. Candidates occupy the exact
`[candidateRoot, rangeEnd)` interval for that identity. Encoding prevents
delimiter overlap between sibling groups, but `KeyPrefix` is collision
isolation, not hostile-tenant isolation. The format is Go-owned and does not
interoperate with Kotlin/JVM leader participants.

Direct Put/Delete access inside a candidate range can force leadership loss and
belongs only to mutually trusted operators and election principals.

## Lease And Ownership Signals

Requested leases are rounded up to integer seconds. etcd may return a different
server-granted TTL; `EffectiveTTL` exposes the current or most recent fully
published grant and is the only TTL suitable for retry scheduling.
`RenewInterval` must be at least 100 milliseconds and less than `Lease`, so
periodic `Proclaim` cadence is no faster than 10 Hz per published leader. Plan
aggregate capacity as published leader groups multiplied by their configured
`Proclaim` rate.

The capacity inventory must also bound live contenders across all groups. Each
contender consumes one of the expected leases/sessions/candidate keys, while
each published group adds one of the expected exact-key watches. Calculate
aggregate Proclaim QPS as the sum of `1 / RenewInterval` over published groups,
then validate that full contender and publication envelope against the target
etcd cluster.

Ownership uses three separate signals:

- the official `Session` owns lease keepalive and closes on lease loss;
- a bounded `Proclaim` at `RenewInterval` verifies the candidate revision and
  stops overlapping renewal work;
- an exact-key watch detects deletion, replacement, compaction, or stream loss.

Any signal failure clears local `IsLeader` immediately. Local state is an
execution guard, not a fencing token or remote-deletion proof.

## Failure Recovery

Inspect `leader.OperationError`, `leader.ErrCommitUnknown`, and
`leader.ErrCleanupPending` with `errors.Is` and `errors.As`. Diagnostic strings
redact keys, endpoints, lease IDs, and owner tokens. The error chain preserves
the raw etcd cause for explicit `errors.Is`/`errors.As` inspection; do not emit
unwrapped or joined raw causes to logs or telemetry without sanitizing them.

Cancellation can enter the official `Election.Campaign` cleanup using the
long-lived caller client context. Retry bounded `Resign` on the same elector
while that client remains healthy. If cleanup remains unknown, retain the
elector and its inventory. Waiting `EffectiveTTL` only schedules another
linearizable exact-key reconciliation; elapsed time never proves deletion.
An unpublished in-progress `Campaign` owns its cleanup; concurrent `Resign` is
a no-op, so supervisors cancel and join campaign calls before resigning.

## Shutdown And Reconciliation

Use this order per logical group:

1. cancel campaign contexts and stop new protected work;
2. wait a bounded join grace and join protected work;
3. when calls joined, retry same-elector `Resign` while the client is healthy;
4. when a call remains blocked, coordinate every user of its case/service
   client, close that caller-owned client as the hard stop, and join the call;
5. persist every unresolved generation and terminate that process lane;
6. use a separate healthy diagnostic client to prove exact range absence or
   replacement before restart.

Do not close a shared client while unrelated users are active. Do not restart,
cut over, or clear cleanup inventory based only on a timer.

## RBAC And TLS

Grant each principal read/write permission only for its encoded
`[candidateRoot, rangeEnd)` interval. Tests against v3.6.13 prove symmetric
own-range Put/Get/Delete/Watch access and sibling-range denial. They also prove
that any user may revoke an unattached lease, but cannot revoke a lease once a
key outside that user's permitted range is attached, and cannot `KeepAliveOnce`
that attached lease. These are attached-key authorization checks, not
creator-owned or prefix-scoped lease capabilities.

Principals sharing a range remain mutually trusted. The pinned results narrow
cross-principal election-lease interference but do not establish general
hostile-tenant isolation; place mutually untrusted tenants in separate
clusters. Every server-version change must rerun both cross-principal revoke
and keepalive denial tests. TLS must validate both the issuing CA and endpoint
hostname.

## Quorum, Compaction, And Fencing

Coordination availability requires linearizable reads and a writable majority.
A minority partition must not campaign. Restore quorum, verify Status plus a
linearizable roundtrip, and reconcile unresolved candidates before restarting.

Watch compaction or stream interruption fails closed. The API provides no
fencing token, so a stale leader may continue external side effects after
losing connectivity. Safety-critical resources require a separate fencing or
generation check.

## Migration And Rollback

Cutover is stop-the-world per group: stop protected work, drain and prove the
old provider's boundary, then start etcd contenders. Rollback is symmetric:
stop protected work and every etcd campaign, perform bounded same-elector
cleanup, prove the exact candidate range empty with a healthy client, restore
the diagnostic view has zero etcd contenders, and only then restore the
previous provider. Any provider overlap requires an external fencing authority.

## Observability

Wrap synchronous operations with bounded-cardinality
provider/operation/result/latency metrics. Sample `IsLeader` before work and at
least every `min(RenewInterval, 1s)` during long work; emit one
`leadership_lost` event on the first true-to-false transition. Inventory
`ErrCommitUnknown` and `ErrCleanupPending` at call boundaries, and record every
failed cleanup attempt separately even when a later retry proves cleanup.
The example recorder is synchronous and non-blocking; sanitize its input before
emitting it or hand it off to a bounded internal queue.
Never label or log endpoints, keys, lease IDs, owner tokens, or rendered raw
errors.

## Tested Scope

The serial real-server suite covers all 15 `leader/leadertest` cases, 32-way
contention, cancellation, keepalive, external revoke, exact-key loss, watch
interruption, restart, post-success response loss, stale cleanup, authenticated
range isolation, hard-stop joining, and resource return to baseline. It does
not claim multi-node partition, TLS transport, or cross-minor compatibility.

## Test

Docker-backed tests must run serially:

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
```

Use the bilingual
[v0.19.0 provider rollout runbook](../../docs/release/v0.19.0-provider-conformance-runbook.md)
for deployment, reconciliation, cutover, rollback, and quorum-recovery gates.
