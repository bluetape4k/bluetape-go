# Issue #537 etcd Leader Elector Design

Issue: [#537](https://github.com/bluetape4k/bluetape-go/issues/537)

Parent research: [#496](https://github.com/bluetape4k/bluetape-go/issues/496)

Dependency: [#527](https://github.com/bluetape4k/bluetape-go/issues/527)

Date: 2026-07-19

Branch: `feat/issue-537-etcd-leader`

## Problem

`leader`는 backend-neutral `leader.Elector`와 Redis, MongoDB, PostgreSQL provider를
제공하지만 etcd cluster를 authoritative coordination store로 사용하는 caller가 공식
etcd election/session lifecycle을 재사용할 provider는 없다. #496 research gate는
provider conformance가 먼저 고정된 뒤 `leader/etcd`를 추가하고, caller-owned client와
backend-native semantics를 유지하도록 결정했다.

이번 provider는 #527의 mandatory single-elector contract를 모두 통과해야 한다. 다만
현재 `leader/leadertest`가 고정한 300ms lease는 정수 초 TTL을 사용하는 etcd session에
표현할 수 없다. Contract case를 끄거나 의미를 약화하지 않고 동일 case에 provider timing
profile을 적용할 수 있도록 conformance harness를 source-compatible하게 확장해야 한다.

## Requirements and Scope

- `leader/etcd`에서 `leader.Elector` single-leader contract를 구현한다.
- 공개 constructor는 caller-owned, concurrency-safe `*clientv3.Client`를 받는다.
- provider는 caller client를 닫거나 endpoint, credential, TLS, retry policy를 변경하지 않는다.
- elector는 campaign generation마다 독립적인 `concurrency.Session`과
  `concurrency.Election`을 만들고 그 session lifecycle을 소유한다.
- acquisition, contention wait, session keepalive, ownership observation, resign, cancellation,
  cleanup-pending 및 lease-expiry fallback 의미를 보존한다.
- official `concurrency.Election`의 cancel cleanup 한계를 숨기지 않고 문서화한다.
- #527의 15개 mandatory leader conformance case를 capability skip 없이 모두 실행한다.
- official Testcontainers etcd module과 실제 etcd server를 사용해 integration을 증명한다.
- consistency, RBAC, TLS, quorum, compaction, lease granularity, shutdown 및 fencing 한계를
  영어/한국어 README에 동일하게 문서화한다.
- Consul, Kubernetes Lease, DynamoDB, Hazelcast, ZooKeeper, multi-leader group/strategic
  election, cluster provisioning 및 generic coordination abstraction은 이번 범위에서 제외한다.

## Current Evidence

| Source | Evidence |
|---|---|
| `leader/elector.go`, `leader/errors.go` | Blocking campaign, local state sentinels, cleanup-pending, commit-unknown and redacted provider-error contract. |
| `leader/leadertest` | Mandatory acquire, contention, renewal, owner-loss, expiry, stale-resign, fault and redaction contract. |
| `leader/sql/lifecycle.go` | Generation-guarded ownership state, exact worker join and retryable cleanup inventory. |
| [etcd `Session`](https://github.com/etcd-io/etcd/blob/v3.6.13/client/v3/concurrency/session.go) | Session grants or adopts one integer-second lease, drains keepalive responses and closes `Done` when refresh stops. |
| [etcd `Election`](https://github.com/etcd-io/etcd/blob/v3.6.13/client/v3/concurrency/election.go) | Lease-bound candidate key, revision-ordered campaign, compare-revision proclaim/resign and oldest-candidate observation. |
| [etcd lease API](https://etcd.io/docs/v3.6/learning/api/#lease-api) | Lease expiry or revoke deletes every attached key. |
| [etcd support policy](https://etcd.io/docs/v3.7/op-guide/versioning/) | Current and previous minor release branches are maintained. |
| [Testcontainers etcd](https://golang.testcontainers.org/modules/etcd/) | `etcd.Run` and `ClientEndpoint(s)` provide a maintained real-server fixture. |
| `bluetape4k-leader` issue #227 lesson/design | Caller-owned client, real-server tests, cancellation cleanup and TTL fallback are reusable ecosystem constraints, not APIs to port mechanically. |

Direct source inspection confirms that etcd v3.6.13 and v3.7.0 have the same relevant
session/election cancellation behavior. `Election.Campaign` waits synchronously. When its caller
context ends while waiting, it calls `Resign(client.Ctx())`; the caller deadline does not bound
that cleanup RPC because `client.Ctx()` belongs to the long-lived client. Running Campaign in a
detached wrapper goroutine would return sooner but leave state mutation and cleanup racing after
the method returns, so this design explicitly rejects that workaround.

## Alternatives

| Approach | Decision | Reason |
|---|---|---|
| Synchronous `concurrency.Session` plus `concurrency.Election` | Selected | Uses the official queue, revision and lease lifecycle, preserves caller-owned client ownership, and creates no detached per-method goroutine. |
| Reimplement election with raw KV/Txn/Watch/Lease | Rejected | It could use fully bounded cleanup contexts but duplicates subtle queue, revision, watch-compaction and stale-candidate logic that the official concurrency package already owns. |
| Create an elector-owned etcd client | Rejected | Closing that client can interrupt cleanup but duplicates connection pools/configuration and violates the caller-owned client boundary. |
| Accept a caller-owned `*concurrency.Session` | Rejected | Shared or externally closed sessions make per-elector ownership, resign, revoke and retry semantics ambiguous; one generation must own one session. |
| Return on `ctx.Done()` while `Election.Campaign` runs in a goroutine | Rejected | Leaves a method-surviving goroutine and permits late acquisition or cleanup races after the caller received cancellation. |

## Selected Package and Public API

Add import path `leader/etcd` with package name `etcdleader`:

```go
func New(client *clientv3.Client, opts leader.Options) (*Elector, error)
```

`New` validates a non-nil client and normalized `leader.Options`, requires
`RenewInterval < Lease`, creates a cryptographically random owner token, computes the election
prefix and effective integer-second TTL, and performs no backend I/O. It never creates a session
until `Campaign` starts and never closes the caller's client.

The public surface intentionally accepts no raw `concurrency.SessionOption`. Allowing callers to
override `WithTTL`, `WithLease` or `WithContext` could silently disconnect the `leader.Options`
contract from the provider's cleanup inventory. Callers own client configuration; the elector
owns the session derived from the normalized leader configuration. A future session adoption or
restart-resume API requires a separate design with explicit ownership transfer.

The owner value is `<MemberID>:<128-bit random hex>`, matching existing provider semantics.
`MemberID` supports operations while the random suffix makes separate elector instances distinct.
The value is an identity token, not a secret, credential, authorization grant or fencing token.

## Key and Session Model

The election prefix is the provider-common normalized identity:

```text
<KeyPrefix>:<Group>
```

`concurrency.Election` stores candidates below `<prefix>/` using the session lease ID in the key.
The oldest creation revision is the current leader. The candidate value is the elector's owner
token, and every candidate key is attached to exactly one elector-owned session lease.

Each campaign generation creates:

- one internal session context independent of the caller's campaign context;
- one `concurrency.Session` with `WithContext` and the effective TTL;
- one `concurrency.Election` for the fixed prefix;
- one generation record containing lease ID, candidate key, creation revision, token, cancel and
  completion handles;
- after acquisition, one event-driven ownership monitor.

The requested positive `Lease` is rounded up to the next whole second because etcd lease grants
use integer seconds. The rounded value is the **effective TTL** used for session creation, cleanup
budgets, expiry fallback and documentation. The provider never rounds down. A sub-second lease
therefore has a one-second effective TTL. Constructor tests cover exact seconds, fractional
round-up, overflow and the `RenewInterval < Lease` rule.

`RenewInterval` remains a provider-neutral safety intent and conformance timing input, but etcd's
session owns the actual keepalive cadence, normally derived from TTL. The provider does not add a
second polling or `KeepAliveOnce` loop and does not claim exact `RenewInterval` scheduling.

## Campaign Lifecycle

`Campaign(ctx)` performs the following serialized state transition:

1. Reject nil or already-ended contexts before dispatch.
2. Under the state mutex, reject `ErrAlreadyLeader`, `ErrCampaignInProgress` or
   `ErrCleanupPending`; otherwise mark campaign in progress and allocate a generation.
3. Create the generation session using the internal session context and effective TTL.
4. Create `concurrency.Election` and call `Election.Campaign(ctx, token)` synchronously.
5. On apparent success, call `Election.Proclaim(ctx, token)` as a compare-revision ownership
   validation. A missing/replaced key fails closed instead of returning stale success.
6. Snapshot `Election.Key()` and `Election.Rev()`, mark the generation owned, and start the
   event-driven ownership monitor.
7. Return success only after the owned state and monitor handles are published atomically.

The internal session context is not derived from the campaign context. A successful campaign may
therefore keep leadership after the method returns, while caller cancellation remains scoped to
that acquisition call.

If Campaign or Proclaim fails after backend dispatch, ownership can be indeterminate. The elector
retains the session/key/revision generation as cleanup inventory, stops session keepalive, attempts
a bounded lease revoke, and returns a redacted `leader.OperationError`. If successful ownership or
cleanup cannot be proved, the result also matches `leader.ErrCommitUnknown` and a following
Campaign returns `leader.ErrCleanupPending` until the same elector completes `Resign` or the full
effective TTL has elapsed.

### Cancellation limitation

The provider calls official `Election.Campaign` synchronously and creates no wrapper goroutine.
On healthy connectivity, caller cancellation stops the watch, removes the candidate and returns
the context error. During a partition, the official implementation's synchronous
`Resign(client.Ctx())` can extend method latency beyond the caller deadline. The provider cannot
interrupt that RPC without closing the shared caller-owned client.

The operational contract is therefore:

- cancellation initiates local and remote cleanup;
- successful resign or revoke removes the candidate immediately;
- unresolved remote cleanup is bounded for safety by stopped keepalive plus effective TTL expiry;
- strict wall-clock return by the caller deadline is not guaranteed during an etcd/network
  partition;
- applications must not start protected work after a campaign context has ended, even if a late
  successful return is observed.

This is an explicit etcd primitive limitation, not a promise weakened through a hidden goroutine.

## Ownership Monitoring and Renewal

After acquisition, one provider-owned monitor observes two event sources without polling:

- `Session.Done()`, which closes when the lease keepalive stream ends or the session is canceled;
- a watch of the exact candidate key beginning after its known acquisition revision.

Lease loss, key deletion, watch compaction/error or an ownership-invalidating event clears
`IsLeader` for the matching generation and stops that generation. A generation check prevents a
stale monitor from clearing newer state. A watch error fails closed because the provider can no
longer prove ownership.

The monitor does not periodically call `Leader`, `TimeToLive`, `Proclaim` or any other backend
operation. Keepalive scheduling and retries remain inside the official session. In production,
renewal health is therefore observed as session keepalive continuity rather than a
provider-generated renewal ticker.

## Resign and Cleanup Lifecycle

`Resign(ctx)` is idempotent and serializes with Campaign and monitor transitions.

1. Reject nil or already-ended contexts before dispatch.
2. If there is no owned or cleanup generation, return nil.
3. Mark local leadership false and prevent a new Campaign.
4. Cancel and join the exact generation monitor so no stale monitor mutates later state.
5. Reconstruct the election with the retained session, key and revision using
   `concurrency.ResumeElection` when necessary, then call its compare-revision `Resign(ctx)`.
6. On confirmed delete or confirmed absence, cancel the session parent context, wait boundedly for
   `Session.Done`, and revoke the lease with the remaining caller budget.
7. Clear generation state only after cleanup is confirmed. Repeated Resign then returns nil.

The key/revision snapshot is retained independently of `Election`'s mutable local fields because
the official `Resign` clears its fields even when the RPC returns an error. A failed dispatched
resign returns a redacted operation error matching `leader.ErrCommitUnknown`, preserves cleanup
inventory and permits a fresh-context retry through `ResumeElection`. The creation-revision
compare prevents a stale retry from deleting a replacement owner.

If the caller context ends while monitor join, delete or revoke is incomplete, `Resign` preserves
cleanup state. Canceling the session context stops keepalive even when immediate revoke cannot be
confirmed; effective TTL expiry is the final safety boundary.

## Leader Observation

`Leader(ctx)` validates the context and performs one sorted prefix `Get` with
`clientv3.WithFirstCreate()`. It does not create a session merely to observe leadership.

- no candidate returns `"", nil`;
- the oldest candidate returns its token;
- a read failure returns a redacted `leader.OperationError("etcd", "leader", cause)`;
- endpoints, keys, groups, lease IDs, tokens and raw provider messages never appear in the rendered
  error string.

Observation is a linearizable etcd read unless the caller deliberately configured the shared
client otherwise. `Leader` reports the backend's current candidate and does not prove this local
elector still owns leadership. `IsLeader` remains the local fail-closed lifecycle signal.

## Error Policy

Every dispatched provider failure is wrapped with
`leader.NewOperationError("etcd", operation, cause)`. Stable operation labels are `campaign`,
`proclaim`, `resign`, `revoke`, `watch` and `leader`.

- pre-dispatch nil context returns `leader.ErrInvalidContext`;
- pre-dispatch canceled/deadline context returns the bare context error;
- campaign/proclaim failure after possible mutation matches `leader.ErrCommitUnknown` unless
  reconciliation or revoke proves no live ownership;
- resign/revoke failure after possible mutation matches `leader.ErrCommitUnknown` and retains
  cleanup state;
- session/watch loss makes `IsLeader=false`; unresolved remote ownership retains cleanup state;
- rendered errors preserve error type and `errors.Is`/`errors.As` without rendering raw etcd text.

No automatic Campaign retry occurs after an indeterminate mutation. Callers retry bounded Resign
on the same elector and fall back to full effective-TTL wait.

## Conformance Timing Amendment

Extend the public test-only harness source-compatibly:

```go
type Timing struct {
    Lease         time.Duration
    RenewInterval time.Duration
    CaseTimeout   time.Duration
    WaitTimeout   time.Duration
    ResignTimeout time.Duration
}

type Harness struct {
    New     Factory
    Control Control
    Timing  Timing
}
```

The zero-value `Timing` resolves field-by-field to the current defaults:

- lease: 300ms;
- renew interval: 50ms;
- case timeout: 5s;
- wait timeout: 2s;
- resign timeout: 250ms.

Normalization rejects negative values and any resolved `RenewInterval >= Lease`. `Run` still
executes the same named 15-case table in the same order. There are no capability flags, case
filters, skips or relaxed assertions. Existing harness literals compile unchanged and keep their
current timing.

The etcd integration harness uses:

- lease: 3s;
- renew interval: 1s;
- case timeout: 12s;
- wait timeout: 4s;
- resign timeout: 2s.

This profile represents integer-second TTL and official keepalive cadence while preserving every
contract assertion. Harness self-tests prove zero-value compatibility, partial defaulting, invalid
timing rejection and that a deliberately broken provider still fails under a custom profile.

## Conformance Adapter and Fault Injection

The etcd conformance adapter uses the same real server and namespace as constructed electors.

- `ReplaceOwner` revokes the observed current leader lease, creates a fresh control lease/session,
  and campaigns a replacement token. Revocation ensures the original `Session.Done` signal and
  candidate-key deletion are both real backend events.
- `Owner` reads the first-created candidate value.
- `OperationCount` is identity/operation-specific, monotonic and concurrency-safe.
- Campaign and resign use a package-private phase hook for deterministic post-linearization
  response loss, following the SQL provider precedent.
- A dedicated test client stream interceptor counts successful LeaseKeepAlive responses as
  `OperationRenew` and can terminate the armed next keepalive stream after a successful response.
  This proves session loss, local fail-closed transition and stopped renewal traffic without a
  production polling hook.

Test-only hooks never become public API and never bypass actual etcd mutation. The lost-response
cases first perform the real operation, then replace only its observed response.

## Dependencies and Fixture

Add production dependency:

```text
go.etcd.io/etcd/client/v3 v3.6.13
```

Add test dependency:

```text
github.com/testcontainers/testcontainers-go/modules/etcd v0.42.0
```

Use `gcr.io/etcd-development/etcd:v3.6.13` in the integration fixture. v3.6.13 is the maintained
previous minor, has a smaller transitive upgrade surface than the newly released v3.7.0, and has
the same relevant Session/Election behavior. v3.5 is outside the normal current-plus-previous
support window. A future v3.7 upgrade is separate dependency work.

The fixture uses `etcd.Run` directly; no public `testcontainers/etcd` wrapper is added. After the
container reports its endpoint, the test performs a bounded real `Status` or KV request before
declaring readiness. Partially initialized containers and clients are cleaned up with bounded
`internal/testcleanup` handling. Docker-backed tests run serially.

## Operational and Security Contract

- All contenders for one group use clients that reach the same etcd cluster and election prefix.
- Production uses an odd-sized quorum, TLS endpoint verification and authenticated clients.
- The runtime role needs lease grant/keepalive/revoke and read/write/watch access only to its
  configured election prefix. `KeyPrefix` is a namespace, not a tenant authorization boundary.
- Direct writes or deletes under the election prefix can force leadership loss and are restricted
  to trusted operators/roles.
- etcd linearizable reads and quorum availability define coordination availability. A minority
  partition cannot safely acquire new leadership.
- The API supplies no fencing token. Protected-resource safety requires an external fencing or
  generation check when stale leaders can continue work after losing etcd connectivity.
- Watch compaction or stream failure clears local leadership rather than guessing ownership.
- Shutdown order is: stop protected work, cancel/join campaign, bounded Resign every elector,
  inventory cleanup-pending generations, wait effective TTL where required, then close the
  caller-owned client after all other users finish.
- Telemetry records only safe provider/operation/result labels. It never records full endpoints,
  keys, lease IDs or owner tokens.

## Test Strategy

### Unit and deterministic lifecycle tests

- nil client, option validation, integer-second TTL conversion and token uniqueness;
- no backend I/O or session creation in `New`;
- duplicate, in-progress and cleanup-pending state rejection;
- generation-safe monitor and resign races;
- session loss, key deletion, watch error and stale monitor fail-closed behavior;
- resign retry through retained key/revision and stale-resign safety;
- context and provider-error mapping, commit-unknown composition and redaction;
- caller client remains usable after elector cleanup.

### Real etcd integration

- acquire/observe and exact one-winner contention;
- canceled waiter leaves no late candidate or keepalive stream;
- session keepalive preserves ownership beyond requested lease;
- lease revoke/stream failure clears local leadership and permits takeover;
- external key loss and watch interruption fail closed;
- resign, idempotent resign, lost response, retry and stale revision protection;
- server restart/endpoint interruption within a bounded fixture where stable;
- complete `leadertest.Run` under the etcd timing profile;
- repeated and race-enabled package runs, serialized against one fixture.

### Repository verification

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -p 1 -count=10 ./leader/etcd
go mod verify
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Dependency verification records the selected etcd/Testcontainers versions and the resulting
gRPC, protobuf, Prometheus, `x/net`, `x/sys` and zap module graph.

## Documentation and Release Surface

- Add `leader/etcd/README.md` and `README.ko.md` with section-for-section parity.
- Register the provider in `leader/README.md`, `leader/README.ko.md`, root `README.md` and
  `README.ko.md`.
- Update the unreleased `CHANGELOG.md` and the v0.19.0 provider-conformance runbook.
- Include compile-checked examples for acquire/work/resign and cancellation/TTL recovery.
- Record a Type A lesson covering the official Campaign cleanup context, effective TTL,
  event-driven ownership monitoring and non-skipping conformance timing profile.
- Do not add a cluster-provisioning package, generic etcd wrapper or public Testcontainers helper.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Official Campaign cleanup exceeds caller deadline during partition | Call synchronously, document the limitation, stop session keepalive after return, attempt bounded revoke and use effective TTL fallback; never detach a wrapper goroutine. |
| Session expires while Campaign is completing | Validate ownership with Proclaim and publish owned state only after key/revision snapshot; monitor session and key immediately. |
| Session remains healthy after candidate key deletion | Watch the exact candidate key as well as `Session.Done`; fail closed on deletion or watch error. |
| Failed Resign clears official Election local fields | Retain key/revision/session independently and retry with `ResumeElection`. |
| Sub-second shared conformance cannot map to etcd TTL | Add timing-only Harness configuration; keep every case and assertion mandatory. |
| Opaque keepalive hides renewal operations | Observe/fault the real gRPC stream only in the conformance client; production stays event-driven. |
| v3.7 dependency causes broad transitive churn | Pin maintained v3.6.13 and verify the module graph; upgrade separately. |
| Error messages leak endpoint/key/token data | Use redacted `leader.OperationError` labels and explicit forbidden-marker tests. |
| Stale leader continues protected work | Document absence of fencing and require application-level fencing for safety-critical resources. |

## Acceptance Criteria

1. `leader/etcd` implements `leader.Elector` over a caller-owned etcd client without closing it.
2. Each campaign generation owns one official Session/Election lifecycle and creates no detached
   per-method goroutine.
3. Acquire, observation, session renewal, owner loss, cancellation, resign retry, stale cleanup and
   effective-TTL fallback semantics are covered on a real etcd server.
4. `leadertest.Harness` gains a source-compatible timing profile with zero-value compatibility,
   no capability flags and no skipped/relaxed cases.
5. The etcd provider passes all mandatory conformance cases with the 3s/1s profile.
6. Operation failures are typed, redacted and commit-unknown/cleanup-pending recovery remains
   discoverable through `errors.Is`/`errors.As`.
7. README pairs, package registration, changelog, release runbook and Type A lesson document
   caller ownership, effective TTL, cancellation limitation, quorum/RBAC/TLS, no-fencing and
   shutdown requirements.
8. Targeted, race, repeated, dependency and repository CI verification pass with Docker-backed
   packages serialized.
