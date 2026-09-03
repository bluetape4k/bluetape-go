# EventBridge 감사 publisher adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `audit/sqloutbox.Publisher` 계약을 보존하는 fake-first EventBridge `PutEvents` adapter를 추가하고, stable audit identity·partial failure·cancellation·redaction을 문서와 검증 증거로 고정한다.

**Architecture:** 새 `audit/sqloutbox/eventbridge` child package가 caller-owned AWS SDK v2 client의 `PutEvents` subset만 의존한다. `Publisher`는 생성 뒤 immutable하고 한 `Publish`마다 검증된 outbox record를 bounded JSON detail의 단일 EventBridge entry로 보내며, relay의 retry/dead-letter와 AWS topology 소유권은 건드리지 않는다. 기존 Redis Streams envelope field 이름과 Korean/English 문서 경계를 재사용하되 Kotlin 코드는 기계적으로 포팅하지 않는다.

**Tech Stack:** Go `1.26.3`, AWS SDK for Go v2 `service/eventbridge v1.47.0`, `encoding/json`, `context`, standard-library errors/reflection/testing, `go test -race`, repository Make targets.

---

## 파일 구조와 책임

| 경로 | 책임 |
|---|---|
| `go.mod`, `go.sum` | root SDK `v1.42.1`과 호환되는 EventBridge service module direct requirement만 추가 |
| `audit/sqloutbox/eventbridge/doc.go` | package 목적, caller-owned client/topology, stable identity와 cancellation의 Go doc |
| `audit/sqloutbox/eventbridge/errors.go` | sentinel, safe typed `Error`, operation allowlist, `errors.Is`/failure accessors |
| `audit/sqloutbox/eventbridge/publisher.go` | `Client`, `Options`, immutable `Publisher`, typed-nil/field validation, detail envelope, `Publish` request/response mapping |
| `audit/sqloutbox/eventbridge/publisher_test.go` | mutex-safe fake, constructor/record/size/cancellation/success/failure/redaction/concurrency/relay contract tests |
| `audit/sqloutbox/eventbridge/example_test.go` | compile-checked caller-owned client construction and `sqloutbox.Publisher` usage example |
| `audit/sqloutbox/eventbridge/README.md` | English API, IAM/topology/idempotency/rollout boundary |
| `audit/sqloutbox/eventbridge/README.ko.md` | 같은 정보의 Korean locale pair |
| `audit/sqloutbox/README.md`, `README.ko.md` | parent outbox package에서 EventBridge adapter와 single-entry semantics 링크 |
| `docs/superpowers/specs/2026-09-03-issue-520-eventbridge-design.md` | 승인 설계와 source ledger (완료) |
| `docs/superpowers/plans/2026-09-03-issue-520-eventbridge-plan.md` | 이 실행 순서 |
| `docs/review/2026-09-03-issue-520-eventbridge-spec-review.md` | Step 2-R (완료) |
| `docs/review/2026-09-03-issue-520-eventbridge-plan-review.md` | Step 3-R six-lens + integration 결과 |
| `docs/review/2026-09-03-issue-520-eventbridge-risk-prediction.md` | external API, size, cancellation, redaction risk와 mitigation |
| `docs/review/2026-09-03-issue-520-eventbridge-implementation-review.md` | Step 6-R 7-Tier 결과와 fresh test evidence |
| `docs/lessons/2026-09-03-issue-520-eventbridge.md` | 재사용 가능한 Go/AWS outbox 교훈과 유예 범위 |

## Task 1: dependency와 plan gate 고정

**Files:**

- Modify: `go.mod`, `go.sum`
- Create: `docs/superpowers/plans/2026-09-03-issue-520-eventbridge-plan.md`
- Create: `docs/review/2026-09-03-issue-520-eventbridge-plan-review.md`
- Create: `docs/review/2026-09-03-issue-520-eventbridge-risk-prediction.md`

- [x] **Step 1: 계획 self-review와 미완성 표기 scan**

  Spec의 목표, 비목표, API, detail, failure, test, docs 섹션마다 아래 task의
  정확한 파일과 명령이 대응하는지 확인한다. 미완성 지시나 무정의 대체
  표현이 없는지 검색하고, API의 `Options`, `Publisher.Publish`, sentinel
  이름은 이후 task와 일치해야 한다.

- [x] **Step 2: 호환 module만 추가**

  `go.mod`에 다음 한 줄을 direct requirement로 추가하고, 다른 AWS root module은
  올리지 않는다.

  ```go
  github.com/aws/aws-sdk-go-v2/service/eventbridge v1.47.0
  ```

  Run: `go mod tidy`

  Expected: `go.mod`의 root `github.com/aws/aws-sdk-go-v2 v1.42.1`과
  smithy/internal modules가 불필요하게 바뀌지 않고 `go.sum`에 필요한
  checksums만 추가된다. `go list -m all | rg 'aws-sdk-go-v2($|/service/eventbridge)'`로
  `v1.42.1`/`v1.47.0` 조합을 확인한다.

- [x] **Step 3: Step 3-R 계획 review 작성**

  여섯 관점별 P0/P1을 0으로 검토하고, 다음 위험을 계획에 반영한다.

  - Performance: single entry와 encoded detail/entry size preflight가 AWS 256 KB 미만을 보장하는지
  - Stability: context preflight/dispatch/post-response 우선순위와 malformed response 처리
  - Security: raw `ErrorMessage`, detail, credentials, bus/source가 `Error()`/`%+v`/log에 들어가지 않는지
  - Operator/Ops: default bus, topology/IAM/client lifecycle/downstream idempotency 소유권
  - Developer/API: compile assertion, typed-nil, immutable options, exact strings, `errors.Is`
  - User/Caller: Redis envelope field parity, Go docs, EN/KO parity, relay semantics

  Create `docs/review/2026-09-03-issue-520-eventbridge-plan-review.md` with
  exact planned command/test evidence;
  no P0/P1 finding may remain unresolved before Task 2.

- [x] **Step 4: external-risk prediction 기록**

  Create `docs/review/2026-09-03-issue-520-eventbridge-risk-prediction.md` with
  probability/impact/mitigation rows for SDK response shape drift, entry-size
  overhead, cancellation after network return, response error leakage, fake
  aliasing, and dependency churn. Each row names a concrete test or code guard.

- [x] **Step 5: plan gate 검증**

  Run: `git diff --check && go mod tidy && go list -m github.com/aws/aws-sdk-go-v2/service/eventbridge`

  Expected: diff check PASS, module resolves exactly `v1.47.0`, and no package
  implementation has been written yet. Commit plan/review/risk artifacts with
  Lore trailers before implementation.

## Task 2: public shape와 RED tests 작성

**Files:**

- Create: `audit/sqloutbox/eventbridge/doc.go`
- Create: `audit/sqloutbox/eventbridge/publisher_test.go`
- Create: `audit/sqloutbox/eventbridge/example_test.go`

- [x] **Step 1: package docs와 compile surface 선언**

  `doc.go`는 Korean Go doc으로 caller-owned client, no live AWS, stable detail
  identity, cancellation/retry boundary를 설명한다. Test file의 첫 선언은 다음
  compile contract를 고정한다.

  ```go
  var _ sqloutbox.Publisher = (*Publisher)(nil)
  var _ Client = (*awseventbridge.Client)(nil)
  ```

- [x] **Step 2: mutex-safe fake 작성**

  `fakeClient`는 `sync.Mutex`로 `calls`, deep-copied `lastInput`, configured
  `output`, `err`, `entered` channel과 `release` channel을 보호한다. `PutEvents`
  구현은 호출 context가 취소되면 해당 context error를 반환하고, blocking
  mode에서는 `<-release` 또는 `<-ctx.Done()`을 선택한다. `lastInput`은
  `Entries`, pointer string/time, `Detail` bytes를 복사하여 publisher가
  request를 재사용해도 fake observation이 변하지 않게 한다.

- [x] **Step 3: constructor/shape RED tests 작성**

  다음 table cases를 먼저 작성한다. 아직 `Publisher` 구현이 없으므로 focused
  test는 compile 또는 undefined symbol로 RED여야 한다.

  ```go
  {name: "nil client", options: Options{}}
  {name: "typed nil client", options: Options{Client: (*fakeClient)(nil), Source: "app", DetailType: "audit"}}
  {name: "blank source", options: Options{Client: fake, Source: "  ", DetailType: "audit"}}
  {name: "blank detail type", options: Options{Client: fake, Source: "app", DetailType: "\t"}}
  {name: "invalid UTF-8", options: Options{Client: fake, Source: string([]byte{0xff}), DetailType: "audit"}}
  {name: "source over 256 bytes", options: Options{Client: fake, Source: strings.Repeat("x", 257), DetailType: "audit"}}
  {name: "detail type over 128 bytes", options: Options{Client: fake, Source: "app", DetailType: strings.Repeat("x", 129)}}
  {name: "negative detail limit", options: Options{Client: fake, Source: "app", DetailType: "audit", MaxDetailSize: -1}}
  ```

  Add success cases proving empty bus uses `EventBusName()==""`, custom bus is
  exact and untrimmed, default `MaxDetailSize==256<<10`, and accessors return
  immutable copied strings.

- [x] **Step 4: Publish RED matrix 작성**

  `testRecord` must create a valid `audit.Entry` whose record identity, revision,
  schema and both timestamps match. Add cases for success, record ID/attempts
  zero, entry validation failure, identity mismatch, oversized detail, canceled
  context, transport error, nil output, zero/one/two response entries,
  `FailedEntryCount`, per-entry error code/message, missing success `EventId`,
  and post-response cancellation.
  Every preflight failure asserts fake calls `==0`.

  Run: `go test ./audit/sqloutbox/eventbridge -run 'Test(New|Publish)' -count=1`

  Expected: RED with missing implementation symbols, not environment/Testcontainers
  failures. Record this command and exact first failure in the plan review notes.

- [x] **Step 5: compile-checked example RED 작성**

  `ExampleNew` uses a fake implementing only `PutEvents`, passes `Options{Client,
  Source:"com.example.billing", DetailType:"InvoicePaid"}`, assigns the result
  to `var publisher sqloutbox.Publisher`, and calls `Publish`. It must not load
  AWS config or create a client; `// Output:` contains only deterministic success
  text from the fake. Run the example focused test and keep it RED until Task 3.

## Task 3: safe errors와 bounded envelope 구현

**Files:**

- Create: `audit/sqloutbox/eventbridge/errors.go`
- Modify: `audit/sqloutbox/eventbridge/publisher_test.go`

- [x] **Step 1: sentinel과 typed Error 구현**

  Export the exact sentinels from the spec:

  ```go
  var (
      ErrNilClient = errors.New("eventbridge: client must not be nil")
      ErrInvalidOptions = errors.New("eventbridge: invalid options")
      ErrInvalidRecord = errors.New("eventbridge: invalid outbox record")
      ErrDetailTooLarge = errors.New("eventbridge: detail exceeds limit")
      ErrPublishFailed = errors.New("eventbridge: publish failed")
      ErrPartialFailure = errors.New("eventbridge: entry failure")
      ErrMalformedOutput = errors.New("eventbridge: malformed response")
  )
  ```

  `Error` fields are unexported except read-only methods `FailureCount() int32`
  and `ErrorCode() string`. `Error()` uses only a safe sentinel, an allowlisted
  operation (`publish`, `marshal detail`, `validate record`, `validate options`)
  and an AWS error-code token matching `[A-Za-z0-9._-]{1,64}`; arbitrary result
  messages are discarded. `Unwrap` returns a sanitized sentinel/cause and `Is`
  matches both package sentinel and transport cause without formatting it.

- [x] **Step 2: redaction tests를 먼저 통과시키기**

  Inject `errors.New("AWS secret detail: customer-42")` as transport cause and
  `ErrorMessage="raw customer-42 credentials"` as entry result. Assert
  `errors.Is(err, cause)`, `errors.Is(err, ErrPublishFailed/ErrPartialFailure)`,
  `errors.As(*Error)`, and that `err.Error()` and `fmt.Sprintf("%+v", err)` contain
  none of `customer-42`, `credentials`, source, bus, or JSON detail. Assert safe
  code/count accessors are stable.

- [x] **Step 3: run error-focused tests**

  Run: `go test ./audit/sqloutbox/eventbridge -run 'TestPublish.*(Error|Failure|Redaction)|TestError' -count=1`

  Expected: PASS for sentinel, `errors.Is`/`errors.As`, code/count, and no secret
  leakage. A failure must be fixed before request mapping.

## Task 4: EventBridge publisher GREEN implementation

**Files:**

- Create: `audit/sqloutbox/eventbridge/publisher.go`
- Modify: `audit/sqloutbox/eventbridge/publisher_test.go`, `example_test.go`

- [x] **Step 1: define constants and immutable options**

  Use `defaultMaxDetailSize = 256 << 10`, `maxEventEntrySize = 256 << 10`,
  `maxSourceBytes=256`, `maxDetailTypeBytes=128`, `maxEventBusNameBytes=256`.
  `New` normalizes only nil detection and default size; it preserves nonblank
  strings exactly. It rejects typed-nil client using the same nil-capable
  reflection kinds as `redisstreams` and stores no caller-owned mutable map/slice.

- [x] **Step 2: validate record and construct envelope**

  Create an unexported `detailEnvelope` matching Redis field names:

  ```go
  type detailEnvelope struct {
      RecordID int64 `json:"record_id"`
      Status string `json:"status"`
      AggregateType string `json:"aggregate_type"`
      AggregateID string `json:"aggregate_id"`
      Revision uint64 `json:"revision"`
      EventID string `json:"event_id"`
      IdempotencyKey string `json:"idempotency_key"`
      EventType string `json:"event_type"`
      OccurredAt string `json:"occurred_at"`
      RecordedAt string `json:"recorded_at"`
      SchemaVersion int `json:"schema_version"`
      Attempts int `json:"attempts"`
      EntryJSON json.RawMessage `json:"entry_json"`
  }
  ```

  Validate positive ID/attempts, `record.Entry.Validate()`, exact record↔entry
  aggregate/revision/event ID/idempotency/event type/schema/timestamp equality,
  and raw JSON/string size before allocation. Marshal `record.Entry`, then marshal
  the envelope. Use UTC RFC3339Nano strings as `redisstreams.messageValues` does. Reject `len(detail)` over configured cap
  or `len(detail)+len(source)+len(detailType)+len(eventBusName) >= 256<<10`.

- [x] **Step 3: implement Publish context and request mapping**

  Normalize nil context to `context.Background()`, check `ctx.Err()` before
  validation and again immediately before `PutEvents`. Build exactly one
  `types.PutEventsRequestEntry`:

  ```go
  entry := awseventbridgetypes.PutEventsRequestEntry{
      Detail: &detail,
      DetailType: &p.detailType,
      Source: &p.source,
      Time: &record.OccurredAt,
  }
  if p.eventBusName != "" { entry.EventBusName = &p.eventBusName }
  output, err := p.client.PutEvents(ctx, &awseventbridge.PutEventsInput{Entries: []awseventbridgetypes.PutEventsRequestEntry{entry}})
  ```

  Do not add retries, goroutines, options mutation, logger calls, resources,
  endpoint IDs, or EventBridge response EventId handling.

- [x] **Step 4: implement response mapping and cancellation priority**

  After the SDK returns, check `ctx.Err()` first. Then require non-nil output and
  exactly one result entry. If transport `err` is non-nil, return `Error{kind:
  ErrPublishFailed, operation:"publish", cause:err}`. If output shape is wrong,
  return `ErrMalformedOutput`. If `FailedEntryCount>0` or result `ErrorCode`/
  `ErrorMessage` is nonempty, return `Error{kind: ErrPartialFailure,
  operation:"publish", failureCount: max(failedCount,1),
  code: safeCode(result.ErrorCode)}`. Only zero count, empty code/message, and a
  nonblank success `EventId` returns nil. Never copy `ErrorMessage` into an error.

- [x] **Step 5: run targeted GREEN and race tests**

  Run sequentially:

  ```bash
  gofmt -w audit/sqloutbox/eventbridge/*.go
  go test -count=1 ./audit/sqloutbox/eventbridge
  go test -race -count=1 ./audit/sqloutbox/eventbridge
  go vet ./audit/sqloutbox/eventbridge
  ```

  Expected: all constructor, envelope, request, response, cancellation,
  redaction, fake isolation, example, and concurrent tests PASS; vet reports no
  diagnostics. If a test fails, keep the smallest failing case and fix code
  before broad tests.

## Task 5: documentation and caller contract

**Files:**

- Create: `audit/sqloutbox/eventbridge/README.md`
- Create: `audit/sqloutbox/eventbridge/README.ko.md`
- Modify: `audit/sqloutbox/README.md`, `audit/sqloutbox/README.ko.md`
- Create: `docs/lessons/2026-09-03-issue-520-eventbridge.md`

- [x] **Step 1: write child README locale pair**

  Document the exact `New(Options{...})` shape, default/custom bus behavior,
  single-entry detail field table, stable identity, `FailedEntryCount` and
  per-entry error mapping, cancellation/no retry semantics, size preflight,
  safe errors, fake-only CI, and compile-checked example. State explicitly that
  caller/operator owns AWS config/credentials/client lifecycle, bus/rule/target
  provisioning, IAM, timeout/retry policy, downstream idempotency and rollout.
  Explain EventBridge response `EventId` is not outbox identity. Keep paragraphs
  semantically identical in English and Korean; retain code/commands/URLs.

- [x] **Step 2: link parent README pair**

  Add an EventBridge subsection/link under existing sqloutbox publisher choices.
  Do not change relay transaction or Redis Streams behavior. Mention no new
  diagram is needed because the existing relay→publisher boundary answers the
  topology question.

- [x] **Step 3: record lesson and docs review**

  Lesson records: narrow SDK subset, AWS partial result handling, exact detail
  size accounting, error redaction, and why Kinesis/#522 remain separate. Run
  `git diff --check` and a locale parity check that compares headings/code block
  identifiers and required terms (`EventBusName`, `FailedEntryCount`,
  `idempotency_key`, `context`, `PutEvents`).

## Task 6: Step 6-R, full verification, and workflow evidence

**Files:**

- Create: `docs/review/2026-09-03-issue-520-eventbridge-implementation-review.md`
- Modify: all implementation/docs files above as required by review
- Modify: `.omx/issue-520-*.json` transient evidence files (ignored)

- [ ] **Step 1: run focused and package verification**

  Run:

  ```bash
  make fmt-check
  make tidy-check
  go vet ./audit/sqloutbox/eventbridge
  go test -count=1 ./audit/sqloutbox/eventbridge
  go test -race -count=1 ./audit/sqloutbox/eventbridge
  git diff --check origin/develop...HEAD
  ```

  Capture command, exact commit SHA, package list, and test counts. Confirm no
  live AWS endpoint/config/credential access appears in tests.

- [ ] **Step 2: perform six-lens + main integration Step 6-R**

  Review exact diff for performance, stability, security, operator/Ops,
  developer/API, and user/caller. Main integration checks scope against #520,
  unchanged `sqloutbox` semantics, EN/KO parity, module diff, generated sums,
  error redaction, and rollback. Record file/line evidence and P0/P1/P2/P3
  separately in `docs/review/2026-09-03-issue-520-eventbridge-implementation-review.md`.
  Any P0/P1 is fixed and re-reviewed before PR; unavailable external review is
  recorded as `PENDING`/main-session fallback, never as clean pass.

- [ ] **Step 3: run repository gates**

  Run from the feature worktree:

  ```bash
  make vet
  make lint
  make test
  make race
  make ci
  ```

  Existing Testcontainers failures must be diagnosed and reported separately;
  changed package normal/race tests cannot be replaced by a skipped check.
  Preserve exact logs and environment in the review artifact. Remote PR CI is a
  separate required gate.

- [ ] **Step 4: attach workflow check/component evidence**

  Use `bluetape-flow.py` with run `20260903T024720Z-33c30024` and owner file
  `.bluetape/handles/issue-520-owner` to record `requirements`, `design`,
  `plan`, `red-green`, `targeted-tests`, `full-ci`, `pre-pr-review`, and later
  `pr-ci` check results. Attach `issue-520` implementation evidence and
  `docs-issue-520` locale/review evidence only after lane completion and all
  required checks pass. Run `verify --run-id` after each material transition.

- [ ] **Step 5: complete and commit**

  Update this plan checkboxes, spec SPW-04/05, implementation review, lesson,
  and workflow evidence. Commit with Lore trailers; the intent line must explain
  why the adapter is safe/compatible, not merely list files. Run
  `git status --short`, `git diff --check`, and `git log -1 --format=%B` before PR.

## Task 7: PR, exact-head CI, merge, sync, cleanup

**Files/state:** feature branch `feat/issue-520-eventbridge-audit`, base `develop`,
PR body and live GitHub metadata.

- [ ] **Step 1: pre-PR exact-head review**

  Re-read `git rev-parse HEAD`, `git diff --stat origin/develop...HEAD`, all
  checks and PR body inputs. Confirm issue #520 title/labels/milestone/assignee
  remain live and no duplicate PR exists. PR body is Korean, closes #520,
  links #517/#512/research, lists changed package/tests/docs, and ends with
  `## DoD Status` containing PASS/PENDING items.

- [ ] **Step 2: create PR and wait for remote CI**

  Push the semantic branch and create the PR only after local gates pass. Poll
  `gh pr checks --watch` and inspect terminal job logs, required matrix, test
  counts, and exact PR head SHA. A green headline or skipped job is not sufficient.
  Record remote evidence in implementation review and workflow `pr-ci` check.

- [ ] **Step 3: fresh merge approval gate**

  Before merge, re-read PR base/head/mergeability, review threads, PR body,
  required checks and exact `headRefOid`. Ask for fresh explicit user approval
  immediately before the irreversible squash merge; never enable auto-merge.

- [ ] **Step 4: exact-head squash merge and live verification**

  Use `gh pr merge --squash --match-head-commit <exact-head>` only after fresh
  approval. Verify PR merged, issue #520 closed, merge commit/tree contains the
  intended files, and `develop` remote points at the observed merge result.

- [ ] **Step 5: local sync and preservation-first cleanup**

  Fetch and fast-forward the main worktree `develop` to the verified remote
  merge. Re-check unrelated dirty/untracked state before deleting only the
  #520 feature worktree; delete the local feature branch only after worktree
  removal and explicit branch-cleanup authority. Do not remove `.worktrees`,
  other worktrees, credentials, or ignored runtime state broadly.

- [ ] **Step 6: final #517 DoD report**

  Report Korean DoD in required order: status/evidence first; user actions
  (none unless merge approval or external gate remains); next slice (#522/#523
  after #520, then #524/#538/#539, then #525; #521 remains deferred). Include
  original five-child status, #520 PR/merge SHA, tests/CI, known gaps, and
  `DONE`/`PENDING`/`BLOCKED` with unchecked items.
