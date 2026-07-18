# Issue #537 etcd Leader Step 2-R Spec Review

Date: 2026-07-19 KST
Issue: [#537](https://github.com/bluetape4k/bluetape-go/issues/537)
Reviewed spec: `docs/superpowers/specs/2026-07-19-issue-537-etcd-leader-design.md`
Final reviewed commit: `287c5eaffa025969a2eae15affc8f5b5faddbe21`
Reviewed SHA-256: `1334036ae726aa3d3adb9085712828e0a4936a3c61ea865ce2f20dcc6f215339`
Baseline: `origin/develop@41663dea0a2a34cd459df24802f59882cff834aa`

## Integrated Verdict

`PASS — P0=0 P1=0`

Six independent lanes reviewed the same final spec commit. The three available
read-only research threads were reused in two bounded waves; each perspective
received a separate role-scoped assignment, and the main session performed the
integration review. No implementation or runtime etcd behavior was exercised
during Step 2-R.

## Final Exact-Commit Results

| Lane | P0 | P1 | P2 | Verdict |
|---|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | PASS |
| Stability/concurrency | 0 | 0 | 2 | PASS |
| Security | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 2 | PASS |
| User/caller | 0 | 0 | 1 | PASS |
| Main-session integration | 0 | 0 | 0 | PASS |

The stability and Developer/API P2 notes overlap: the implementation plan must
make generation shutdown nil-session-safe, qualify `Session.Done` assertions
for successful Session creation, and explicitly join a monitor created before
publication when a later Campaign step fails. The caller P2 note requires docs
to state that protected work from the prior generation must stop and join
before reacquisition, or the caller must provide external generation fencing.
These are bounded implementation and documentation obligations, not unresolved
design blockers.

## Findings Resolved During Review

### Performance

- Added explicit `joinGrace` and `abortBudget` relationships, bounded hard-stop
  behavior, and a fail-stop branch when a stuck operation cannot be contained.
- Made renewal single-flight and rate-bounded, and separated the 32-contender
  resource test from container startup and teardown noise.
- Added package, race, and full-CI admission budgets anchored to a measured
  pre-change baseline.

### Stability/concurrency

- Split caller cancellation from ownership publication with one serialized
  winner and required late success to enter cleanup instead of being published.
- Based expiry and keepalive timing on the server-granted lease TTL, and made
  `EffectiveTTL` observable.
- Required `WithRev(proclaimRevision+1)` plus created notification for loss
  monitoring, and exact PUT identity validation before leadership publication.
- Made generation shutdown idempotent and shared by monitor loss and `Resign`:
  cancel, `Session.Orphan`, observe closed `Session.Done`, and join monitor work.
- Kept local Session termination separate from remote cleanup proof.

### Security

- Defined separator-free encoded election bases and non-overlapping candidate
  ranges used consistently by `Election`, `ResumeElection`, watch, and cleanup.
- Replaced an overstated RBAC isolation claim with the exact lease-level trust
  boundary: same-cluster principals that may revoke another contender's lease
  must be mutually trusted.
- Required authenticated TLS, least-privilege prefix access, redacted errors,
  and a cross-principal revoke boundary test.

### Operator/Ops

- Added coordinated shutdown for in-flight Campaign, healthy and blocked
  cleanup branches, retained unresolved inventory, and restart preconditions.
- Made cutover and rollback symmetric and proof-gated, including quorum-loss
  rules and minority-campaign prohibition.
- Kept observability caller-owned with finite labels and bounded leadership-loss
  sampling.

### Developer/API

- Preserved `Harness` and `Run` source compatibility while adding
  `RunWithConfig`, `Timing`, and `Config` with zero-value defaults and an
  unexported field to reject external unkeyed literals.
- Kept the public provider API narrow: caller-owned `*clientv3.Client`,
  constructor-only `*Elector`, concrete `EffectiveTTL`, and typed/redacted
  operation errors.
- Defined exact lease Grant, server-granted TTL, Session construction,
  `WithLease` Campaign, `ResumeElection`, and cleanup reconciliation semantics
  against official etcd APIs.

### User/caller

- Made granted TTL discoverable and gave `RenewInterval` the precise meaning of
  bounded single-flight `Proclaim` cadence rather than lease keepalive cadence.
- Required cleanup state to clear only after successful revoke or linearizable
  exact-key absence/replacement proof; elapsed TTL alone is never proof.
- Added symmetric cutover/rollback, cancellation, cleanup recovery, caller-owned
  client shutdown, and compile-checked bilingual documentation obligations.

## Main-Session Integration Check

- The provider stores only the encoded election value required by etcd's
  Election protocol; it introduces no general application-value codec.
- Every ownership transition has a local lifecycle join and a distinct remote
  proof rule.
- Cancellation, loss, resign, shutdown, restart, cutover, and rollback paths
  converge on one bounded generation lifecycle.
- The conformance harness remains provider-neutral and source-compatible.
- Security claims match what etcd prefix permissions and lease revocation can
  actually enforce.
- Every issue acceptance criterion maps to an implementation, test,
  documentation, or operational obligation in the spec.
- No implementation mutation is authorized by this review artifact itself;
  Step 3 implementation planning and plan review remain the next gate.

## Verification

```bash
git diff --check 41663dea0a2a34cd459df24802f59882cff834aa..287c5eaffa025969a2eae15affc8f5b5faddbe21
git show --check --stat 287c5eaffa025969a2eae15affc8f5b5faddbe21
```

Result: PASS.

Runtime etcd behavior, Docker integration, race execution, and performance
budgets are intentionally deferred to the approved implementation plan and its
test gates.
