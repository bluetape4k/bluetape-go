# Issue #527 Provider Conformance Pre-Implementation Risk Record

**Milestone:** 0.19.0  
**Plan commit:** `bf0afa92c31059350778252213e2debc18b0514e`  
**Recorded before source changes:** yes

## Step 3-R Gate

The approved plan passed the required plan review with P0=0 and P1=0 across performance, stability, security, operator/Ops, developer/API, and user/caller perspectives. The operator lane did not conclude within its bounded review window, so `lane timed out; main integration fallback performed`. Main integration verified bounded retry and cleanup, TTL takeover, mixed-version migration, telemetry, canary, rollback, and container lifecycle requirements. The performance integration verified that normal paths add no storage schema or replay protocol, fault probes remain exceptional-path work, and the private nil-hook path has an allocation and latency regression gate.

## Predicted Risks

| Risk | Detection signal | Mitigation and rerun gate | Rollback or recovery owner |
|---|---|---|---|
| Late acquisition after caller cancellation | owner probe shows a token after a bare context error; exact winner/operation counts disagree | distinguish pre-dispatch cancellation from post-linearization response loss; reconcile by owner token and require the common runner plus race gate | provider task owner reverts the provider commit; bounded Resign or TTL removes an indeterminate owner |
| Dispatch commits but response is lost | mutation count is one while the returned error is bare context or lacks `ErrCommitUnknown` | inject failure after the real mutation boundary; require typed provider error, owner-aware cleanup, and no false confirmed failure | leader/lock provider owner retries bounded cleanup; rate caller waits a full refill window and does not replay |
| Resign overlaps an in-flight renewal | renew count grows after resign begins or takeover never succeeds | stop new scheduling, permit at most one already-linearized renew, retain cleanup state, and prove eventual retry/TTL takeover | leader provider owner rolls back state-machine commit and waits for TTL before rollout retry |
| Retry storm | Redis command rate exceeds 12 operations/second during bounded contention/reconciliation | fixed bounded retry budget, caller deadlines, exact operation-count assertions, canary command-rate alert | operator halts canary and reverts the provider commit |
| False-positive conformance gate | an intentionally broken adapter passes a named case or classifier accepts nil/bare context/raw cause | evaluator self-tests for every invariant, including always-true/false/panic classifiers and raw-marker diagnostics | helper-package owner reverts the helper commit before provider adoption |
| Helper import cycle or provider-shaped public API | `go test`/`go list` reports a cycle or neutral adapters require provider imports | keep helper result/error contracts parent-independent and convert provider types explicitly at adapter boundaries | helper-package owner reverts the API commit and redraws the neutral boundary |
| Key or owner-token migration corrupts existing ownership | valid byte table changes, whitespace token is trimmed, old/new owner cannot release safely | preserve Redis key/schema and generated tokens; validate structure without mutating valid custom token bytes; run mixed-version/byte-identity tests | provider owner reverts behavior commit; release owner documents the custom-token exception |
| Testcontainers or client leak | cleanup leaves a container/process/client, or failure path hides the first cleanup error | dedicated clients, immediate cleanup registration, independent client close and container termination, serial shared-resource tests | fixture owner fixes cleanup before rerun; no provider verification claim while Docker is unavailable |
| TokenBucket hot-path regression | `allocs/op` rises above zero or median latency rises more than 10% against the baseline | private default-nil hook, identical five-sample benchmark before/after; any allocation blocks progress and latency regression requires investigation and fresh samples | local rate-limit owner reverts the hook commit |
| Secret/key/token/endpoint leakage | forbidden markers appear in rendered errors or captured runner diagnostics | safe typed wrappers preserve `errors.As`/sentinels while redacting raw cause context; provider and broken-adapter marker tests | error-contract owner reverts the offending wrapper/diagnostic commit |

## TokenBucket Baseline

Environment:

```text
go version go1.26.5 darwin/arm64
goos: darwin
goarch: arm64
cpu: Apple M4 Pro
```

Exact command:

```bash
go test -run '^$' -bench 'TokenBucket' -benchmem -count=5 ./ratelimit
```

Authoritative samples:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| TokenBucketAllowAllowed-12 | 113.7 | 0 | 0 |
| TokenBucketAllowAllowed-12 | 113.8 | 0 | 0 |
| TokenBucketAllowAllowed-12 | 113.9 | 0 | 0 |
| TokenBucketAllowAllowed-12 | 121.8 | 0 | 0 |
| TokenBucketAllowAllowed-12 | 115.9 | 0 | 0 |
| TokenBucketAllowRejected-12 | 76.30 | 0 | 0 |
| TokenBucketAllowRejected-12 | 75.77 | 0 | 0 |
| TokenBucketAllowRejected-12 | 78.86 | 0 | 0 |
| TokenBucketAllowRejected-12 | 75.78 | 0 | 0 |
| TokenBucketAllowRejected-12 | 77.16 | 0 | 0 |

Task 9 must rerun the identical command. Any non-zero allocation is a stop condition. A latency regression above 10% must be investigated and rerun; it cannot be dismissed without fresh samples.
