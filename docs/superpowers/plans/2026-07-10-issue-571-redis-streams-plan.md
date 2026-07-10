# Issue #571 Redis Streams Primitive Implementation Plan

> **For agentic workers:** Execute this plan task-by-task with TDD. Do not
> combine the primitive with a consumer loop, retry worker, generic Redis
> facade, or audit payload model.

**Goal:** Add a small Go-native Redis Streams command primitive and migrate the
existing SQL outbox Redis Streams publisher to its append contract without
changing provider payload or delivery policy.

**Architecture:** `redis/stream` owns argument validation, nil/typed-nil
client checks, context preflight, and sanitized `btredis.OpError` conversion.
Every exported function maps to one caller-requested Redis Streams command and
returns existing `go-redis` values. The primitive owns no client, goroutine,
consumer loop, retry, or topology. `audit/sqloutbox/redisstreams` retains its
record-to-fields conversion and delegates only `XADD` dispatch to `Append`.

**Tech Stack:** Go 1.26, `github.com/redis/go-redis/v9`, existing `redis`
foundation, Redis Testcontainers, `testing/concurrency.GoroutineStressTester`.

## File Map

| File | Responsibility |
|---|---|
| `redis/stream/doc.go` | Package documentation and public semantic boundary. |
| `redis/stream/stream.go` | Narrow command interfaces and direct helpers. |
| `redis/stream/validation.go` | Shared non-mutating argument, context, and typed-nil validation helpers. |
| `redis/stream/stream_test.go` | Unit validation, preservation, dispatch error, and return-value tests. |
| `redis/stream/stream_integration_test.go` | Serial Redis Testcontainers command, cancellation, and concurrency coverage. |
| `redis/stream/example_test.go` | Compile-checked caller-owned public usage example. |
| `redis/stream/README.md` | English operations guide and diagram link. |
| `redis/stream/README.ko.md` | Korean parity operations guide and diagram link. |
| `docs/images/readme-diagrams/redis-streams-consumer-lifecycle.svg` | Source-backed sequence lifecycle visual. |
| `docs/images/readme-diagrams/redis-streams-consumer-lifecycle.png` | Rendered PNG paired with the SVG. |
| `redis/README.md`, `redis/README.ko.md` | Link the public stream subpackage and clarify its boundary. |
| `README.md`, `README.ko.md` | Add the public `redis/stream` package inventory link. |
| `audit/sqloutbox/redisstreams/publisher.go` | Delegate append dispatch to `redisstream.Append`. |
| `audit/sqloutbox/redisstreams/publisher_test.go` | Preserve provider public and error contracts under the shared append path. |
| `docs/review/2026-07-10-issue-571-redis-streams-review.md` | 7-tier implementation review and evidence. |
| `docs/lessons/2026-07-10-issue-571-redis-streams.md` | Durable Streams delivery/error/redaction lesson. |

## Task 1: Lock the Command Contract With Unit Tests

**Files:** create `redis/stream/stream_test.go`

- [ ] Add failing package tests for `Append`, `Read`, `CreateGroup`,
  `ReadGroup`, `Acknowledge`, `Pending`, `AutoClaim`, `TrimMaxLen`,
  `TrimMinID`, and `Delete` using command-specific fakes.
- [ ] Verify blank names, nil/typed-nil values, missing IDs, malformed stream arrays,
  invalid trim length, nil clients, and typed-nil clients match
  `redisstream.ErrInvalidArgument` without dispatching a fake command.
- [ ] Verify all helper inputs preserve caller-owned names/IDs and never mutate
  caller `X*Args`, stream slices, or payload maps.
- [ ] Verify `nil` contexts normalize to usable background contexts; canceled
  and deadline-expired contexts return their original error before dispatch.
- [ ] Make fake dispatched errors assert both `errors.Is` for the original
  cause and `errors.As` for `*btredis.OpError`; formatted error strings must
  omit the raw stream key and injected provider text.
- [ ] Run the focused unit test before implementation; expected result is a
  compile failure because `redis/stream` does not yet exist.

## Task 2: Implement the Stateless Primitive

**Files:** create `redis/stream/{doc.go,stream.go,validation.go}`

- [ ] Declare `package redisstream`, `ErrInvalidArgument`, narrow interfaces,
  and the exported helper signatures specified in the approved spec.
- [ ] Implement a single context preflight helper: nil becomes
  `context.Background()`; an already-done context returns directly before any
  client invocation.
- [ ] Implement typed-nil interface detection without panics; keep it private
  and reuse it for every narrow client interface.
- [ ] Validate only structural requirements. Use trimmed values solely to
  determine blankness; send original names, IDs, fields, values, flags, and
  durations unchanged to go-redis.
- [ ] For `XRead` and `XReadGroup`, validate go-redis ordering as an even list
  of all stream keys followed by all IDs. Validate only the key half as stream
  keys; do not constrain valid IDs such as `0`, `$`, or `>`.
- [ ] Copy argument structs and stream slices before dispatch. Do not deep-copy
  or encode `Values`; it remains caller-owned arbitrary payload data.
- [ ] Wrap only dispatched Redis failures in `btredis.NewOpError` with family
  `redis stream`, listed operation labels, and a deterministic length-delimited
  ordered-stream correlation key. Direct validation and preflight context
  errors stay unwrapped.
- [ ] Use `XAUTOCLAIM` only. Return its messages and next cursor exactly so the
  caller owns recovery progress.
- [ ] Format and run `go test -p 1 -count=1 ./redis/stream`.

## Task 3: Prove Redis Semantics And Concurrent Call Safety

**Files:** create `redis/stream/stream_integration_test.go`

- [ ] Build a package-local Redis Testcontainers fixture with a two-minute
  context, test-name-derived stream prefix, bounded cleanup, and closed client.
- [ ] Add append/read coverage that verifies returned IDs and caller-provided
  values; include optional caller-selected `XAddArgs` max-length trimming.
- [ ] Add consumer group coverage: `CreateGroup`, `ReadGroup` with `>`,
  `Pending`, and `Acknowledge`; assert pending exists before ack and is absent
  after ack.
- [ ] Add `AutoClaim` coverage with consumer A leaving work pending and consumer
  B receiving it through a caller-chosen idle threshold/cursor. Avoid a
  production-scale sleep; use a Redis-supported small/zero test threshold.
- [ ] Add explicit trim and delete cases. Assert these commands return Redis
  counts and no append/read operation performs retention implicitly.
- [ ] Add a blocked `Read` with a Redis block longer than a short context
  timeout. Assert `errors.Is(err, context.DeadlineExceeded)` and `errors.As`
  to `*btredis.OpError` after dispatched cancellation; do not assert exact
  elapsed time.
- [ ] Add bounded unique-message concurrent append stress using
  `concurrencytest.NewGoroutineStressTester`. Assert every task returns an ID
  and the final stream count equals the task count.
- [ ] Run `go test -p 1 -count=1 ./redis/stream` and
  `go test -p 1 -race -count=1 ./redis/stream`.

## Task 4: Migrate Only the SQL Outbox Append Dispatch

**Files:** modify `audit/sqloutbox/redisstreams/{publisher.go,publisher_test.go}`

- [ ] Replace the private `XAdd` dispatch path with `redisstream.Append` using
  the identical `redis.XAddArgs{Stream: p.stream, Values: values}`.
- [ ] Keep `Options`, `Publisher`, `Stream`, default stream, field encoding,
  cancellation preflight, and duplicate-attempt behavior unchanged.
- [ ] Make the provider `Client` compile-time compatible with
  `redisstream.Appender`; do not expose unrelated Redis operations.
- [ ] Keep the established `redis streams publish` error boundary while
  preserving `errors.Is`/`errors.As` for the primitive's typed cause and its
  sanitized key diagnostics.
- [ ] Add/adjust fake client tests for successful append, raw-key redaction,
  typed cause propagation, and zero calls after pre-canceled context.
- [ ] Run normal and race tests for `./audit/sqloutbox/redisstreams`.

## Task 5: Public Documentation, Example, And Diagram

**Files:** create package README/doc/example and the SVG/PNG; modify Redis/root
README locale pairs.

- [ ] Write English/Korean package guides with caller-owned timeout examples,
  at-least-once duplicate guidance, pending/ack behavior, replay/`XAUTOCLAIM`,
  explicit trim/delete retention risk, consumer shutdown, cancellation
  ambiguity, and sanitized error/runbook guidance.
- [ ] Add a compile-checked `Example` that uses a caller-owned Redis client and
  timeout; it must not model a package-managed loop or claim exactly-once.
- [ ] Create `redis-streams-consumer-lifecycle.svg` using the sequence diagram
  contract. Read `docs/images/readme-diagrams/redis-lock-owner-token-lifecycle.png`
  and one other approved sequence PNG at full size before drawing. Include
  numbered messages, participant headers/lifelines/activations, a transparent
  recovery branch, and explicit caller-owned ack/replay/trim labels.
- [ ] Render its PNG with CairoSVG, inspect the full-size PNG, and run XML,
  connector, geometry, endpoint, mixed-corner, and sequence-style audits.
  Record concrete counts and results in the implementation review.
- [ ] Link the subpackage in `redis` README locale pairs and `redis/stream` in
  root README locale pairs. Keep English/Korean public behavior in parity.
- [ ] Run `go test -count=1 ./redis/stream -run Example` and `git diff --check`.

## Task 6: Full Verification, Review, And Publication

**Files:** create review/lesson artifacts; update PR metadata after commit.

- [ ] Run focused normal/race tests for both changed packages, then
  `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`,
  `make race`, and `make ci`.
- [ ] Perform the mandatory six-perspective 7-Tier implementation review over
  `origin/develop...HEAD`: performance, stability, security, operator/Ops,
  developer/API, and user/caller. The main session owns the integration
  verdict. Resolve every P0/P1 before PR publication.
- [ ] Write the durable lesson covering why a provider may share a command
  primitive without leaking its domain envelope or hiding at-least-once policy.
- [ ] Commit with a Lore-protocol message. Create a PR closing #571 with the
  issue's assignee, milestone, and labels. The PR body must end in
  `## DoD Status` and include targeted/final verification plus the explicit
  #560 benchmark N/A rationale.
- [ ] Wait for CI. On success, report exact run evidence; do not merge unless
  the user explicitly asks to merge.

## Rollback

Revert the #571 commit. `audit/sqloutbox/redisstreams` returns to its direct
`XADD` dispatch and no Redis key/data migration is required because this issue
creates no package-owned topology or background state.
