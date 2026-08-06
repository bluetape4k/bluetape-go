# Issue #571 Redis Streams primitive 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 실행 this plan task-by-task 함께 TDD. 하지 않는다
> combine the primitive 함께 a consumer loop, retry worker, generic Redis
> facade, 또는 audit payload model.

**목표:** 추가 a small Go-native Redis Streams command primitive 및 migrate the
기존 SQL outbox Redis Streams publisher to its append 계약 without
changing provider payload 또는 delivery policy.

**아키텍처:** `redis/stream` owns argument validation, nil/typed-nil
client checks, context preflight, 및 sanitized `btredis.OpError` conversion.
Every exported function maps to one 호출자-requested Redis Streams command 및
returns 기존 `go-redis` values. The primitive owns 없음 client, goroutine,
consumer loop, retry, 또는 topology. `audit/sqloutbox/redisstreams` retains its
record-to-fields conversion 및 delegates 만 `XADD` dispatch to `Append`.

**기술 스택:** Go 1.26, `github.com/redis/go-redis/v9`, 기존 `redis`
foundation, Redis Testcontainers, `testing/concurrency.GoroutineStressTester`.

## 파일 지도

| 파일 | 책임 |
|---|---|
| `redis/stream/doc.go` | Package documentation 및 공개 semantic boundary. |
| `redis/stream/stream.go` | Narrow command interfaces 및 direct helpers. |
| `redis/stream/validation.go` | Shared non-mutating argument, context, 및 typed-nil validation helpers. |
| `redis/stream/stream_test.go` | Unit validation, preservation, dispatch 오류, 및 return-value 테스트. |
| `redis/stream/stream_integration_test.go` | Serial Redis Testcontainers command, cancellation, 및 concurrency coverage. |
| `redis/stream/example_test.go` | Compile-checked 호출자-owned 공개 usage example. |
| `redis/stream/README.md` | 영문 operations guide 및 diagram link. |
| `redis/stream/README.ko.md` | 한국어 parity operations guide 및 diagram link. |
| `docs/images/readme-diagrams/redis-streams-consumer-lifecycle.svg` | Source-backed sequence lifecycle visual. |
| `docs/images/readme-diagrams/redis-streams-consumer-lifecycle.png` | Rendered PNG paired 함께 the SVG. |
| `redis/README.md`, `redis/README.ko.md` | Link the 공개 stream subpackage 및 clarify its boundary. |
| `README.md`, `README.ko.md` | 추가 the 공개 `redis/stream` 패키지 inventory link. |
| `audit/sqloutbox/redisstreams/publisher.go` | Delegate append dispatch to `redisstream.Append`. |
| `audit/sqloutbox/redisstreams/publisher_test.go` | 보존 provider 공개 및 오류 contracts under the 공유 append path. |
| `docs/review/2026-07-10-issue-571-redis-streams-review.md` | 7-tier implementation review 및 evidence. |
| `docs/lessons/2026-07-10-issue-571-redis-streams.md` | Durable Streams delivery/오류/redaction lesson. |

## 작업 1: 고정 the Command Contract With Unit Tests

**파일:** create `redis/stream/stream_test.go`

- [ ] 추가 failing 패키지 테스트 for `Append`, `Read`, `CreateGroup`,
  `ReadGroup`, `Acknowledge`, `Pending`, `AutoClaim`, `TrimMaxLen`,
  `TrimMinID`, 및 `Delete` using command-specific fakes.
- [ ] 검증 blank names, nil/typed-nil values, missing IDs, malformed stream arrays,
  invalid trim length, nil clients, 및 typed-nil clients match
  `redisstream.ErrInvalidArgument` without dispatching a fake command.
- [ ] 검증 모든 helper inputs preserve 호출자-owned names/IDs 및 never mutate
  호출자 `X*Args`, stream slices, 또는 payload maps.
- [ ] 검증 `nil` contexts normalize to usable background contexts; canceled
  및 deadline-expired contexts return their original 오류 전에 dispatch.
- [ ] Make fake dispatched 오류 assert both `errors.Is` for the original
  원인 및 `errors.As` for `*btredis.OpError`; formatted 오류 strings must
  omit the raw stream key 및 injected provider text.
- [ ] 실행 the focused unit 테스트 전에 implementation; 예상 결과 is a
  compile failure because `redis/stream` does 아님 yet exist.

## 작업 2: 구현 the Stateless Primitive

**파일:** create `redis/stream/{doc.go,stream.go,validation.go}`

- [ ] Declare `package redisstream`, `ErrInvalidArgument`, narrow interfaces,
  및 the exported helper signatures specified in the approved spec.
- [ ] 구현 a single context preflight helper: nil becomes
  `context.Background()`; an already-done context returns directly 전에 any
  client invocation.
- [ ] 구현 typed-nil interface detection without panics; keep it private
  및 reuse it for every narrow client interface.
- [ ] Validate 만 structural requirements. 사용 trimmed values solely to
  determine blankness; send original names, IDs, fields, values, flags, 및
  durations unchanged to go-redis.
- [ ] For `XRead` 및 `XReadGroup`, validate go-redis ordering as an even list
  of 모든 stream keys fol낮음ed by 모든 IDs. Validate 만 the key half as stream
  keys; do 아님 constrain valid IDs such as `0`, `$`, 또는 `>`.
- [ ] Copy argument structs 및 stream slices 전에 dispatch. 다음을 하지 않는다: deep-copy
  또는 encode `Values`; it remains 호출자-owned arbitrary payload data.
- [ ] Wrap 만 dispatched Redis failures in `btredis.NewOpError` 함께 family
  `redis stream`, listed operation labels, 및 a deterministic length-delimited
  ordered-stream correlation key. Direct validation 및 preflight context
  오류 stay unwrapped.
- [ ] 사용 `XAUTOCLAIM` 만. Return its messages 및 next cursor exactly so the
  호출자 owns recovery progress.
- [ ] Format 및 run `go test -p 1 -count=1 ./redis/stream`.

## 작업 3: 증명 Redis Semantics And Concurrent Call Safety

**파일:** create `redis/stream/stream_integration_test.go`

- [ ] 구성 a 패키지-local Redis Testcontainers fixture 함께 a two-minute
  context, 테스트-name-derived stream prefix, bounded cleanup, 및 closed client.
- [ ] 추가 append/read coverage that verifies returned IDs 및 호출자-provided
  values; include optional 호출자-selected `XAddArgs` max-length trimming.
- [ ] 추가 consumer group coverage: `CreateGroup`, `ReadGroup` 함께 `>`,
  `Pending`, 및 `Acknowledge`; assert pending exists 전에 ack 및 is absent
  후 ack.
- [ ] 추가 `AutoClaim` coverage 함께 consumer A leaving work pending 및 consumer
  B receiving it through a 호출자-chosen idle threshold/cursor. Avoid a
  production-scale sleep; use a Redis-supported small/zero 테스트 threshold.
- [ ] 추가 explicit trim 및 delete cases. 검증 these commands return Redis
  counts 및 없음 append/read operation performs retention implicitly.
- [ ] 추가 a blocked `Read` 함께 a Redis block longer than a short context
  timeout. 검증 `errors.Is(err, context.DeadlineExceeded)` 및 `errors.As`
  to `*btredis.OpError` 후 dispatched cancellation; do 아님 assert exact
  elapsed time.
- [ ] 추가 bounded unique-message concurrent append stress using
  `concurrencytest.NewGoroutineStressTester`. 검증 every task returns an ID
  및 the final stream count equals the task count.
- [ ] 실행 `go test -p 1 -count=1 ./redis/stream` 및
  `go test -p 1 -race -count=1 ./redis/stream`.

## 작업 4: 마이그레이션 Only the SQL Outbox Append Dispatch

**파일:** modify `audit/sqloutbox/redisstreams/{publisher.go,publisher_test.go}`

- [ ] 교체 the private `XAdd` dispatch path 함께 `redisstream.Append` using
  the identical `redis.XAddArgs{Stream: p.stream, Values: values}`.
- [ ] 유지 `Options`, `Publisher`, `Stream`, default stream, field encoding,
  cancellation preflight, 및 duplicate-attempt behavior unchanged.
- [ ] Make the provider `Client` compile-time compatible 함께
  `redisstream.Appender`; do 아님 expose unrelated Redis operations.
- [ ] 유지 the established `redis streams publish` 오류 boundary while
  preserving `errors.Is`/`errors.As` for the primitive's typed 원인 및 its
  sanitized key 진단.
- [ ] 추가/adjust fake client 테스트 for successful append, raw-key redaction,
  typed 원인 propagation, 및 zero calls 후 pre-canceled context.
- [ ] 실행 normal 및 race 테스트 for `./audit/sqloutbox/redisstreams`.

## 작업 5: Public Documentation, Example, And Diagram

**파일:** create 패키지 README/doc/example 및 the SVG/PNG; modify Redis/root
README locale pairs.

- [ ] Write 영문/한국어 패키지 guides 함께 호출자-owned timeout example,
  at-least-once duplicate guidance, pending/ack behavior, replay/`XAUTOCLAIM`,
  explicit trim/delete retention risk, consumer shutdown, cancellation
  ambiguity, 및 sanitized 오류/runbook guidance.
- [ ] 추가 a compile-checked `Example` that uses a 호출자-owned Redis client 및
  timeout; it must 아님 model a 패키지-managed loop 또는 claim exactly-once.
- [ ] 생성 `redis-streams-consumer-lifecycle.svg` using the sequence diagram
  계약. Read `docs/images/readme-diagrams/redis-lock-owner-token-lifecycle.png`
  및 one other approved sequence PNG at full size 전에 drawing. Include
  numbered messages, participant headers/lifelines/activations, a transparent
  recovery branch, 및 explicit 호출자-owned ack/replay/trim labels.
- [ ] Render its PNG 함께 CairoSVG, inspect the full-size PNG, 및 run XML,
  connector, geometry, endpoint, mixed-corner, 및 sequence-style audits.
  기록 concrete counts 및 results in the implementation review.
- [ ] Link the subpackage in `redis` README locale pairs 및 `redis/stream` in
  root README locale pairs. 유지 영문/한국어 공개 behavior in parity.
- [ ] 실행 `go test -count=1 ./redis/stream -run Example` 및 `git diff --check`.

## 작업 6: Full 검증, 리뷰, And Publication

**파일:** create review/lesson artifacts; update PR metadata 후 commit.

- [ ] 실행 focused normal/race 테스트 for both changed packages, then
  `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`,
  `make race`, 및 `make ci`.
- [ ] Perform the mandatory six-perspective 7-Tier implementation review over
  `origin/develop...HEAD`: 성능, 안정성, 보안, 운영자/Ops,
  개발자/API, 및 사용자/호출자. The main session owns the integration
  verdict. Resolve every P0/P1 전에 PR publication.
- [ ] Write the durable lesson covering why a provider may share a command
  primitive without leaking its domain envelope 또는 hiding at-least-once policy.
- [ ] 커밋 함께 a Lore-protocol message. 생성 a PR closing #571 함께 the
  issue's assignee, milestone, 및 labels. The PR body must end in
  `## DoD Status` 및 include targeted/final verification plus the explicit
  #560 benchmark N/A rationale.
- [ ] Wait for CI. On success, report exact run evidence; do 아님 merge unless
  the 사용자 explicitly asks to merge.

## 롤백

Revert the #571 commit. `audit/sqloutbox/redisstreams` returns to its direct
`XADD` dispatch 및 없음 Redis key/data migration is required because this issue
creates 없음 패키지-owned topology 또는 background state.
