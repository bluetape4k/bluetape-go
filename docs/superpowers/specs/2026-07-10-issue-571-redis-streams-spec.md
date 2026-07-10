# Issue #571 Redis Streams Primitive Spec

Date: 2026-07-10 KST
Issue: #571
Milestone: 0.19.0
Branch: `feat/issue-571-redis-streams`

## Problem

`audit/sqloutbox/redisstreams` currently owns a narrow `XADD` client interface
and dispatches directly to `go-redis`. That is suitable for the provider, but
there is no shared Go-native surface for the Redis Streams operations that a
provider or application must explicitly control: append, read, consumer groups,
acknowledgement, pending-entry recovery, and retention actions.

Issue #571 adds that small direct-command surface at:

```go
import redisstream "github.com/bluetape4k/bluetape-go/redis/stream"
```

It must be a Redis Streams primitive, not a generic message bus. Callers own
their `go-redis` client, stream names, consumer/group topology, payload
encoding, retry policy, deadlines, and recovery decisions.

## Current Contract Evidence

| Evidence | Observation | Design implication |
|---|---|---|
| `audit/sqloutbox/redisstreams/publisher.go` | The provider has a private `XAdd` interface and sends caller-owned stream/value data directly. | Share append validation, cancellation, and sanitized error behavior; retain audit-specific field construction in the provider. |
| `audit/sqloutbox/redisstreams/publisher_test.go` | The provider preserves duplicate audit attempts and caller-owned stream keys. | The primitive cannot deduplicate, normalize stream names, or impose audit fields. |
| `redis/errors.go` | `btredis.OpError` keeps a typed cause while formatting only low-cardinality labels and a redacted Redis key id. | All Redis dispatch failures use that established redaction contract. |
| `redis/integration_test.go` | Redis Testcontainers tests use bounded contexts, explicit cleanup, and serial execution. | Stream integration tests follow the same fixture pattern and run serially. |
| Local `go doc github.com/redis/go-redis/v9` | `XAdd`, `XRead`, `XReadGroup`, `XAck`, `XPendingExt`, `XAutoClaim`, trim, and delete APIs are available. | The package should expose narrow command-shaped interfaces and return existing `go-redis` response values. |
| `testing/concurrency/goroutine_stress.go` | The repository has a bounded goroutine stress harness. | A bounded concurrent append stress test can prove stateless primitive calls do not share mutable package state. |

## Goals

- Add `redis/stream` with package clause `redisstream`.
- Wrap the direct Redis Streams operations needed for explicit at-least-once
  workflows: `XADD`, `XREAD`, `XGROUP CREATE ... MKSTREAM`, `XREADGROUP`,
  `XACK`, `XPENDING` via `XPendingExt`, `XAUTOCLAIM`, explicit trim, and
  explicit delete.
- Preserve stream keys, group names, consumer names, IDs, field keys, and
  values exactly as supplied after validation. The package does not serialize
  payloads or introduce its own envelope.
- Normalize a nil context to `context.Background()` and return an already-done
  context error before Redis dispatch, matching established provider behavior.
- Return `btredis.OpError` for dispatched command failures, preserving the
  original cause for `errors.Is` and `errors.As` while keeping formatted errors
  free of raw Redis stream keys and provider text.
- Migrate only the append dispatch in `audit/sqloutbox/redisstreams` to the
  shared primitive. Its audit record field schema, duplicate-attempt behavior,
  default stream, and public `Publisher` API remain unchanged.
- Document at-least-once delivery, duplicate handling, pending entries,
  replay, retention/trim, consumer shutdown, and the indeterminate state after
  a dispatched cancellation.

## Non-Goals

- No generic broker, producer/consumer framework, background polling loop,
  automatic retry, global client, global metrics, or global logging.
- No Kafka, NATS, EventBridge, RESP3, Redis module, or topology abstraction.
- No audit-specific fields, JSON encoding, idempotency policy, or automatic
  duplicate suppression in `redis/stream`.
- No implicit stream trimming, deletion, group creation, pending recovery, or
  acknowledgement. Each is a caller-visible command.
- No legacy `XCLAIM` wrapper in this first slice. `XAUTOCLAIM` is the supported
  recovery command because it advances a caller-owned cursor without requiring
  callers to preselect message IDs.
- No benchmark execution in this issue. Provider comparison matrices, charts,
  and written benchmark analysis are owned by #560; no benchmark result chart
  is applicable until that issue runs measurements.

## Alternatives Considered

| Approach | Decision | Reason |
|---|---|---|
| Stateless command helpers over narrow per-command `go-redis` interfaces | Accept | Directly matches the issue and permits small fakes without hiding Redis semantics. |
| Stateful consumer service with package-owned polling goroutines | Reject | It would own shutdown, retry, backpressure, topology, and duplicate semantics that must remain caller policy. |
| Full `go-redis` facade or generic message bus | Reject | It duplicates a mature client and conflicts with the explicit non-goals. |
| Keep #533's private `XAdd` path indefinitely | Reject | It leaves duplicate cancellation/error behavior in a provider that should use the shared primitive. |
| Add both `XCLAIM` and `XAUTOCLAIM` now | Reject | The latter covers the intended recovery flow with a smaller, clearer API; add `XCLAIM` only after a concrete caller needs explicit IDs. |

## Selected API Shape

The package accepts only the command methods it invokes. It does not accept or
construct a broad `redis.Cmdable` facade.

```go
package redisstream

var ErrInvalidArgument = errors.New("redis stream: invalid argument")

type Appender interface {
    XAdd(context.Context, *redis.XAddArgs) *redis.StringCmd
}

func Append(ctx context.Context, client Appender, args redis.XAddArgs) (string, error)

type Reader interface {
    XRead(context.Context, *redis.XReadArgs) *redis.XStreamSliceCmd
}

func Read(ctx context.Context, client Reader, args redis.XReadArgs) ([]redis.XStream, error)

type GroupCreator interface {
    XGroupCreateMkStream(context.Context, string, string, string) *redis.StatusCmd
}

func CreateGroup(ctx context.Context, client GroupCreator, stream, group, start string) error

type GroupReader interface {
    XReadGroup(context.Context, *redis.XReadGroupArgs) *redis.XStreamSliceCmd
}

func ReadGroup(ctx context.Context, client GroupReader, args redis.XReadGroupArgs) ([]redis.XStream, error)

type Acknowledger interface {
    XAck(context.Context, string, string, ...string) *redis.IntCmd
}

func Acknowledge(ctx context.Context, client Acknowledger, stream, group string, ids ...string) (int64, error)

type PendingInspector interface {
    XPendingExt(context.Context, *redis.XPendingExtArgs) *redis.XPendingExtCmd
}

func Pending(ctx context.Context, client PendingInspector, args redis.XPendingExtArgs) ([]redis.XPendingExt, error)

type AutoClaimer interface {
    XAutoClaim(context.Context, *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
}

func AutoClaim(ctx context.Context, client AutoClaimer, args redis.XAutoClaimArgs) ([]redis.XMessage, string, error)

type Trimmer interface {
    XTrimMaxLen(context.Context, string, int64) *redis.IntCmd
    XTrimMinID(context.Context, string, string) *redis.IntCmd
}

func TrimMaxLen(ctx context.Context, client Trimmer, stream string, maxLen int64) (int64, error)
func TrimMinID(ctx context.Context, client Trimmer, stream, minID string) (int64, error)

type Deleter interface {
    XDel(context.Context, string, ...string) *redis.IntCmd
}

func Delete(ctx context.Context, client Deleter, stream string, ids ...string) (int64, error)
```

The implementation may use unexported helpers for context normalization,
typed-nil interface detection, validation, and redacted operation errors. It
must copy value arguments before handing pointers to `go-redis` so a caller
cannot observe package mutation; the values themselves remain byte-for-byte
caller data.

## Command And Validation Contract

- Blank detection may use `strings.TrimSpace`, but valid stream/group/consumer
  names are passed to Redis verbatim. The package never trims or rewrites them.
- A nil context becomes `context.Background()`. If `ctx.Err()` is already
  non-nil after normalization, no Redis command is dispatched and that context
  error is returned directly.
- A nil or typed-nil command client is rejected with an error matching
  `ErrInvalidArgument` before dispatch.
- `Append` requires a non-blank `args.Stream` and non-nil `args.Values`,
  including no typed-nil map, slice, pointer, or interface value; it leaves
  IDs, values, and optional `XAddArgs` trimming settings unchanged.
  Redis remains authoritative for command-specific combinations such as trim
  modes and IDs.
- `Read` requires a non-empty, even-length `args.Streams` list in go-redis
  order: every stream key first, followed by one ID per stream. Each key in the
  first half is non-blank; each ID is preserved and may use Redis cursor forms
  such as `0`, `$`, or `>` where the underlying command allows them.
- `CreateGroup` requires non-blank stream, group, and start IDs and always uses
  `XGROUP CREATE ... MKSTREAM`. Existing-group responses remain typed provider
  errors; the caller decides whether an existing group is acceptable.
- `ReadGroup` requires non-blank `Group` and `Consumer` plus an even non-empty
  `Streams` list in the same all-streams-then-all-IDs order. It preserves
  `NoAck`, `Claim`, IDs, and blocking values.
- `Acknowledge` and `Delete` require a non-blank stream and at least one
  non-blank message ID. `Acknowledge` additionally requires a non-blank group.
- `Pending` requires a non-blank stream and group; it preserves start/end
  cursor semantics, consumer filter, count, and idle duration for Redis.
- `AutoClaim` requires non-blank stream, group, consumer, and start cursor.
  Its `MinIdle` must be non-negative, and its returned next cursor must be
  passed back by callers that continue scanning pending entries.
- `TrimMaxLen` requires a positive `maxLen`; `TrimMinID` requires a non-blank
  stream and minimum ID. Both are explicit retention commands and never run as
  a side effect of append/read/group operations.
- Error labels use `Family: "redis stream"` and operation values
  `append`, `read`, `group-create`, `group-read`, `ack`, `pending`,
  `autoclaim`, `trim-maxlen`, `trim-minid`, and `delete`. For multi-stream
  reads, the redacted operation key is a deterministic length-delimited ordered
  aggregation of the supplied stream keys; raw names never appear in formatted
  errors.

## Delivery And Operations Contract

- Redis Streams consumer groups are at-least-once. A message can be delivered
  again after consumer failure, timeout, a non-acknowledging read, or recovery
  through `XAUTOCLAIM`. Consumers must make effects idempotent using their own
  record identity.
- `ReadGroup` creates pending entries unless callers set `NoAck`. `NoAck`
  disables pending tracking and therefore disables acknowledgement/recovery for
  that read; it is an explicit caller choice.
- `Acknowledge` removes only the supplied pending IDs for the supplied group.
  It does not delete stream entries. `Delete` removes supplied stream entries
  but does not replace acknowledgement or retention policy.
- `Pending` and `AutoClaim` expose recovery state; the package does not decide
  idle thresholds, steal policy, replay cursor persistence, or retry limits.
- Trim and delete can make history unavailable, including entries a slow
  consumer expects to replay. Retention must be chosen by the owning service
  after considering consumer lag, pending entries, and incident replay needs.
- A caller stopping a consumer should stop issuing reads, finish or record its
  in-flight idempotent work, acknowledge only completed effects, and leave
  unresolved work pending for a controlled recovery policy.
- Once a Redis command has been dispatched, cancellation or deadline expiry
  can leave command commit state indeterminate. The returned `OpError` still
  unwraps the context/provider cause; callers must inspect stream/group state
  or retry through an idempotent workflow rather than assuming no effect.

## Provider Migration

`audit/sqloutbox/redisstreams.Publisher.Publish` will retain record encoding,
default stream selection, and the existing `redis streams publish` error
boundary. Its final append dispatch changes from `p.client.XAdd(...).Result()`
to `redisstream.Append(...)` with the same `redis.XAddArgs`. This removes its
duplicate command/cancellation/error handling without making the primitive
aware of audit fields or changing the provider's duplicate-attempt contract.

The provider's `Client` type may become an alias of, or otherwise compile-time
compatible with, `redisstream.Appender`; no broader client surface is added.

## Documentation And Diagram

- Add `redis/stream/doc.go`, `redis/stream/README.md`, and
  `redis/stream/README.ko.md` with directly runnable append and consumer-group
  examples using caller-owned timeouts.
- Add an executable `Example` test for the direct append/read/group boundary.
- Link the package from `redis/README.md` and `redis/README.ko.md`; update the
  root locale README package inventory only if its current public-package list
  includes `redis` descendants.
- Add a Mermaid source diagram plus the repository-required rendered artifact
  showing append, group read, pending, acknowledge, and explicit recovery / trim
  ownership. The diagram labels must state at-least-once and caller-owned
  operations without presenting trim or replay as automatic.

## Test Plan

- Unit-test each validation branch, nil and typed-nil client rejection,
  verbatim stream/group/consumer preservation, value-argument non-mutation,
  pre-dispatch cancellation, and `btredis.OpError` redaction/cause matching.
- Use Redis Testcontainers for append/read, group creation/read, acknowledge,
  pending inspection, `XAUTOCLAIM`, explicit trim/delete, and a blocked read
  that reaches context deadline. Run these tests serially.
- Use the existing bounded `GoroutineStressTester` for concurrent appends with
  unique IDs/values. The primitive owns no goroutines or loops, so its race
  obligation is concurrent call safety, not loop shutdown correctness.
- Preserve #533 provider tests and add an assertion that its dispatch reaches
  the shared append contract while retaining audit values and duplicate attempts.
- Verify with targeted normal and race tests, then `make ci` before PR
  publication.

## Risks And Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| A helper silently introduces consumer policy. | P1 | Keep every state-changing command explicit and omit background loops/retries. |
| Raw stream names or provider error text leak in diagnostics. | P1 | Wrap dispatched failures through `btredis.OpError`; test formatted errors against raw keys. |
| Group recovery is mistaken for exactly-once delivery. | P1 | README, diagram, and integration tests make pending/replay/duplicate semantics explicit. |
| Trim invalidates incident replay unexpectedly. | P1 | Expose trim only as an explicit command and document its retention trade-off. |
| #533 behavior changes while sharing append. | P1 | Keep the provider public API/field schema unchanged and retain its duplicate-attempt integration test. |
| Blocking test flakes in CI. | P2 | Use a bounded context and assert the unwrapped deadline rather than elapsed wall-clock precision. |
