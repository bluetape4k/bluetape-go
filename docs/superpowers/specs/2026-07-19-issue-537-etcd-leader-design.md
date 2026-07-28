# Issue #537 etcd Leader Elector Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.


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

## 요구사항 and Scope

- `leader/etcd`에서 `leader.Elector` single-leader contract를 구현한다.
- 공개 constructor는 caller-owned, concurrency-safe `*clientv3.Client`를 받는다.
- provider는 caller client를 닫거나 endpoint, credential, TLS, retry policy를 변경하지 않는다.
- elector는 campaign generation마다 독립적인 `concurrency.Session`과
  `concurrency.Election`을 만들고 그 session lifecycle을 소유한다.
- acquisition, contention wait, session keepalive, ownership observation, resign, cancellation,
  cleanup-pending 및 lease-expiry 후 reconciliation 의미를 보존한다.
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
| `bluetape4k-leader` issue #227 lesson/design | Caller-owned client, real-server tests, cancellation cleanup and TTL-delayed reconciliation are reusable ecosystem constraints, not APIs to port mechanically. |

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
func (e *Elector) EffectiveTTL() time.Duration
```

`New` validates a non-nil client and normalized `leader.Options`, requires
`100ms <= RenewInterval < Lease`, creates a cryptographically random owner token, computes the encoded
election prefix and requested integer-second TTL, and performs no backend I/O. It never creates a
lease or session until `Campaign` starts and never closes the caller's client. A compile-time
assertion keeps `*Elector` assignable to `leader.Elector`; the zero-value `Elector` is not usable.

`EffectiveTTL` is a concrete-provider diagnostic because etcd may grant a different TTL from the
request. It returns the current published generation TTL; otherwise the last fully published TTL;
otherwise the requested rounded TTL. An in-progress grant is invisible until its positive,
duration-safe server TTL is atomically published. Each generation retains its own granted TTL.
`EffectiveTTL` is a retry/wait input, not proof that remote ownership has expired; cleanup state
clears only after revoke or linearizable reconciliation. It is not added to the backend-neutral
interface.

The public surface intentionally accepts no raw `concurrency.SessionOption`. Allowing callers to
override `WithTTL`, `WithLease` or `WithContext` could silently disconnect the `leader.Options`
contract from the provider's cleanup inventory. Callers own client configuration; the elector
owns the session derived from the normalized leader configuration. A future session adoption or
restart-resume API requires a separate design with explicit ownership transfer.

The owner value is `<MemberID>:<128-bit random hex>`, matching existing provider semantics.
`MemberID` supports operations while the random suffix makes separate elector instances distinct.
The value is an identity token, not a secret, credential, authorization grant or fencing token.

## Key and Session Model

Provider input is encoded into non-overlapping path segments:

```text
/bluetape4k/leader/<base64url(KeyPrefix)>/<base64url(Group)>
```

Both segments use unpadded `base64.RawURLEncoding`. The separator-free path above is
`electionBase` and is passed to `concurrency.NewElection`, which appends exactly one slash.
Raw operations use `candidateRoot := electionBase + "/"`; `ResumeElection` receives that
candidate root because, unlike `NewElection`, it does not append a slash. Every range operation
uses that exact candidate-root start and `clientv3.GetPrefixRangeEnd(start)`; raw prefixes,
delimiter concatenation and broad parent-prefix scans are forbidden. The oldest creation revision
is the current leader. The candidate value is the elector's owner token, and every candidate key
is attached to exactly one elector-owned lease. Tests cover slash-containing inputs, encoded
sibling groups and range-end isolation.

Each campaign generation creates:

- one internal generation context and cancel function;
- one explicit `Lease.Grant` result containing the server-granted TTL;
- one `concurrency.Session` with `WithContext` and `WithLease` for that grant;
- one `concurrency.Election` for the fixed prefix;
- one generation record containing lease ID, granted TTL, deterministic candidate key, optional
  creation revision, token, cancel and completion handles;
- after acquisition, one generation-owned ownership monitor.

The requested positive `Lease` is rounded up to the next whole second because etcd lease grants
use integer seconds. The provider never rounds down; a sub-second lease therefore requests one
second. `Campaign` calls `Lease.Grant` explicitly, records `LeaseGrantResponse.TTL`, and adopts that
lease with `WithLease`; retry budgets always use the granted value, not the request. Constructor
tests cover exact seconds, fractional round-up, overflow and the
`100ms <= RenewInterval < Lease` rule. The lower bound caps ownership-validation `Proclaim`
traffic at ten transactions per second per published leader. Grant tests cover a server TTL that
differs from the request.

The session owns lease keepalive cadence. Separately, the provider honors `RenewInterval` as an
ownership-validation cadence by calling `Election.Proclaim` with the same token. Proclaim's
compare-revision transaction proves that the candidate still exists and is still this generation;
it does not replace or duplicate lease keepalive.

## Campaign Lifecycle

`Campaign(ctx)` performs the following serialized state transition:

1. Reject nil or already-ended contexts before dispatch.
2. Under the state mutex, reject `ErrAlreadyLeader` or `ErrCampaignInProgress`. If cleanup is
   pending, snapshot it, release the lock and run bounded linearizable reconciliation; continue
   only when exact absence/replacement is proved, otherwise return `ErrCleanupPending`. Reacquire
   the lock, verify the snapshotted generation identity/state is unchanged, then atomically clear it
   and mark the new campaign in progress; a stale reconciliation result retries or loses to the
   current state error.
3. Explicitly grant the rounded TTL, record the server-granted TTL, and create the session using
   the generation context plus `WithLease`.
4. Create `concurrency.Election`. Build a campaign context canceled by caller completion,
   generation cancellation or `Session.Ctx()` completion. The cancellation bridges use joined
   `context.AfterFunc` callbacks; callbacks are stopped or observed before method return.
5. Call `Election.Campaign(campaignCtx, token)` synchronously. No wrapper goroutine is detached.
6. On apparent success, perform a bounded `Election.Proclaim` as compare-revision validation.
   A missing/replaced key fails closed instead of returning stale success.
7. Snapshot `Election.Key()`, `Election.Rev()` and `Election.Header().Revision`. Start an exact-key
   watch with `WithRev(proclaimRevision+1)` and `WithCreatedNotify`, then wait boundedly for the
   server-created response.
8. Serialize caller cancellation and ownership publication under the generation state lock. The
   callback cancels only an unpublished generation. Publication re-checks caller/session state and
   marks the generation published while holding the same lock; only then does it stop and join the
   callback. If cancellation wins, retain cleanup inventory. If publication wins, return success.

Caller cancellation cancels an unpublished generation context and therefore stops session
keepalive even if the official Campaign cleanup RPC remains blocked. After successful publication
the caller bridge is removed, so later cancellation of the acquisition context does not end
established leadership. Every campaign exit joins its local cancellation callbacks; monitor
goroutines are generation-owned and must be joined by cleanup.

If Campaign or Proclaim fails after backend dispatch, ownership can be indeterminate. The candidate
key is precomputed from `candidateRoot` and the granted lease ID, while creation revision remains
explicitly unknown until Campaign state or exact-key reconciliation supplies it. The elector
retains that inventory, immediately cancels the generation context, attempts a bounded lease
revoke, synchronously joins the local session, and returns a redacted `leader.OperationError`.
Generation shutdown is an idempotent `sync.Once` helper that cancels the context, calls
`Session.Orphan()` and observes closed `Session.Done()` before a terminal cleanup path returns or
state is cleared. `Session.Done` proves only local keepalive-goroutine termination, never the last
server keepalive or remote deletion. If revoke or linearizable exact-key absence/replacement cannot be
proved, the result also matches `leader.ErrCommitUnknown` and following Campaign calls return
`leader.ErrCleanupPending`; elapsed TTL alone never clears inventory.

### Cancellation limitation

The provider calls official `Election.Campaign` synchronously and creates no wrapper goroutine.
On healthy connectivity, caller cancellation stops the wait and attempts candidate removal.
During a partition, the official implementation's synchronous `Resign(client.Ctx())` can extend
method latency beyond the caller deadline. Canceling the generation stops lease keepalive and
bounds backend ownership by the granted TTL, but it cannot interrupt that cleanup RPC without
closing the shared caller-owned client.

The operational contract is therefore:

- cancellation attempts remote candidate removal and always initiates local keepalive shutdown;
- successful resign or revoke removes the candidate immediately;
- the server-granted TTL plus margin is an operational retry interval, not deletion proof;
- strict wall-clock return by the caller deadline is not guaranteed during an etcd/network
  partition;
- the production hard stop is: cancel all campaign contexts, wait a bounded grace period,
  coordinate every shared-client user, close the caller-owned client, join the calls, and retain
  the cleanup inventory until revoke or exact linearizable reconciliation proves absence;
- applications must not start protected work after a campaign context has ended, even if a late
  successful return is observed.

This is an explicit etcd primitive limitation, not a promise weakened through a hidden goroutine.

## Ownership Monitoring and Renewal

After acquisition, one provider-owned monitor observes three event sources:

- `Session.Done()`, which closes when the lease keepalive stream ends or the session is canceled;
- a watch of the exact candidate key beginning at the successful Proclaim header revision plus one;
- a `RenewInterval` ticker that performs one bounded compare-revision `Election.Proclaim`.

The watch's ready handshake is the bounded server-created notification requested by
`WithCreatedNotify`, not merely local goroutine startup. A PUT is ignored only when key, token,
create revision and lease all match the generation; any other PUT, delete, cancellation,
compaction or watch error invalidates ownership. Session loss or any Proclaim failure also
invalidates ownership. Every invalidation atomically clears `IsLeader`, marks cleanup pending when
absence is not proved, and immediately cancels the generation context so keepalive stops. A
generation check prevents a stale monitor from clearing newer state.

Each Proclaim uses a fresh context derived from the generation context and bounded by the smallest
positive value among `RenewInterval`, granted TTL divided by four, and one second. One monitor loop
permits at most one in-flight Proclaim; a slow call never overlaps or queues another. Watch and
Proclaim contexts cancel with Resign so the monitor joins promptly. The monitor holds no mutex
across a backend RPC, join or wait. The session remains the only lease keepalive owner; Proclaim is
the provider's public `renew` operation and deterministic conformance fault seam.

Every terminal monitor path invokes the same generation shutdown helper before closing its monitor
done channel. Session loss already has a closed `Session.Done`; PUT/delete/watch/renew loss cancels
and synchronously orphans the session. Concurrent Resign joins the single shared shutdown result
rather than starting a second orphan operation.

## Resign and Cleanup Lifecycle

`Resign(ctx)` is idempotent and serializes with Campaign and monitor transitions.

1. Reject nil or already-ended contexts before dispatch.
2. If there is no owned or cleanup generation, return nil.
3. Mark local leadership false, prevent a new Campaign, and immediately cancel the generation
   context so session keepalive stops.
4. Run or join the generation shutdown helper until `Session.Done()` is closed, then join the exact
   monitor so no stale monitor or local keepalive goroutine survives the method.
5. When creation revision is known, reconstruct the election with the retained session,
   candidate-root, key and revision using `concurrency.ResumeElection`, then call `Resign(ctx)`.
   When revision is unknown, reconcile the deterministic key first and resume only after obtaining
   the matching revision.
6. A nil official Resign result is not deletion proof because its compare may have failed. Attempt
   owned-lease revoke with the remaining budget, then perform a linearizable exact-key Get that
   compares key, create revision, token and lease where known.
7. Clear generation state only after the local session and monitor are joined and successful revoke
   or reconciliation proves exact absence or replacement. Repeated Resign then returns nil.

The key/revision snapshot is retained independently of `Election`'s mutable local fields because
the official `Resign` clears its fields on every transaction result. A failed dispatched resign or
nil result without follow-up proof returns a redacted operation error matching
`leader.ErrCommitUnknown`, preserves cleanup inventory and permits a fresh-context retry. The
creation-revision/token/lease comparison prevents a stale retry from deleting a replacement owner.

If the caller context ends while monitor join, delete, revoke or reconciliation is incomplete,
`Resign` preserves cleanup state. `Session.Done` and elapsed TTL are scheduling evidence only, not
last-renewal or deletion proof. Campaign and Resign entry reconcile exact key/revision/token/lease
absence and clear stale inventory only on a successful linearizable response; if etcd remains
unavailable, cleanup remains pending. Resign called while another Campaign is still waiting follows
the common contract and returns nil; callers cancel the Campaign context to stop that attempt.

## Leader Observation

`Leader(ctx)` validates the context and performs one linearizable sorted `Get` over the exact
encoded candidate range with `clientv3.WithFirstCreate()`. It does not create a session merely to
observe leadership.

- no candidate returns `"", nil`;
- the oldest candidate returns its token;
- a read failure returns a redacted `leader.OperationError("etcd", "lookup", cause)`;
- endpoints, keys, groups, lease IDs, tokens and raw provider messages never appear in the rendered
  error string.

The provider does not request serializable reads, so observation uses etcd's default linearizable
Get. `Leader` reports the backend's current candidate and does not prove this local elector still
owns leadership. `IsLeader` remains the local fail-closed lifecycle signal.

## Error Policy

Every returned synchronous provider failure is wrapped with
`leader.NewOperationError("etcd", operation, cause)`. Stable public operation labels remain
provider-neutral: `campaign`, `renew`, `resign` and `lookup`. Grant, session, watch, Proclaim,
revoke and reconciliation are private phases mapped to the owning public operation.

- pre-dispatch nil context returns `leader.ErrInvalidContext`;
- pre-dispatch canceled/deadline context returns the bare context error;
- synchronous campaign validation failure after possible mutation matches `leader.ErrCommitUnknown`
  unless reconciliation or revoke proves no live ownership;
- resign/revoke failure after possible mutation matches `leader.ErrCommitUnknown` and retains
  cleanup state;
- session/watch loss makes `IsLeader=false`; unresolved remote ownership retains cleanup state;
- rendered errors preserve error type and `errors.Is`/`errors.As` without rendering raw etcd text;
- provider-owned logs, examples and telemetry never unwrap causes. Tests verify `Error()` and every
  repository-owned logging path omit endpoint, username, password, token, encoded key/group, lease
  ID, certificate path and injected raw markers. Documentation warns that an explicitly unwrapped
  etcd cause is diagnostic-sensitive and must not be logged unsanitized.

No automatic Campaign retry occurs after an indeterminate mutation. Callers retry bounded Resign
on the same elector. An asynchronous monitor renew failure cannot be returned through
`leader.Elector`; it clears `IsLeader`, preserves cleanup state and exposes only safe lifecycle
signals. Waiting an effective TTL is only a backoff before another reconciliation attempt.

## Conformance Timing Amendment

Keep the public `Harness` type unchanged so existing keyed and unkeyed literals compile. Add a
source-compatible opt-in runner configuration:

```go
type Timing struct {
    Lease         time.Duration
    RenewInterval time.Duration
    CaseTimeout   time.Duration
    WaitTimeout   time.Duration
    ResignTimeout time.Duration
    _             struct{}
}

type AbortFunc func(context.Context, leader.Options) error

type Config struct {
    Timing Timing
    Abort  AbortFunc
    _      struct{}
}

func Run(t *testing.T, harness Harness)
func RunWithConfig(t *testing.T, harness Harness, config Config)
```

The blank unexported fields prevent external unkeyed literals for the new structs while preserving
zero-value and keyed-literal use, so future fields do not repeat the `Harness` compatibility trap.

The zero-value `Timing` resolves field-by-field to the current defaults:

- lease: 300ms;
- renew interval: 50ms;
- case timeout: 5s;
- wait timeout: 2s;
- resign timeout: 250ms.

Normalization rejects negative values, any resolved `RenewInterval >= Lease`, non-positive
timeouts, and profiles that fail both precise containment inequalities:

```text
joinGrace   = min(ResignTimeout, CaseTimeout/10)
abortBudget = min(ResignTimeout, 1s)
WaitTimeout   + joinGrace + abortBudget < CaseTimeout
ResignTimeout + joinGrace + abortBudget < CaseTimeout
```

`Run` is a wrapper around `RunWithConfig` using current defaults. Both execute the same named
15-case table in the same order; there are no capability flags, filters, skips or relaxed
assertions.

Every case owns a cancelable root context. Campaign, observation, control, waits, and workers derive
from it; bounded cleanup uses a fresh context so root cancellation cannot skip provider release. On
case timeout the runner cancels the root, waits a bounded join grace, invokes `Abort` with a fresh
`abortBudget` context whether the evaluator joined or still needs a hard stop, and joins the case
goroutine before the subtest returns. A provider adapter that can hit official unbounded cleanup
must supply case-dedicated clients and an Abort function that closes only those clients. This
prevents timed-out cases from mutating later tests or leaking sessions.

Nil Abort is valid for existing bounded providers. If cancellation does not join and Abort is nil,
or Abort fails to unblock the case, the harness records a containment-contract violation and does
not return from that subtest; the outer `go test -timeout` terminates the process rather than
allowing a leaked case to race later tests. When Abort returns an error but the case joins, the
failure reports both the original timeout and abort error. Subprocess self-tests cover these
fail-stop paths.

The etcd integration harness uses:

- lease: 3s;
- renew interval: 1s;
- case timeout: 12s;
- wait timeout: 4s;
- resign timeout: 2s.

This profile represents integer-second TTL and provider ownership validation while preserving
every contract assertion. Harness self-tests prove zero-value compatibility, unkeyed-literal
compatibility, partial defaulting, invalid relationship rejection, timeout cancel/abort/join
ordering and that a deliberately broken provider still fails under a custom profile. `waitFor`
uses events where the case exposes them, otherwise a 10-20ms ticker/backoff rather than a 1ms poll.

## Conformance Adapter and Fault Injection

The etcd conformance adapter uses the same real server and namespace as constructed electors.

- `ReplaceOwner` revokes the observed current leader lease, creates a fresh control lease/session,
  campaigns a replacement token, and immediately calls `Session.Orphan`. The replacement lease ID
  is retained for bounded suite teardown but is not kept alive; this preserves the expiry-takeover
  case instead of installing a permanent control owner.
- `Owner` reads the first-created candidate value.
- `OperationCount` is identity/operation-specific, monotonic and concurrency-safe.
- Campaign, renew and resign use package-private phase hooks for deterministic
  post-linearization response loss, following the SQL provider precedent.
- Successful bounded Proclaim calls count as `OperationRenew`; the armed post-success renew hook
  fails the next call after the real transaction. This proves local fail-closed transition and
  stopped future renewal traffic without gRPC interceptor coupling. Deterministic tests also prove
  one in-flight Proclaim and an operation-count upper bound of
  `ceil(elapsed/RenewInterval)+1`, including slow and failing calls.

Test-only hooks never become public API and never bypass actual etcd mutation. The lost-response
cases first perform the real operation, then replace only its observed response.

## Dependency and Fixture

Add production dependency:

```text
go.etcd.io/etcd/client/v3 v3.6.13
```

Add test dependency:

```text
github.com/testcontainers/testcontainers-go/modules/etcd v0.42.0
```

Use `gcr.io/etcd-development/etcd:v3.6.13` in the integration fixture and pin the platform digest:

```text
linux/amd64 sha256:946dfbae58b1dec56af786a23e7322484b58281547bef1e848321f6beeb388d5
linux/arm64 sha256:23c14fbdf70105a54146cf5ed3a81613b99a973c60d5907851a251ca15664e96
```

The fixture selects only the current runtime platform and records tag, digest and platform in test
output. v3.6.13 is the maintained previous minor, has a smaller transitive upgrade surface than
the newly released v3.7.0, and has the same relevant Session/Election behavior. v3.5 is outside
the normal current-plus-previous
support window. A future v3.7 upgrade is separate dependency work.

The fixture uses `etcd.Run` directly; no public `testcontainers/etcd` wrapper is added. Readiness
requires bounded `Status` evidence for member/leader plus a linearizable KV roundtrip. This proves
single-node fixture health, not quorum behavior. Partially initialized containers and clients are
cleaned up with bounded `internal/testcleanup` handling. Docker-backed tests run serially.

## Operational and Security Contract

- All contenders for one group use clients that reach the same etcd cluster and election prefix.
- Production uses an odd-sized quorum, TLS endpoint verification and authenticated clients.
- Every credential with write/delete permission in an election range is a mutually trusted
  election participant. `KeyPrefix` is collision isolation, never tenant isolation.
- etcd RBAC restricts KV read/write/watch to the exact encoded candidate range. Lease operations
  are not creator-owned or prefix-scoped capabilities. Pinned v3.6.13 tests prove that attached-key
  authorization denies both cross-principal revoke and keepalive, while an unattached lease can be
  revoked by another principal. These results narrow election-lease interference but do not prove
  general hostile-tenant isolation. Principals sharing one range are mutually trusted, mutually
  untrusted tenants require separate clusters, and every server-version change reruns both denial
  tests.
- Production TLS loads a trusted CA, validates endpoint hostname/`ServerName`, and keeps
  `InsecureSkipVerify=false`. The plaintext single-node fixture is explicitly test-only.
- Direct writes or deletes under the election prefix can force leadership loss and are restricted
  to trusted operators/roles.
- etcd linearizable reads and quorum availability define coordination availability. A minority
  partition cannot safely acquire new leadership.
- The API supplies no fencing token. Protected-resource safety requires an external fencing or
  generation check when stale leaders can continue work after losing etcd connectivity.
- Watch compaction or stream failure clears local leadership rather than guessing ownership.
- Shutdown order branches after a bounded campaign-join grace. If calls join, run bounded
  same-elector Resign/reconciliation while the client remains usable, then close it. If calls stay
  blocked, coordinate every shared-client user, close the caller-owned client, join, persist/report
  unresolved inventory and terminate that process; a separate healthy diagnostic client must prove
  exact range absence before any restart or provider cutover. TTL wait schedules reconciliation and
  never proves absence.
- Provider cutover is stop-the-world per logical group: stop protected work, drain the old provider
  and verify its safety boundary, then start etcd contenders. Rollback is symmetric: stop protected
  work and all etcd campaigns, bounded same-elector Resign, verify exact candidate-range absence,
  then re-enable the previous provider and verify zero etcd contenders. Unresolved etcd cleanup
  blocks rollback. Any provider overlap requires an external fencing authority.
- Quorum recovery never forces minority campaigning. Stop protected work on ownership loss,
  restore a majority, verify member/leader Status plus a linearizable KV roundtrip, reconcile every
  cleanup inventory item, and only then restart contenders.
- No observer API or telemetry dependency is added in this issue. Applications wrap synchronous
  calls for bounded provider/operation/result/latency metrics, sample `IsLeader` for the single
  asynchronous `leadership_lost` signal before every protected-work unit and at least every
  `min(RenewInterval, 1s)` during a long unit, and alert on the first true-to-false transition.
  They inventory `ErrCommitUnknown`/`ErrCleanupPending` at call boundaries. Labels are finite and
  never include endpoints, keys, lease IDs or owner tokens.

## Test Strategy

### Unit and deterministic lifecycle tests

- nil client, option validation, integer-second TTL conversion and token uniqueness;
- encoded exact-range construction, slash-containing and sibling-group isolation;
- no backend I/O or session creation in `New`;
- duplicate, in-progress and cleanup-pending state rejection;
- generation-safe monitor and resign races;
- server-granted TTL, EffectiveTTL concurrency and no time-only cleanup proof;
- session loss, key deletion, watch error, renew failure and stale monitor fail-closed behavior;
- caller-callback versus publication winner, created-notify timeout/cancellation, and mismatched
  PUT or DELETE immediately after the created response;
- every failed/canceled Campaign return, and every Resign path that claims owned or cleanup
  inventory, has closed `Session.Done()` and joined monitor handles;
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
- authenticated own-range success, sibling-range denial, unattached revoke behavior, and attached
  cross-principal revoke/keepalive denial, with plaintext marked test-only;
- a 32-contender resource test asserting at most 32 live leases/sessions/candidate watches, one
  Proclaim per leader, and return to the fixture baseline after cancellation plus teardown;
- repeated and race-enabled package runs, serialized against one fixture.

### Repository verification

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -p 1 -count=1 ./leader/etcd
go mod verify
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Dependency verification records the selected etcd/Testcontainers versions and the resulting
gRPC, protobuf, Prometheus, `x/net`, `x/sys` and zap module graph.

Pull-request CI runs one real-server conformance pass and records measured package/race duration.
The targeted `go test -p 1 -count=10 ./leader/etcd` soak is a nightly/manual or pre-release gate
unless cold and warm measurements show normal package tests at most three minutes, race tests at
most five minutes, and their combined eight-minute bound stays below one third of the current
25-minute CI job. Record the pre-change full-CI baseline as well; projected total runtime must stay
below 20 minutes, 80% of the job timeout, or the Docker/race work moves to a separate lane. The
supported server target is v3.6.13; other etcd minors are explicitly untested until a separate
compatibility smoke matrix is added. Dependency rollback restores the previous module files and
provider registration in one staged change.

## Documentation and Release Surface

- Add `leader/etcd/README.md` and `README.ko.md` with section-for-section parity.
- Register the provider in `leader/README.md`, `leader/README.ko.md`, root `README.md` and
  `README.ko.md`.
- Update the unreleased `CHANGELOG.md` and the v0.19.0 provider-conformance runbook.
- Include compile-checked examples for acquire/work/resign and cancellation/TTL recovery. Examples
  inspect `campaignCtx.Err()` even after nil Campaign return, stop protected work when `IsLeader`
  clears, preserve initiating and cleanup errors, retry Resign on the same elector, use
  `EffectiveTTL` only to schedule reconciliation, and never treat elapsed time as cleanup proof.
- Include a compile-checked shutdown-supervisor example with bounded cancellation grace,
  coordination of shared-client users, caller-owned client close, blocked Campaign join and
  cleanup-inventory preservation, plus symmetric rollback that requires exact range absence.
- Record a Type A lesson covering the official Campaign cleanup context, server-granted TTL,
  watch-plus-Proclaim ownership monitoring and non-skipping conformance timing profile.
- Do not add a cluster-provisioning package, generic etcd wrapper or public Testcontainers helper.

## 위험 and Mitigations

| Risk | Mitigation |
|---|---|
| Official Campaign cleanup exceeds caller deadline during partition | Call synchronously, cancel the generation to stop keepalive, document the coordinated caller-client hard stop, and retain inventory until revoke/reconciliation proof; never detach a wrapper goroutine. |
| Session expires while Campaign is completing | Validate ownership with bounded Proclaim, start a revision-correct ready watch, and publish owned state only after context re-check. |
| Session remains healthy after candidate key deletion | Watch the exact candidate key as well as `Session.Done`; fail closed on deletion or watch error. |
| Failed or compare-missed Resign clears official Election local fields | Retain deterministic key, optional revision and session; require revoke or linearizable exact-key proof before clearing. |
| Sub-second shared conformance cannot map to etcd TTL | Add source-compatible `RunWithConfig`; keep Harness and every case/assertion unchanged. |
| Opaque keepalive hides renewal operations | Use bounded compare-revision Proclaim at RenewInterval as ownership renewal while Session remains the sole lease keepalive owner. |
| Raw prefix delimiters overlap sibling groups | Encode both identity segments and use exact candidate range ends for Get, RBAC and tests. |
| v3.7 dependency causes broad transitive churn | Pin maintained v3.6.13 and verify the module graph; upgrade separately. |
| Error messages leak endpoint/key/token data | Use redacted `leader.OperationError` labels and explicit forbidden-marker tests. |
| Stale leader continues protected work | Document absence of fencing and require application-level fencing for safety-critical resources. |

## Acceptance Criteria

1. `leader/etcd` implements `leader.Elector` over a caller-owned etcd client without closing it.
2. Each campaign generation owns one explicitly granted lease plus official Session/Election
   lifecycle, joins its cancellation/monitor work, and creates no detached per-method goroutine.
3. Acquire, observation, session renewal, owner loss, cancellation, resign retry and stale cleanup
   reconciliation semantics are covered on a real etcd server; TTL passage alone never proves
   deletion.
4. `leadertest.Harness` remains unchanged; `RunWithConfig` adds source-compatible timing and
   cancel/abort/join containment with no capability flags or skipped/relaxed cases.
5. The etcd provider passes all mandatory conformance cases with the 3s/1s profile.
6. Operation failures are typed, redacted and commit-unknown/cleanup-pending recovery remains
   discoverable through `errors.Is`/`errors.As`.
7. README pairs, package registration, changelog, release runbook and Type A lesson document
   caller ownership, server-granted TTL/EffectiveTTL, exact encoded ranges, cancellation hard stop,
   lease-level trust, quorum/RBAC/TLS, symmetric stop-the-world migration, no-fencing and shutdown
   requirements.
8. Targeted, race, repeated, dependency and repository CI verification pass with Docker-backed
   packages serialized.
