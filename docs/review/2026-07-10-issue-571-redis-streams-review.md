# Issue #571 Redis Streams Primitive Implementation Review

## Scope

- Specification: `docs/superpowers/specs/2026-07-10-issue-571-redis-streams-spec.md`
- Plan: `docs/superpowers/plans/2026-07-10-issue-571-redis-streams-plan.md`
- New package: `redis/stream`
- Provider adoption: `audit/sqloutbox/redisstreams`
- Review mode: local six-perspective equivalent. Native subagent spawning is
  not exposed in this session; the main session independently applied every
  required perspective and owns the integration verdict.

## Resolved Implementation Findings

| Priority | Area | Finding | Resolution | Evidence |
|---|---|---|---|---|
| P1 | Cancellation | Redis `XREAD BLOCK` can return a Redis nil-style response after the caller deadline, losing `context.DeadlineExceeded`. | Dispatched errors now join the provider cause with an already-done context before `btredis.OpError` wrapping. | `TestReadDeadlinePreservesTypedError`; `TestOperationErrorRetainsDispatchedCauseAndExpiredContext`. |
| P1 | Test isolation | Static test stream names can retain a consumer group across normal/race processes when container cleanup is not immediately observable. | The fixture appends a UUID to each test-owned stream key and deletes only that key during cleanup. | Two consecutive normal/race targeted runs passed. |
| P1 | Lint state | golangci-lint cache referenced a removed #585 worktree; the new example also initially ignored `Client.Close`'s error. | Updated the example cleanup closure and cleared the linter cache before rerunning repository gates. | `make lint` reports `0 issues`. |

## 7-Tier Findings

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Each helper issues one caller-requested command; no client, retry, polling loop, or additional payload conversion is introduced. The bounded 24-task append stress test protects concurrent call safety. No benchmark is run; #560 owns provider benchmark table, chart, and analysis. |
| Stability | 0 | 0 | 0 | 0 | Pre-dispatch cancellation, post-dispatch dual-cause preservation, serial Testcontainers integration, UUID stream isolation, pending/ack/autoclaim, trim/delete, and normal/race coverage passed. |
| Security | 0 | 0 | 0 | 0 | `btredis.OpError` redacts raw stream keys and provider text while retaining `errors.Is`/`errors.As`; tests cover append, read, provider migration, and expired context paths. Payload data remains caller-owned and unlogged. |
| Operator/Ops | 0 | 0 | 0 | 0 | README and diagram expose at-least-once, PEL, recovery cursor, consumer shutdown, replay, and explicit retention ownership. No implicit trim/delete/recovery is hidden in helpers. |
| Developer/API | 0 | 0 | 0 | 0 | Narrow per-command interfaces match go-redis command shapes and native result types. `Read`/`ReadGroup` validate the exact all-streams-then-all-IDs order. #533 aliases only `Appender`. |
| User/Caller | 0 | 0 | 0 | 0 | Stream/group/consumer names and payload values remain verbatim after blank validation. The provider keeps its public API, field envelope, default stream, and duplicate-attempt behavior. |

## Diagram Evidence Ledger

| Gate | Evidence |
|---|---|
| Scope | `docs/images/readme-diagrams/redis-streams-consumer-lifecycle.{svg,png}`; related Redis diagram scan found the local lock lifecycle and leader sequence references. |
| Source | `redis/stream/stream.go`, `redis/stream/README.md`, and `audit/sqloutbox/redisstreams/publisher.go` were read before drawing. |
| References | Full-size `docs/images/readme-diagrams/redis-lock-owner-token-lifecycle.png` and `docs/images/readme-diagrams/redis-leader-election-sequence.png` were inspected. |
| XML | `xmllint --noout` passed. |
| Render | CairoSVG rendered `3200x2240` PNG with `/Users/debop/.local/bin/cairosvg ... -s 2`. |
| Connector audits | `markers=5 connectors=11 cards=0 intrusions=0 crossings=0`; geometry failures `0`; endpoint and mixed-corner audits passed. |
| Sequence fallback | The generic sequence script classified no sequence file, so a targeted invariant proved `participants=4 lifelines=4 activations=5 numbered_badges=11 message_labels=11 alt_frames=1 else_lines=1 marker_refs=5`. |
| Visual inspection | Full-size source PNG and a `1200x840` reduced preview were opened. Text, numbered messages, lifelines, transparent branch frame, and explicit trim/recovery labels were legible; recovery message 10 was moved inside the branch frame. |
| Diff hygiene | `git diff --check` passed. |

## Verification

| Command | Result |
|---|---|
| `go test -p 1 -count=1 ./redis/stream` | PASS, including Redis Testcontainers behavior. |
| `go test -p 1 -race -count=1 ./redis/stream` | PASS; repeated normal/race pair passed after UUID fixture isolation. |
| `go test -p 1 -count=1 ./audit/sqloutbox/redisstreams` | PASS. |
| `go test -p 1 -race -count=1 ./audit/sqloutbox/redisstreams` | PASS. |
| `go test -count=1 ./redis/stream -run Example` | PASS. |
| `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`, `make ci` | PASS; lint reports `0 issues` after clearing stale cache. |

## Integration Verdict

The implementation satisfies the direct Redis Streams primitive boundary and
removes duplicate append dispatch behavior from the SQL outbox provider without
turning either package into a message broker. Public documentation and the
rendered lifecycle diagram make at-least-once and operational ownership
explicit.

P0=0 P1=0
