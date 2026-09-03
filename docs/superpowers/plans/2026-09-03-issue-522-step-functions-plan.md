# Step Functions 실행 bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `workflow/stepfunctions`에 caller-owned AWS SDK for Go v2 client로 Step Functions 실행을 시작·조회·중지하고 bounded polling으로 관찰하는 Go-native bridge를 추가한다.

**Architecture:** `StartExecution`과 `DescribeExecution`을 필수로 주입하는 좁은 `Client`와 선택적 `StopClient` capability를 사용한다. bridge는 입력 bound, typed response/error mapping, context checkpoint를 소유하고 AWS client lifecycle, credentials, retry, timeout, IAM, state-machine provisioning은 caller/operator에게 남긴다. `Wait`는 즉시 describe 후 cancellable timer와 capped backoff를 사용하며 자동 stop/retry를 하지 않는다.

**Tech Stack:** Go 1.26.3, AWS SDK for Go v2 `github.com/aws/aws-sdk-go-v2/service/sfn@v1.48.0`, standard library `context`, `encoding/json`, `errors`, `reflect`, `time`, table-driven tests, `go test -race`, compile-checked examples.

**Source of truth:** `docs/superpowers/specs/2026-09-03-issue-522-step-functions-design.md`, issue #522, research gate #513, AWS `StartExecution`/`DescribeExecution`/`StopExecution` API references.

---

## File map and ownership

- Create `workflow/stepfunctions/doc.go`: package purpose and caller-owned boundary.
- Create `workflow/stepfunctions/errors.go`: safe sentinel and typed error mapping.
- Create `workflow/stepfunctions/bridge.go`: client interfaces, request/response values, validation, start/describe/stop/wait behavior.
- Create `workflow/stepfunctions/bridge_test.go`: fake client and table-driven contract tests; no live AWS.
- Create `workflow/stepfunctions/example_test.go`: compile-checked caller usage with fake client.
- Create `workflow/stepfunctions/README.md` and `workflow/stepfunctions/README.ko.md`: API, limits, polling, failure, IAM and live-test boundaries.
- Modify `workflow/README.md`, `workflow/README.ko.md`, root `README.md`, and root `README.ko.md`: package index/link parity.
- Modify `go.mod` and `go.sum`: add only `service/sfn` and required checksums.
- Create `docs/review/2026-09-03-issue-522-step-functions-implementation-review.md` and `docs/lessons/2026-09-03-issue-522-step-functions.md` before PR.
- Do not modify existing `workflow` runtime behavior or add state-machine/provisioning wrappers.

## Task 1: Dependency and constructor contract (TDD RED/GREEN)

**Files:** `go.mod`, `go.sum`, `workflow/stepfunctions/doc.go`, `errors.go`, `bridge.go`, `bridge_test.go`.

- [ ] **Step 1: Add the AWS SDK dependency.** Run `go get github.com/aws/aws-sdk-go-v2/service/sfn@v1.48.0`. Expected: selected module/checksums only; existing root SDK line remains compatible.
- [ ] **Step 2: Write RED tests.** Define `Client` with only `StartExecution`/`DescribeExecution`, `StopClient` with only `StopExecution`, `Options{Client, MaxInputSize}`, `StartRequest`, `StopRequest`, `WaitOptions`, and `Backoff`. Test `New(Options{})`, typed-nil clients for every reflect-nil kind, negative/over-limit input size, and zero-value `Bridge`; assert `errors.Is` sentinels and zero provider calls.
- [ ] **Step 3: Run RED.** `go test -count=1 ./workflow/stepfunctions`; record missing constructor/sentinel failures.
- [ ] **Step 4: Implement minimum contract.** Add immutable `Bridge`, constructor/default input limit, nil-context normalization, typed-nil detection, and safe `*Error` (`Error`, `Unwrap`, `Is`, status accessor). No provider text, ARN, payload, credential, or trace-header formatting.
- [ ] **Step 5: Run GREEN.** `gofmt -w workflow/stepfunctions/*.go && go test -count=1 ./workflow/stepfunctions`; constructor/zero-value tests must pass.

## Task 2: Start, Describe, and optional Stop

**Files:** `workflow/stepfunctions/bridge.go`, `errors.go`, `bridge_test.go`.

- [ ] **Step 1: Start RED tests.** Fake deep-copies `StartExecutionInput`, records context/call count, and covers valid mapping, nil input→`{}`, invalid ARN/name/JSON/UTF-8/size, trace header, and pre-dispatch cancellation; every reject has zero calls.
- [ ] **Step 2: Implement `Start`.** Validate ARN (1–256 UTF-8 bytes), optional ASCII name (1–80 `[A-Za-z0-9_-]`), input (nil/empty `{}`, valid UTF-8 JSON, configured maximum 262144 bytes), and trace header (ASCII ≤256). Build `sfn.StartExecutionInput`, check context before/after SDK call, map transport to `ErrStartFailed`, and reject nil/missing ARN or start time as `ErrMalformedOutput`.
- [ ] **Step 3: Describe/Stop RED tests.** Cover ARN validation, request mapping, every known status, optional metadata, malformed required/optional fields, transport errors, absent stop capability, error/cause bounds, and before/after cancellation.
- [ ] **Step 4: Implement `Describe`.** Map required ARN/state-machine ARN/status/start time and optional name/input/output/error/cause/stop time into `Execution`; validate AWS byte bounds/UTF-8; map transport to `ErrDescribeFailed` while preserving `errors.Is` cause.
- [ ] **Step 5: Implement `Stop`.** Type-assert `StopClient`; absent capability returns `ErrStopUnsupported` without a call. Validate ARN, optional error ≤256 and cause ≤32768 bytes, checkpoint context, map transport to `ErrStopFailed`, require `StopDate`, and never call Stop from Wait.
- [ ] **Step 6: Focused GREEN.** Run `gofmt -w workflow/stepfunctions/*.go`, `go test -count=1 ./workflow/stepfunctions`, and `go vet ./workflow/stepfunctions`; all mapping/error tests must pass.

## Task 3: Bounded Wait and GO-HARD-08

**Files:** `workflow/stepfunctions/bridge.go`, `errors.go`, `bridge_test.go`.

- [ ] **Step 1: Wait RED tests.** Fake sequences cover immediate success, `RUNNING→SUCCEEDED`, `FAILED`, `TIMED_OUT`, `ABORTED`, `PENDING_REDRIVE`, unknown status, describe transport error, explicit timeout, caller cancellation during timer and after response, custom backoff order/cap, and proof of no implicit Stop/retry.
- [ ] **Step 2: Normalize options.** Defaults: poll interval 1s, max interval 30s, timeout 0 (caller context/deadline only). Reject negative/invalid values and max < poll. Custom `Backoff(attempt, previous)` may return zero, rejects negative values, and is capped at max.
- [ ] **Step 3: Implement polling.** Validate before calls; create a child deadline only for positive Timeout; describe immediately; poll only `RUNNING`; allowlist terminal statuses; return last `Execution` plus status-specific errors for failed/timed-out/aborted; fail closed on unknown status; use `time.NewTimer` + `select`; parent cancellation wins; check after each response; never stop/retry implicitly or publish late success.
- [ ] **Step 4: Run GREEN race proof.** `gofmt -w workflow/stepfunctions/*.go && go test -count=1 ./workflow/stepfunctions && go test -race -count=1 ./workflow/stepfunctions`; all wait/backoff/cancellation tests and race detector must pass.

## Task 4: Fake isolation, example, redaction

**Files:** `workflow/stepfunctions/bridge_test.go`, `example_test.go`.

- [ ] **Step 1: Harden fake.** Deep-copy requests, return fresh outputs, record contexts and logical calls, support blocking/cancellation and output-plus-error, and run concurrent distinct requests to prove isolation and no caller-slice retention.
- [ ] **Step 2: Add `ExampleNew`.** Construct fake, call `New`, `Start`, and bounded `WaitOptions`; compile only, no credentials/network/live AWS.
- [ ] **Step 3: Redaction tests.** Inject provider errors containing credential-like text, payload, ARN, and message; assert neither `Error()` nor `%+v` contains them while `errors.Is` matches the cause.
- [ ] **Step 4: Run examples/race.** `go test -run '^Example' -count=1 ./workflow/stepfunctions && go test -race -count=1 ./workflow/stepfunctions`; expect PASS.

## Task 5: Documentation and locale parity

**Files:** package README EN/KO, `workflow/README.md`, `workflow/README.ko.md`, root `README.md`, root `README.ko.md`.

- [ ] **Step 1: Write package READMEs.** Document import, interfaces, limits, STANDARD idempotency, EXPRESS/Describe/Stop limits, eventual consistency, status/errors, 1s/30s polling defaults, timeout/cancellation precedence, no implicit stop/retry, fake-only CI, and caller/operator ownership. Keep code/tables/links semantically aligned.
- [ ] **Step 2: Register indexes.** Add matching package links to workflow and root indexes; do not claim provisioning or live AWS support.
- [ ] **Step 3: Read back parity.** Run `git diff --check`, compare EN/KO headings/tables/code/limits/URLs manually, and record the parity matrix in the workflow evidence.

## Task 6: Spec/plan verification and repository checks

**Files:** `docs/superpowers/plans/2026-09-03-issue-522-step-functions-plan.md`.

- [ ] **Step 1: Run checks sequentially.** Execute `git diff --check`; targeted normal/race/example/vet; then `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, and `make ci`. Heavy suites remain serialized; diagnose and record any first failure before retry.
- [ ] **Step 2: Verify traceability.** Read spec and current diff; map narrow API, bounds, response mapping, statuses, timeout/backoff, cancellation, no provisioning, fake-only CI, docs, example, and race proof to exact files and fresh commands; check only proved rows.
- [ ] **Step 3: Record GO-HARD-08.** Implementation review/lesson must cite explicit timeout/deadline ownership, cancellable timer, capped backoff, terminal allowlist/unknown policy, no implicit Stop/retry, response-boundary cancellation, fake sequence, and race/resource result.

## Task 7: Step 6-R review and Lore commit

**Files:** `docs/review/2026-09-03-issue-522-step-functions-implementation-review.md`, `docs/lessons/2026-09-03-issue-522-step-functions.md`.

- [ ] **Step 1: Review exact diff.** Perform six read-only lenses (performance, stability, security, operator/Ops, developer/API, user/caller) plus main-session integration; report `file:line` findings as P0/P1/P2/P3. P0/P1 must be zero.
- [ ] **Step 2: Apply writer SPW-01..05.** Review records exact head/baseline/commands, no-live-AWS scope, GO-HARD-08 evidence, gaps, and verdict. Lesson records reusable failed assumption/evidence/decision/future guard, not a diary.
- [ ] **Step 3: Commit.** Use Korean Lore trailers (`Constraint`, `Rejected`, `Confidence`, `Scope-risk`, `Directive`, `Tested`, `Not-tested`) and include only #522 files.

## Task 8: PR, CI, merge, cleanup

- [ ] **Step 1: Publish/create PR.** After CG-11/12/12A, push `feat/issue-522-step-functions`, re-read guidance/issue metadata, create Korean PR against `develop`, assign `debop`, mirror milestone/labels, and end body with `## DoD Status`.
- [ ] **Step 2: Post-PR review/CI.** Read live diff/reviews/threads, wait for required `ci` on exact head, prove no skipped required jobs and P0/P1=0, and update PR DoD with run/head; solo human review is documented N/A only.
- [ ] **Step 3: Merge-ready report.** Render `Required checks: X/Y; N/A: N; Blocked: 0`, exact PR/head, CI/review/lesson/docs/GO-HARD-08 evidence and pending CG-16–18; stop for fresh approval.
- [ ] **Step 4: Merge after fresh approval.** Use only approved strategy with `--match-head-commit`; verify merged state, merge SHA, issue closure, and merge tree.
- [ ] **Step 5: Sync/cleanup.** Fast-forward local `develop`, verify local/upstream equality and clean/preserved status, remove only proven merged #522 worktree/local branch, prune stale refs, and preserve unrelated state.

## Rollback and stop conditions

- Before PR, revert only issue #522 branch commits if dependency/API validation fails; never alter `develop` or existing package behavior.
- P0/P1, race, cancellation leak, malformed-response ambiguity, or scope drift returns to the affected task and reruns dependents.
- Stale head, failed/skipped CI, unresolved review, missing metadata, or absent fresh merge approval is `PENDING`; do not auto-merge.
- Stop only after all applicable plan/common/type-A/Go checks PASS, P0=0/P1=0, exact-head CI is green, fresh approval is recorded, merge/sync/cleanup is read back, and no required item is unchecked.

## Plan self-review and writer gate

- Spec coverage: issue acceptance themes map to Tasks 1–6; GO-HARD-08 maps to Tasks 3, 6, 7; docs/lesson/PR/merge map to Tasks 5, 7, 8.
- Placeholder/type scan: no unresolved marker or undefined type/function appears; all public types are defined in Task 1 and reused unchanged.
- SPW-01..05: PASS — audience/source/side effects, complete ordered contract, Korean technical register, spec-to-task traceability, and rendered read-back are recorded here.
