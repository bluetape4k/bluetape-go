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
func (e *Elector) EffectiveTTL() time.Duration
```

`New` validates a non-nil client and normalized `leader.Options`, requires
`RenewInterval < Lease`, creates a cryptographically random owner token, computes the encoded
election prefix and requested integer-second TTL, and performs no backend I/O. It never creates a
lease or session until `Campaign` starts and never closes the caller's client. A compile-time
assertion keeps `*Elector` assignable to `leader.Elector`; the zero-value `Elector` is not usable.

`EffectiveTTL` is a concrete-provider diagnostic because etcd may grant a different TTL from the
request. Before the first successful grant it returns the requested rounded TTL. After a grant it
returns the current generation's, or last successfully granted, server TTL. Each generation also
retains its own granted TTL so later grants cannot shorten an older cleanup deadline. Callers use
this value for cleanup wait budgets; it is not added to the backend-neutral interface.

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

Both segments use unpadded `base64.RawURLEncoding`. `concurrency.Election` receives the path above
plus one trailing separator and stores lease-suffixed candidates below it. Every range operation
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
- one generation record containing lease ID, granted TTL, candidate key, creation revision,
  latest mutation time, cleanup deadline, token, cancel and completion handles;
- after acquisition, one generation-owned ownership monitor.

The requested positive `Lease` is rounded up to the next whole second because etcd lease grants
use integer seconds. The provider never rounds down; a sub-second lease therefore requests one
second. `Campaign` calls `Lease.Grant` explicitly, records `LeaseGrantResponse.TTL`, and adopts that
lease with `WithLease`; cleanup budgets and expiry fallback always use the granted value, not the
request. Constructor tests cover exact seconds, fractional round-up, overflow and the
`RenewInterval < Lease` rule. Grant tests cover a server TTL that differs from the request.

The session owns lease keepalive cadence. Separately, the provider honors `RenewInterval` as an
ownership-validation cadence by calling `Election.Proclaim` with the same token. Proclaim's
compare-revision transaction proves that the candidate still exists and is still this generation;
it does not replace or duplicate lease keepalive.

## Campaign Lifecycle

`Campaign(ctx)` performs the following serialized state transition:

1. Reject nil or already-ended contexts before dispatch.
2. Under the state mutex, reject `ErrAlreadyLeader`, `ErrCampaignInProgress` or
   `ErrCleanupPending`; otherwise mark campaign in progress and allocate a generation.
3. Explicitly grant the rounded TTL, record the server-granted TTL, and create the session using
   the generation context plus `WithLease`.
4. Create `concurrency.Election`. Build a campaign context canceled by caller completion,
   generation cancellation or `Session.Ctx()` completion. The cancellation bridges use joined
   `context.AfterFunc` callbacks; callbacks are stopped or observed before method return.
5. Call `Election.Campaign(campaignCtx, token)` synchronously. No wrapper goroutine is detached.
6. On apparent success, perform a bounded `Election.Proclaim` as compare-revision validation.
   A missing/replaced key fails closed instead of returning stale success.
7. Snapshot `Election.Key()`, `Election.Rev()` and `Election.Header().Revision`. Start an exact-key
   watch at the Proclaim header revision plus one and wait for its ready handshake.
8. Stop the caller-cancellation bridge, re-check the caller and session contexts, then publish
   owned state plus monitor handles atomically. Return success only after publication.

Caller cancellation cancels the generation context and therefore stops session keepalive even if
the official Campaign cleanup RPC remains blocked. After successful publication the caller bridge
is removed, so later cancellation of the acquisition context does not end established leadership.
Every campaign exit joins its local cancellation callbacks; monitor goroutines are generation-owned
and must be joined by cleanup.

If Campaign or Proclaim fails after backend dispatch, ownership can be indeterminate. The elector
retains the session/key/revision generation as cleanup inventory, immediately cancels the
generation context, waits for `Session.Done`, records a conservative expiry deadline, attempts a
bounded lease revoke, and returns a redacted `leader.OperationError`. If absence or revoke cannot
be proved, the result also matches `leader.ErrCommitUnknown`; a following Campaign returns
`leader.ErrCleanupPending` until the same elector completes `Resign`, proves exact-key absence, or
the recorded server-granted-TTL deadline has elapsed.

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
- unresolved remote cleanup is bounded for safety by observed keepalive stop plus the
  server-granted TTL and a one-second clock/scheduling margin;
- strict wall-clock return by the caller deadline is not guaranteed during an etcd/network
  partition;
- the production hard stop is: cancel all campaign contexts, wait a bounded grace period,
  coordinate every shared-client user, close the caller-owned client, join the calls, and retain
  the cleanup inventory until exact absence or TTL expiry;
- applications must not start protected work after a campaign context has ended, even if a late
  successful return is observed.

This is an explicit etcd primitive limitation, not a promise weakened through a hidden goroutine.

## Ownership Monitoring and Renewal

After acquisition, one provider-owned monitor observes three event sources:

- `Session.Done()`, which closes when the lease keepalive stream ends or the session is canceled;
- a watch of the exact candidate key beginning at the successful Proclaim header revision plus one;
- a `RenewInterval` ticker that performs one bounded compare-revision `Election.Proclaim`.

The watch has a ready handshake before Campaign can publish success. PUT events from Proclaim are
ignored; delete, cancellation, compaction and watch errors invalidate ownership. Session loss or
any Proclaim failure also invalidates ownership. Every invalidation atomically clears `IsLeader`,
marks cleanup pending when absence is not proved, and immediately cancels the generation context
so keepalive stops. A generation check prevents a stale monitor from clearing newer state.

Each Proclaim uses a fresh operation context bounded by the smallest positive value among
`RenewInterval`, granted TTL divided by four, and one second. The monitor holds no mutex across a
backend RPC, join or wait. The session remains the only lease keepalive owner; Proclaim is the
provider's public `renew` operation and deterministic conformance fault seam.

## Resign and Cleanup Lifecycle

`Resign(ctx)` is idempotent and serializes with Campaign and monitor transitions.

1. Reject nil or already-ended contexts before dispatch.
2. If there is no owned or cleanup generation, return nil.
3. Mark local leadership false, prevent a new Campaign, and immediately cancel the generation
   context so session keepalive stops.
4. Join the exact generation monitor so no stale monitor mutates later state.
5. Reconstruct the election with the retained session, key and revision using
   `concurrency.ResumeElection` when necessary, then call its compare-revision `Resign(ctx)`.
6. On confirmed delete or confirmed absence, revoke the lease with the remaining caller budget.
7. Clear generation state only after cleanup is confirmed. Repeated Resign then returns nil.

The key/revision snapshot is retained independently of `Election`'s mutable local fields because
the official `Resign` clears its fields even when the RPC returns an error. A failed dispatched
resign returns a redacted operation error matching `leader.ErrCommitUnknown`, preserves cleanup
inventory and permits a fresh-context retry through `ResumeElection`. The creation-revision
compare prevents a stale retry from deleting a replacement owner.

If the caller context ends while monitor join, delete or revoke is incomplete, `Resign` preserves
cleanup state. When `Session.Done` is observed, the generation records a conservative cleanup
deadline of `max(sessionDoneTime, lastPossibleMutationTime) + grantedTTL + 1s`. Campaign and Resign
entry reconcile exact key/revision/token absence and clear stale inventory; if reconciliation is
unavailable, the deadline is the final safety boundary. Resign called while another Campaign is
still waiting follows the common contract and returns nil; callers cancel the Campaign context to
stop that attempt.

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

Every dispatched provider failure is wrapped with
`leader.NewOperationError("etcd", operation, cause)`. Stable public operation labels remain
provider-neutral: `campaign`, `renew`, `resign` and `lookup`. Grant, session, watch, Proclaim,
revoke and reconciliation are private phases mapped to the owning public operation.

- pre-dispatch nil context returns `leader.ErrInvalidContext`;
- pre-dispatch canceled/deadline context returns the bare context error;
- campaign/renew failure after possible mutation matches `leader.ErrCommitUnknown` unless
  reconciliation or revoke proves no live ownership;
- resign/revoke failure after possible mutation matches `leader.ErrCommitUnknown` and retains
  cleanup state;
- session/watch loss makes `IsLeader=false`; unresolved remote ownership retains cleanup state;
- rendered errors preserve error type and `errors.Is`/`errors.As` without rendering raw etcd text;
- provider-owned logs, examples and telemetry never unwrap causes. Tests verify `Error()` and every
  repository-owned logging path omit endpoint, username, password, token, encoded key/group, lease
  ID, certificate path and injected raw markers. Documentation warns that an explicitly unwrapped
  etcd cause is diagnostic-sensitive and must not be logged unsanitized.

No automatic Campaign retry occurs after an indeterminate mutation. Callers retry bounded Resign
on the same elector and fall back to full effective-TTL wait.

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
}

type AbortFunc func(context.Context, leader.Options) error

type Config struct {
    Timing Timing
    Abort  AbortFunc
}

func Run(t *testing.T, harness Harness)
func RunWithConfig(t *testing.T, harness Harness, config Config)
```

The zero-value `Timing` resolves field-by-field to the current defaults:

- lease: 300ms;
- renew interval: 50ms;
- case timeout: 5s;
- wait timeout: 2s;
- resign timeout: 250ms.

Normalization rejects negative values, any resolved `RenewInterval >= Lease`, non-positive
timeouts, and `WaitTimeout` or `ResignTimeout` values that do not leave cleanup margin below
`CaseTimeout`. `Run` is a wrapper around `RunWithConfig` using current defaults. Both execute the
same named 15-case table in the same order; there are no capability flags, filters, skips or
relaxed assertions.

Every case owns a cancelable root context and all evaluator contexts derive from it. On case
timeout the runner cancels the root, waits a bounded join grace, invokes `Abort` if the case is
still running with a fresh bounded abort context, and joins the case goroutine before the subtest
returns. A provider adapter that can hit official unbounded cleanup must supply case-dedicated
clients and an Abort function that closes only those clients. This prevents timed-out cases from
mutating later tests or leaking sessions.

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
  stopped future renewal traffic without gRPC interceptor coupling.

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
  election participant. `KeyPrefix` is collision isolation, never tenant isolation. Tenant or
  trust isolation requires separate exact encoded ranges and credentials, or separate clusters.
- etcd RBAC restricts KV read/write/watch to the exact encoded candidate range. Lease operations
  are not prefix-scoped authorization; documentation must not claim they are. Authenticated
  integration tests prove the role can use its own range and cannot read/write a sibling range.
- Production TLS loads a trusted CA, validates endpoint hostname/`ServerName`, and keeps
  `InsecureSkipVerify=false`. The plaintext single-node fixture is explicitly test-only.
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
- Provider cutover and rollback are stop-the-world per logical group: stop protected work, drain
  the old provider and its TTL boundary, then start etcd contenders. Old and etcd providers must
  never campaign the same logical group concurrently without an external fencing authority.
- Telemetry records only safe provider/operation/result/status labels. It never records full
  endpoints, keys, lease IDs or owner tokens. Required signals cover campaigning, leadership,
  session/watch loss, cleanup-pending age, commit-unknown, revoke outcome and operation latency.

## Test Strategy

### Unit and deterministic lifecycle tests

- nil client, option validation, integer-second TTL conversion and token uniqueness;
- encoded exact-range construction, slash-containing and sibling-group isolation;
- no backend I/O or session creation in `New`;
- duplicate, in-progress and cleanup-pending state rejection;
- generation-safe monitor and resign races;
- server-granted TTL, EffectiveTTL concurrency and per-generation cleanup deadline;
- session loss, key deletion, watch error, renew failure and stale monitor fail-closed behavior;
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
- authenticated own-range success and sibling-range denial, with plaintext marked test-only;
- higher-contention bounded resource test documenting O(contenders) leases, sessions and watches;
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
unless its measured upper bound fits normal CI. The supported server target is v3.6.13; other etcd
minors are explicitly untested until a separate compatibility smoke matrix is added. Dependency
rollback restores the previous module files and provider registration in one staged change.

## Documentation and Release Surface

- Add `leader/etcd/README.md` and `README.ko.md` with section-for-section parity.
- Register the provider in `leader/README.md`, `leader/README.ko.md`, root `README.md` and
  `README.ko.md`.
- Update the unreleased `CHANGELOG.md` and the v0.19.0 provider-conformance runbook.
- Include compile-checked examples for acquire/work/resign and cancellation/TTL recovery. Examples
  inspect `campaignCtx.Err()` even after nil Campaign return, stop protected work when `IsLeader`
  clears, preserve initiating and cleanup errors, retry Resign on the same elector, wait
  `EffectiveTTL` where required, and close the caller client last.
- Record a Type A lesson covering the official Campaign cleanup context, server-granted TTL,
  watch-plus-Proclaim ownership monitoring and non-skipping conformance timing profile.
- Do not add a cluster-provisioning package, generic etcd wrapper or public Testcontainers helper.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Official Campaign cleanup exceeds caller deadline during partition | Call synchronously, cancel the generation to stop keepalive, document the coordinated caller-client hard stop, and use granted-TTL fallback; never detach a wrapper goroutine. |
| Session expires while Campaign is completing | Validate ownership with bounded Proclaim, start a revision-correct ready watch, and publish owned state only after context re-check. |
| Session remains healthy after candidate key deletion | Watch the exact candidate key as well as `Session.Done`; fail closed on deletion or watch error. |
| Failed Resign clears official Election local fields | Retain key/revision/session independently and retry with `ResumeElection`. |
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
3. Acquire, observation, session renewal, owner loss, cancellation, resign retry, stale cleanup and
   server-granted-TTL fallback semantics are covered on a real etcd server.
4. `leadertest.Harness` remains unchanged; `RunWithConfig` adds source-compatible timing and
   cancel/abort/join containment with no capability flags or skipped/relaxed cases.
5. The etcd provider passes all mandatory conformance cases with the 3s/1s profile.
6. Operation failures are typed, redacted and commit-unknown/cleanup-pending recovery remains
   discoverable through `errors.Is`/`errors.As`.
7. README pairs, package registration, changelog, release runbook and Type A lesson document
   caller ownership, server-granted TTL/EffectiveTTL, exact encoded ranges, cancellation hard stop,
   quorum/RBAC/TLS, stop-the-world migration, no-fencing and shutdown requirements.
8. Targeted, race, repeated, dependency and repository CI verification pass with Docker-backed
   packages serialized.
