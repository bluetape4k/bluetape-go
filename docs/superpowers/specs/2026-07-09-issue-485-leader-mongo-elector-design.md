# Issue #485 leader/mongo Single Elector Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: [#485](https://github.com/bluetape4k/bluetape-go/issues/485)  
Date: 2026-07-09  
Branch: `feat/issue-485-leader-mongo-elector`

## Problem

`bluetape-go` has a backend-neutral `leader.Elector` contract and a Redis
backend, but no MongoDB-backed elector. Issue #431 already bounded the MongoDB
scope: implement only a single `leader.Elector` first, and leave
`GroupElector` and `StrategicElector` to separate designs.

## Current Evidence

| Source | Evidence |
|---|---|
| `leader/elector.go` | `Campaign`, `Resign`, `IsLeader`, and `Leader` define the single-elector contract. |
| `leader/redis/elector.go` | Existing backend uses owner tokens, a renewal goroutine, and local `owned` state. |
| `docs/research/2026-07-09-issue-431-leader-mongodb-storage.md` | MongoDB first slice is one lease document per leader key; TTL is cleanup only. |
| `docs/lessons/2026-07-09-issue-431-leader-mongo-research.md` | Caller-owned MongoDB resources and single-elector-only scope are explicit lessons. |
| `jwt/mongo_repository.go` | Existing MongoDB code keeps caller-owned lifecycle, wraps driver errors, and handles duplicate-key races. |
| MongoDB Go driver docs | `FindOneAndUpdate` supports atomic update, `SetUpsert`, and `SetReturnDocument(options.After)`. |

## Selected Design

Add package `leader/mongo` with package name `mongoleader`.

Public API:

```go
func New(collection *mongo.Collection, opts leader.Options, options ...Option) (*Elector, error)
func EnsureIndexes(ctx context.Context, collection *mongo.Collection) error
func WithRetryDelay(delay time.Duration) Option
func WithClock(clock func() time.Time) Option
```

`New` accepts a caller-owned `*mongo.Collection`; the package never creates or
closes clients. `EnsureIndexes` is optional and creates a TTL cleanup index on
`lease_until`. The unique key is `_id`, so no separate unique index is required.

## Lease Document

| Field | Meaning |
|---|---|
| `_id` | Normalized leader key: `<keyPrefix>:<group>`. |
| `group` | Human-readable leader group. |
| `member_id` | Caller member ID. |
| `token` | Owner token returned by `Leader`. |
| `lease_until` | Authoritative lease expiry. |
| `created_at` | First creation timestamp. |
| `updated_at` | Last acquire/renew timestamp. |

## Operation Semantics

| Operation | Semantics |
|---|---|
| `Campaign(ctx)` | Reject duplicate local campaign, then loop until this elector acquires ownership or `ctx` is canceled. Acquisition uses `FindOneAndUpdate` with upsert and an expired-or-same-token predicate. Duplicate-key races mean another owner is active and should retry until context cancellation. |
| `renew` | `UpdateOne` only when `_id`, `token`, and `lease_until > now` match. Zero match means ownership is lost. |
| `Resign(ctx)` | Clear local state, stop renewal, then `DeleteOne` only when `_id` and `token` match. Repeated resign succeeds. |
| `Leader(ctx)` | Read only documents where `lease_until > now`; expired but not-yet-TTL-deleted documents return no leader. |
| `IsLeader()` | Local state only; false after resign, lost renewal, backend renewal failure, or canceled renewal loop. |

## Time Policy

The first slice computes `lease_until` with a package clock and documents the
bounded clock-skew requirement. `updated_at` and `created_at` use the same
clock. Server-side lease calculation is deferred because it would require an
aggregation update pipeline and broader compatibility proof.

## Rejected Approaches

| Approach | Reason |
|---|---|
| Ship group and strategic electors now | They need different document shapes and contention proofs. |
| Use TTL monitor for lease validity | TTL deletion is asynchronous, so correctness must use `lease_until` predicates. |
| Create/own MongoDB client in package | Existing repo boundary keeps MongoDB clients caller-owned. |
| Require transactions for single elector | Single-document atomicity is sufficient for the first slice. |

## Acceptance Criteria

- `leader/mongo` implements `leader.Elector`.
- Owner-token acquire, renew, release, and observation are covered.
- `Campaign(ctx)` respects cancellation while waiting for an active owner.
- Failed renewal or stolen token flips `IsLeader()` false.
- Expired documents can be taken over before TTL cleanup.
- README and README.ko document caller ownership, write concern guidance, TTL
  cleanup caveat, and clock-skew caveat.
- Verification passes:
  - `go test -count=1 ./leader ./leader/mongo`
  - `go test -race -count=1 ./leader ./leader/mongo`
  - `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb`

