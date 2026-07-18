# Issue #537 etcd Leader Step 3-R Plan Review

Date: 2026-07-19 KST
Issue: [#537](https://github.com/bluetape4k/bluetape-go/issues/537)
Reviewed plan: `docs/superpowers/plans/2026-07-19-issue-537-etcd-leader-plan.md`
Final reviewed commit: `bac98c95114d05dfba712d71667bcf95c7f9750c`
Reviewed SHA-256: `0a88baf47bf5c5a21c4bfd40299a05c36a3f715d99aa23daf46a8eaa1d3b74e6`
Approved spec commit: `287c5eaffa025969a2eae15affc8f5b5faddbe21`
Baseline: `origin/develop@41663dea0a2a34cd459df24802f59882cff834aa`

## Integrated Verdict

`PASS — P0=0 P1=0`

Six independent role-scoped lanes reviewed each exact plan revision in two
bounded waves, and the main session integrated the results. Every P0/P1 was
repaired and all six perspectives reran against the final exact commit. This
review validates implementation readiness only; no `leader/etcd` source,
dependency, workflow, or runtime behavior was implemented during Step 3-R.

## Review Convergence

| Round | Exact commit | Performance | Stability | Security | Ops | Developer/API | User/caller |
|---|---|---:|---:|---:|---:|---:|---:|
| Initial | `525d209` | 0/2/3 | 0/5/4 | 0/2/1 | 0/2/3 | 0/5/4 | 0/3/2 |
| R2 | `6072e4e` | 0/1/2 | 0/1/3 | 0/0/0 | 0/1/2 | 0/1/4 | 0/0/1 |
| R3 | `f54112d` | 0/0/1 | 0/1/2 | 0/0/0 | 0/1/0 | 0/1/1 | 0/1/0 |
| R4 final | `bac98c9` | 0/0/0 | 0/0/1 | 0/0/0 | 0/0/0 | 0/0/2 | 0/0/0 |

Each cell is `P0/P1/P2`. The final distinct P2 set contains two bounded plan
clarifications: Task 4's RED explanation still says monitor code is absent even
though Task 3 creates the minimal monitor primitive, and Task 5's narrow regex
does not name the zero-value contract test. Both tests are included by the
later full-package gates; neither changes implementation architecture,
correctness criteria, or the `P0=0 P1=0` verdict.

## Final Exact-Commit Results

| Lane | P0 | P1 | P2 | Verdict |
|---|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | PASS |
| Stability/concurrency | 0 | 0 | 1 | PASS |
| Security | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 2 | PASS |
| User/caller | 0 | 0 | 0 | PASS |
| Main-session integration | 0 | 0 | 2 | PASS |

## Blocking Findings Resolved

### Task order and dependency integrity

- Added the production etcd client only after RED source required it and
  deferred the Testcontainers etcd module until its fixture import existed.
- Kept the constructor task compilable with a minimal generation type and
  deferred the interface assertion until all public methods exist.
- Required tidy/verify stability and explicit review of the gRPC, protobuf,
  Prometheus, zap, and `x/*` graph changes.

### Conformance containment

- Preserved public `Harness` and `Run` source compatibility while adding
  `RunWithConfig`, zero-valued timing defaults, and overflow-safe containment.
- Changed the private evaluator shape to receive one cancelable case root and
  required every Campaign, Control, wait, contention, and Resign context to
  derive from it. Only Abort receives an independent post-cancel context.
- Added cancel-before-Abort, abort/join ordering, and fail-stop subprocess
  evidence so timed-out provider work cannot leak into the next case.

### Generation lifecycle and official etcd feasibility

- Made operations per-elector and immutable-by-value per generation, including
  Session lifecycle, ticker, watch, revoke, and lookup seams.
- Added `snapshotElection` so deterministic tests can supply official Election
  key/create-revision/header-revision state without mutating unexported etcd
  fields; production validates a non-nil Header and positive revisions.
- Moved the monitor start/Created/terminal/join primitive into Campaign's task,
  handed it watch and Session signals before publication, and required a locked
  terminal-state recheck for Created-then-loss races.
- Required nil-session-safe shutdown, `Session.Orphan`, closed `Session.Done`,
  joined monitor work, and separate remote cleanup proof.

### Cleanup, errors, and caller contracts

- Followed every dispatched official Resign result with bounded revoke and
  linearizable exact-key proof; TTL passage and Session termination never clear
  unresolved inventory.
- Corrected official API usage for `leader.NewOperationError`,
  `WithFirstCreate()...`, `ResumeElection`, explicit Grant, and server-granted
  TTL handling.
- Separated synchronous Campaign/Resign/Leader error assertions from
  asynchronous renew failure, while preserving sanitized strings and
  `errors.Is`/`errors.As` identity.
- Documented that the exported `Elector` zero value is unusable, required `New`,
  and added an external-package deterministic no-panic/no-dispatch contract.

### Real-server, security, and resources

- Required one case-dedicated concurrency-safe client shared by every elector
  in a conformance case, with Abort closing all and only that case's users.
- Selected immutable etcd image digests from `DOCKER_DEFAULT_PLATFORM` or
  Docker daemon target information and passed the same platform explicitly to
  Testcontainers; host GOOS is never used as the container target.
- Added isolated authenticated RBAC evidence, attached-key cross-principal
  revoke denial, unattached revoke success, caller-owned verified TLS, and
  repository-owned diagnostic redaction.
- Added exact lease, watcher, Session, monitor, Proclaim, contender, and
  teardown-to-baseline resource assertions.

### Documentation and CI admission

- Required compile-checked acquire, shutdown, TLS, cutover, and symmetric
  rollback examples plus English/Korean section parity and executable runbook
  contracts.
- Added `EffectiveTTL` transition/race coverage and corrected generic
  `leader.Elector` cleanup documentation so provider-specific proof rules win.
- Made workflow parsing mandatory with pinned actionlint and admitted CI by
  complete job duration, including setup, Docker pull, existing workloads, and
  all tests. Dedicated PR/nightly jobs must retain at least 20% live headroom.

## Main-Session Integration Check

- The plan implements the approved design without adding a generic etcd value
  codec, public wrapper, provider-owned client, capability skip, or detached
  Campaign mutation path.
- All 15 conformance cases remain named, ordered, mandatory, and unrelaxed.
- Local goroutine termination and remote ownership cleanup use separate proof
  obligations on every failure, cancellation, shutdown, and retry path.
- Test seams map to official etcd v3.6.13 and Testcontainers v0.42.0 APIs.
- Public API, zero-value behavior, error identity, trust boundary, and
  caller-owned lifecycle are testable before documentation claims are accepted.
- Task ordering is TDD-oriented and each commit example follows the repository
  Lore protocol.
- Implementation remains blocked on a fresh explicit user approval after this
  reviewed-plan handoff.

## Verification

```bash
git diff --check 41663dea0a2a34cd459df24802f59882cff834aa..bac98c95114d05dfba712d71667bcf95c7f9750c
git show --check --stat bac98c95114d05dfba712d71667bcf95c7f9750c
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/*.yml
```

Result: PASS.

Runtime etcd behavior, Docker-backed conformance, race execution, dependency
mutation, documentation changes, live CI, PR review, and merge readiness remain
future gates in the approved implementation plan.
