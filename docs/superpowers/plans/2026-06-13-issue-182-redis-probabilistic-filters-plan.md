# Issue #182 Redis Probabilistic Filters Implementation Plan

> **Agentic worker 지침:** 이 계획은 task-by-task 실행을 전제로 한다. 체크박스(`- [ ]`)는 진행 추적용이며, 명령어·경로·API 이름은 원문 그대로 유지한다.

**Goal:** Go service instance들이 plain Redis bitmap commands를 통해 membership state를 공유하는 Redis-backed Bloom filter package를 만든다.

**Architecture:** public import path는 `github.com/bluetape4k/bluetape-go/probabilistic/redis`로 유지하고 package clause는 `redisbloom`을 사용한다. `probabilistic.Config`, `Hasher[T]`, existing SHA-256 double-hash formula를 small internal helper로 재사용한다. Redis state는 hash-tagged `{namespace}` key pair에 저장하고, immutable metadata는 static Lua scripts로 검증하며, bitmap operations는 server-side single round trip으로 수행한다.

**Tech Stack:** Go, `github.com/redis/go-redis/v9`, Redis Lua scripts through `redis.NewScript.Run`, Redis Testcontainers, `testing/concurrency.GoroutineStressTester`, `testing/concurrency.AsyncJobTester`, Graphviz-backed `$bluetape4k-diagram` assets.

## 소스 명세

- Spec: `docs/superpowers/specs/2026-06-13-issue-182-redis-probabilistic-filters-design.md`
- Step 2-R evidence: `docs/review/2026-06-13-issue-182-step-2r-spec-review.md`
- Issue: #182
- Branch/worktree: `.worktrees/issue-182-redis-probabilistic-filters`

## 파일 구조

| Path | 책임 |
| --- | --- |
| `probabilistic/hasher.go` | `Hasher.Bytes(value T) ([]byte, error)`를 export하고 기존 unexported `bytes` helper를 연결한다. |
| `probabilistic/internal/bloomhash/indexes.go` | shared deterministic Bloom index calculation |
| `probabilistic/internal/bloomhash/indexes_test.go` | deterministic index parity tests |
| `probabilistic/hash.go` | in-memory index calculation을 `bloomhash.Indexes`로 위임한다. |
| `probabilistic/bloom_filter.go` | in-memory behavior는 유지하고 `Hasher.Bytes`를 사용한다. |
| `probabilistic/bloom_filter_test.go` | `Hasher.Bytes`와 shared index parity regression coverage |
| `probabilistic/redis/doc.go` | package docs와 production caveats |
| `probabilistic/redis/errors.go` | sentinel errors와 `RedisError` |
| `probabilistic/redis/options.go` | option normalization, namespace validation, context normalization, config limits |
| `probabilistic/redis/keys.go` | Redis Cluster-safe hash-tagged key derivation과 redacted key id |
| `probabilistic/redis/scripts.go` | static Lua script constants와 `redis.NewScript` wrappers |
| `probabilistic/redis/config.go` | config fingerprint, config hash encoding/decoding, constructor metadata init |
| `probabilistic/redis/filter.go` | public constructors와 `BloomFilter[T]` implementation |
| `probabilistic/redis/options_test.go` | options, namespace, key layout, error contract tests |
| `probabilistic/redis/config_test.go` | constructor/config/mismatch/corrupt/concurrency tests |
| `probabilistic/redis/filter_test.go` | Bloom behavior, no-false-negative, clear, external deletion, command-count tests |
| `probabilistic/redis/concurrency_test.go` | `GoroutineStressTester`와 `AsyncJobTester` coverage |
| `probabilistic/redis/filter_benchmark_test.go` | `Put`과 `MightContain` hot-path benchmarks |
| `probabilistic/redis/example_test.go` | construction, put/check, clear warning, diagnostics examples |
| `probabilistic/README.md`, `probabilistic/README.ko.md` | API docs, operational caveats, diagram |
| `README.md`, `README.ko.md` | package table/update note |
| `CHANGELOG.md` | `[Unreleased]` item |
| `WIP.md` | #182 또는 0.6.1 Redis probabilistic work를 아직 추적한다면 갱신 |
| `scripts/generate-redis-bloom-diagram.mjs` | `$bluetape4k-diagram` 규칙을 따르는 deterministic README diagram generator |
| `docs/images/readme-diagrams/redis-bloom-key-layout-01.*` | dot/plain/graphviz/svg/png diagram outputs |
| `docs/review/2026-06-13-issue-182-code-review.md` | Step 6-R 산출물 |
| `docs/review/2026-06-13-issue-182-pr-review.md` | Step 7-R 산출물 |

## Step 3-R 계획 리뷰

Step 4 implementation 전에 7-Tier gate를 여섯 독립 subagent lane과 main integration으로 실행한다.

1. Tier 1 Performance: script round trips, `BITCOUNT` scope, benchmark gates, allocation risk.
2. Tier 2 Stability: cancellation, stale handles, Redis deletion/eviction, Testcontainers serial execution.
3. Tier 3 Security: namespace/key leakage, static Lua, ACL/TLS docs, error redaction.
4. Tier 4 Operator/Ops: Redis Cluster hash tags, runbook, migration, persistence/eviction.
5. Tier 5 Developer/API: Go package shape, errors, context handling, repo fit.
6. Tier 6 User/Caller: examples, misuse resistance, `Put(false)`, `Clear`, Kotlin migration.

Exit condition은 latest integrated table의 `P0=0 P1=0`이다. 근거는 `docs/review/2026-06-13-issue-182-step-3r-plan-review.md`에 저장한다.

## Step 4-T 구현 작업

### Task 1: Shared Hasher and Bloom Index Boundary

**Files:**
- Modify: `probabilistic/hasher.go`
- Modify: `probabilistic/hash.go`
- Modify: `probabilistic/bloom_filter.go`
- Modify: `probabilistic/bloom_filter_test.go`
- Create: `probabilistic/internal/bloomhash/indexes.go`
- Create: `probabilistic/internal/bloomhash/indexes_test.go`

- [ ] `Hasher.Bytes` exported boundary를 검증하는 failing tests를 먼저 작성한다.
- [ ] `bloomhash.Indexes` deterministic parity tests를 작성한다.
- [ ] production code 전 다음 명령이 실패하는지 확인한다.

```bash
go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash -run 'HasherBytes|Indexes'
```

- [ ] `Hasher.Bytes`를 구현하고 기존 `bytes` helper는 `Bytes`로 위임한다.
- [ ] SHA-256 double-hash 기반 shared index helper를 구현한다.
- [ ] in-memory Bloom filter가 helper를 사용하도록 연결하고 regression tests를 통과시킨다.

### Task 2: Redis Options, Keys, Errors

**Files:**
- Create: `probabilistic/redis/errors.go`
- Create: `probabilistic/redis/options.go`
- Create: `probabilistic/redis/keys.go`
- Create: `probabilistic/redis/options_test.go`

- [ ] namespace validation, config validation, context normalization, max payload/bit limits를 테스트로 고정한다.
- [ ] Redis Cluster-safe hash-tagged key pair를 만든다.
- [ ] error contract는 `%w`, `errors.Is`, redacted key id를 지원한다.
- [ ] Redis client lifecycle은 caller-owned로 문서화한다.

### Task 3: Redis Metadata and Lua Scripts

**Files:**
- Create: `probabilistic/redis/scripts.go`
- Create: `probabilistic/redis/config.go`
- Create: `probabilistic/redis/config_test.go`

- [ ] immutable Bloom metadata fingerprint를 정의한다.
- [ ] constructor가 metadata를 초기화하고 mismatch/corrupt state를 거부하는 테스트를 작성한다.
- [ ] `redis.NewScript.Run` 기반 static Lua script wrappers를 추가한다.
- [ ] Redis deletion/eviction 후 stale handle behavior를 테스트한다.

### Task 4: Redis Bloom Filter Behavior

**Files:**
- Create: `probabilistic/redis/filter.go`
- Create: `probabilistic/redis/filter_test.go`
- Create: `probabilistic/redis/concurrency_test.go`

- [ ] `Put`, `MightContain`, `Clear`, diagnostics API를 구현한다.
- [ ] no-false-negative behavior, false-positive caveat, external deletion, command counts를 검증한다.
- [ ] `GoroutineStressTester`와 `AsyncJobTester`로 concurrent put/check와 cancellation path를 검증한다.
- [ ] Testcontainers Redis tests는 resource conflict가 있으면 sequential execution으로 실행한다.

### Task 5: Benchmarks, Examples, Docs, Diagram

**Files:**
- Create: `probabilistic/redis/filter_benchmark_test.go`
- Create: `probabilistic/redis/example_test.go`
- Modify: `probabilistic/README.md`, `probabilistic/README.ko.md`
- Modify: `README.md`, `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `WIP.md` if still tracking #182
- Create: `scripts/generate-redis-bloom-diagram.mjs`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01.*`

- [ ] `Put`과 `MightContain` benchmark를 추가한다.
- [ ] examples는 construction, put/check, clear warning, diagnostics를 compile-checked로 제공한다.
- [ ] README에는 key layout, immutable metadata, Redis Cluster hash tags, `Clear` warning, ACL/TLS caveats를 포함한다.
- [ ] diagram asset은 generator에서 재생성 가능해야 한다.

## 최종 검증 게이트

완료 전 다음을 실행하거나 infrastructure failure를 명시적으로 기록한다.

- `go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash ./probabilistic/redis`
- `go test -race -count=1 ./probabilistic ./probabilistic/redis`
- `go test -run '^$' -bench . ./probabilistic/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `make fmt-check`
- `make tidy-check`
- `git diff --check`
- Step 6-R code review와 Step 7-R PR review with `P0=0 P1=0`

## 중단 조건

- Redis state와 in-memory Bloom index calculation이 parity를 잃으면 수정 전에는 PR을 열지 않는다.
- `MightContain`이 inserted value에 대해 false negative를 만들면 release blocker로 처리한다.
- Redis keys, namespace secrets, raw script values가 error/log/README/PR body에 노출되면 수정한다.
