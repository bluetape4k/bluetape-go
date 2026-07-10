# Issue #571 Redis Streams Primitive Test Specification

Date: 2026-07-10 KST
Issue: #571
Companion: `2026-07-10-issue-571-redis-streams-spec.md`

## Test Boundaries

| Layer | Location | Purpose |
|---|---|---|
| Unit | `redis/stream/stream_test.go` | Validate arguments, context behavior, typed-nil detection, argument preservation, and redacted typed errors without Redis. |
| Integration | `redis/stream/stream_integration_test.go` | Prove command behavior against Redis Testcontainers with bounded contexts and explicit cleanup. |
| Example | `redis/stream/example_test.go` | Compile and execute the public append/read or group contract as a caller-owned workflow. |
| Provider regression | `audit/sqloutbox/redisstreams/publisher_test.go` | Confirm #533 keeps its public behavior while reaching the shared append primitive. |
| Documentation | README/diagram review | Confirm public operational semantics accurately match the command surface. |

## Unit Cases

| Case | Assertions |
|---|---|
| `Append` validation | Reject blank stream, nil/typed-nil values, nil client, and typed-nil client with `ErrInvalidArgument`; no fake command invocation. |
| `Read` validation | Reject missing/odd go-redis stream lists (all keys followed by all IDs) and blank stream keys without changing valid IDs. |
| Group validation | Reject blank group/consumer/start values and malformed all-keys-then-all-IDs lists without dispatch. |
| Ack/delete validation | Reject missing or blank IDs; preserve exact valid IDs. |
| Pending/autoclaim/trim validation | Reject missing required names/cursors and invalid `maxLen`; preserve valid Redis command arguments. |
| Verbatim ownership | Stream, group, and consumer names containing leading/trailing spaces are observed unchanged by fake clients after blank validation. |
| Context preflight | Nil context becomes usable; canceled and expired contexts return their original context error and produce zero fake calls. |
| Dispatch errors | A fake `go-redis` command error is reachable through `errors.Is`; `errors.As` finds `*btredis.OpError`; formatted text contains neither raw stream key nor injected provider text. |
| Argument non-mutation | Each helper passes an equivalent copied args value/pointer and does not mutate caller-owned structs, slices, maps, or `Values`. |
| Return values | Successful fakes return command IDs, stream slices, pending slices, counts, and auto-claim next cursors unchanged. |

## Redis Testcontainers Cases

All integration tests use a `context.WithTimeout` bounded fixture based on
`testcontainers/redis`, clean only test-owned stream keys, close the client,
and run serially with `go test -p 1`.

| Case | Setup | Assertions |
|---|---|---|
| Append with optional max length | Append multiple entries using caller-set `XAddArgs.MaxLen`/`Approx` as appropriate. | Returned IDs are non-empty; retention setting is passed only because the caller explicitly set it. |
| Read | Append an entry, then `Read` from `0`. | Returned stream and field values match caller data. |
| Group read, pending, acknowledge | Create a group at `0`, append, read as consumer with `>`, inspect pending, acknowledge. | Read produces one pending entry; `XPendingExt` shows it before ack and no longer reports it after ack. |
| Auto-claim recovery | Consumer A reads without acknowledgement; Consumer B calls `AutoClaim` with a caller-chosen idle threshold and cursor. | The message becomes visible to B and the returned cursor is available for continued scans. |
| Explicit trim and delete | Append test-owned entries, call `TrimMaxLen` or `TrimMinID`, then `Delete` a known ID. | Counts are returned from Redis and no implicit retention action occurs on append/read. |
| Canceled blocking read | Call `Read` with a Redis block duration longer than a short caller timeout. | `errors.Is(err, context.DeadlineExceeded)` is true; when dispatch occurred, `errors.As(err, *btredis.OpError)` is true. No wall-clock timing threshold is asserted. |
| Concurrent append stress | Use `testing/concurrency.GoroutineStressTester` with a bounded task count and unique values. | All appends return IDs, Redis receives the expected count, and no shared helper state corrupts data. |

`XAUTOCLAIM` test setup may use a zero or small minimum idle time supported by
the tested Redis version; it must not rely on sleeping for a production-scale
idle interval.

## Provider Regression Cases

- Existing default stream, caller-owned stream preservation, record field
  encoding, duplicate attempts, canceled context, and Redis error propagation
  remain covered.
- Update the fake client only as needed to satisfy `redisstream.Appender`; do
  not expand it to an unrelated Redis facade.
- Add a focused assertion that provider failures retain `errors.Is` for the
  injected Redis error and `errors.As` for `*btredis.OpError` after shared
  append dispatch, while `err.Error()` does not expose the raw stream key.

## Verification Commands

```bash
go test -p 1 -count=1 ./redis/stream
go test -p 1 -race -count=1 ./redis/stream
go test -p 1 -count=1 ./audit/sqloutbox/redisstreams
go test -p 1 -race -count=1 ./audit/sqloutbox/redisstreams
go test -count=1 ./redis/stream -run Example
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

## Explicit Non-Applicable Checks

- No JUnit run: this is a Go repository; the equivalent package/race checks
  above are mandatory.
- No package-owned consumer-loop shutdown test: `redis/stream` owns no
  goroutine, polling loop, retry worker, or connection lifecycle.
- No benchmark result table/chart/analysis in this issue: no benchmark is run.
  Issue #560 owns provider benchmark measurement and, when it runs, must
  publish the required table, chart, and written analysis.
