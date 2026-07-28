# Issue #410 Redis Cuckoo and HyperLogLog Research

> 한국어 연구 요약: 이 문서는 사용자 협업용 조사/결정 기록이다. 아래 표와 목록의 URL, package name, command, issue number, version, source path는 evidence이므로 그대로 보존한다. 의사결정, 선택/보류/거절 사유, 후속 이슈 경계는 한국어 독자가 바로 이해할 수 있도록 이 요약을 우선 적용한다.
> 추가 한국어 해석: 이 문서에서 영어로 남은 표의 값은 원문 근거이며, 실제 채택 여부는 한국어 결정 문장을 따른다. 후속 작업자는 보류와 거절 항목을 새 구현 범위로 착각하지 않아야 한다.\n

Issue: #410
Parent: #409
Milestone: 0.16.0
Date: 2026-07-08

## 목표

Choose the first Redis probabilistic follow-up structure after the existing
Redis-backed Bloom filter scope, and record the Redis/runtime assumptions that
must shape #411, #412, and #413.

## Current Repo Evidence

- `probabilistic/redis` currently provides only Redis-backed Bloom filters.
- Root README and `probabilistic/README.md` state that Redis-backed Cuckoo and
  HyperLogLog/HLL are follow-up work after Redis Bloom.
- #182 deliberately rejected combining Bloom, Cuckoo, and HLL in one PR because
  Cuckoo deletion/saturation behavior and HLL cardinality estimation are
  separate contracts.
- Existing Redis packages use `github.com/redis/go-redis/v9` and caller-owned
  `context.Context`.
- Existing Redis integration tests use `testcontainers/redis`, not Redis Stack
  or a RedisBloom-specific fixture.

## External Evidence

Primary Redis documentation fetched with bounded `curl` on 2026-07-08:

| Source | Status | Evidence Used |
|---|---|---|
| https://redis.io/docs/latest/develop/data-types/probabilistic/hyperloglogs/ | HTTP 200 | HyperLogLog is a cardinality-estimation structure; command examples use `PFADD`, `PFCOUNT`, and `PFMERGE`. |
| https://redis.io/docs/latest/commands/pfadd/ | HTTP 200 | `PFADD` adds elements to an HLL key and creates the key when absent. |
| https://redis.io/docs/latest/develop/data-types/probabilistic/cuckoo-filter/ | HTTP 200 | Cuckoo operations use `CF.RESERVE`, `CF.ADD`, `CF.EXISTS`, and `CF.DEL`. |
| https://redis.io/docs/latest/commands/cf.add/ | HTTP 200 | `CF.ADD` is a Cuckoo command with command-group/module-style availability expectations. |

Local go-redis v9.20.0 source evidence:

- `hyperloglog_commands.go` exposes `PFAdd`, `PFCount`, and `PFMerge` on
  context-aware `HyperLogLogCmdable`.
- `probabilistic.go` exposes RedisBloom-style `BF*`, `CF*`, Count-Min Sketch,
  TopK, and TDigest command methods.
- go-redis command methods existing locally do not prove a plain Redis server
  supports `CF*`; server command availability must still be tested.

Durable web research note:

- `bluetape4k-wiki/research/2026-07-08-redis-cuckoo-hll-bluetape-go.md`

## Candidate Comparison

| Candidate | Fit | Risks | Decision |
|---|---|---|---|
| HyperLogLog first | Uses core Redis `PF*` commands, fits existing `testcontainers/redis`, and has a narrow cardinality API. | Must avoid presenting HLL as a membership filter; estimates are approximate and merge behavior is key-based. | Accept for #411. |
| Cuckoo first | Provides deletion-capable membership, which Bloom does not. | Needs Redis Cuckoo command availability, reserve options, deletion/count semantics, saturation behavior, and module-backed fixture evidence. | Defer until module/runtime assumptions are explicit. |
| Cuckoo and HLL together | Covers both follow-up structures in one PR. | Mixes membership and cardinality contracts, expands tests and docs, and risks hiding module assumptions. | Reject for 0.16.0 first implementation. |
| Wrap all go-redis probabilistic commands | Reuses existing go-redis methods broadly. | Exposes RedisBloom/advanced module surface before bluetape-go owns the operator contract. | Reject. |

## Recommended First Scope for #411

Implement Redis HyperLogLog first in `probabilistic/redis`.

API direction:

```go
type HyperLogLog[T any] interface {
    Add(ctx context.Context, values ...T) (bool, error)
    Count(ctx context.Context) (uint64, error)
    Merge(ctx context.Context, sources ...string) error
}
```

Constructor direction:

- accept `redis.Cmdable`, a validated namespace/key, and
  `probabilistic.Hasher[T]`;
- provide string and bytes convenience constructors;
- keep values caller-owned and never log raw values;
- preserve `context.Canceled` and `context.DeadlineExceeded` through
  `errors.Is`;
- wrap Redis failures with operation and redacted key information.

Test direction for #411/#412:

- use the existing `testcontainers/redis` fixture for HLL;
- cover `PFADD`, `PFCOUNT`, `PFMERGE`, context cancellation/deadline, invalid
  options, Redis command errors, and race-safe concurrent adds/counts;
- run targeted tests under `go test -race`;
- do not add Cuckoo tests until the fixture can prove `CF*` support or the
  unsupported-command behavior is intentionally documented.

## Cuckoo Follow-Up Contract

Cuckoo should remain a later slice until #412 or a new issue records:

- whether the runtime fixture is Redis Stack, Redis Open Source with Bloom
  commands, or another RedisBloom-capable container;
- exact unsupported-command behavior on plain Redis;
- reserve options to expose first (`CAPACITY`, `BUCKETSIZE`, `MAXITERATIONS`,
  `EXPANSION`);
- deletion semantics and duplicate count behavior;
- capacity/saturation error behavior;
- Testcontainers coverage for `CF.RESERVE`, `CF.ADD`, `CF.EXISTS`, `CF.COUNT`,
  and `CF.DEL`.

## Documentation Implications for #413

README updates should say:

- 0.16.0 starts with Redis HLL because it is core Redis and fits the current
  fixture/driver boundary.
- HLL estimates cardinality; it does not answer membership.
- Cuckoo is module-gated and deferred until `CF*` runtime assumptions are
  explicit.
- RedisBloom/go-redis command availability and Redis server command
  availability are separate concerns.

## Acceptance Mapping

| #410 Acceptance Criterion | Status | Evidence |
|---|---|---|
| One first structure is recommended for implementation. | PASS | HLL is selected for #411. |
| Redis module assumptions are documented. | PASS | HLL uses core Redis `PF*`; Cuckoo is deferred until `CF*` runtime/module assumptions are explicit. |
| Rejected alternatives are documented. | PASS | Cuckoo-first, combined Cuckoo+HLL, and broad go-redis probabilistic wrapper are rejected for the first implementation. |

## 검증 Plan

- `git diff --check`
- targeted `rg` for #410, HLL, and Cuckoo decision terms
- PR CI after publication
