# Issue #173 Distributed JWT KeyChain Repositories Implementation Plan

> **Agentic worker 지침:** 이 계획은 task-by-task 실행을 전제로 한다. 체크박스(`- [ ]`)는 진행 추적용이며, 명령어·경로·API 이름은 원문 그대로 유지한다.

**Goal:** issue #173을 위해 Redis-backed distributed JWT KeyChain repositories와 context-aware `DistributedProvider`를 구현한다.

**Architecture:** `DistributedProvider`는 `*Provider`를 private named composition으로 보유하고, distributed key lookup, rotation, reset을 context-aware `DistributedKeyChainRepository`에 위임한다. Redis core storage와 DTO reconstruction은 package `jwt` 내부에 두어 private `KeyChain` material이 public raw-key API로 노출되지 않게 한다. package `jwt/redis`는 user-facing import facade로 유지한다. Redis는 signing authority이며, O(1)-style `kid` lookup, Lua-backed atomic rotation, strict DTO validation, Testcontainers-backed verification을 제공한다.

**Tech Stack:** Go 1.25, `github.com/golang-jwt/jwt/v5`, `github.com/redis/go-redis/v9`, `github.com/testcontainers/testcontainers-go/modules/redis`, repo-local `testing/concurrency`, `$bluetape-go-patterns`, `$bluetape4k-diagram`, `$vega`.

## 소스 명세

- Issue: #173
- Milestone: 0.6.1
- Spec: `docs/superpowers/specs/2026-06-12-issue-173-distributed-jwt-keychain-repositories-design.md`
- Spec review: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-step-2r-spec-review.md`
- MongoDB backlog: #198

## 실행 경계

구현 범위는 Redis-backed distributed KeyChain repository로 제한한다.

- `DistributedProvider`와 `DistributedKeyChainRepository`에 필요한 package `jwt` 변경
- Redis storage core, DTO validation, Lua scripts, tests, benchmarks
- public Redis facade인 package `jwt/redis`
- distributed JWT usage와 operations를 설명하는 `jwt/README.md`, `jwt/README.ko.md`
- #173 metadata visibility가 필요한 경우에만 root docs 또는 release notes
- 구현 후 Step 6-R, Step 7-R 산출물

다음은 구현하지 않는다.

- MongoDB backend는 #198에 남긴다.
- fixed/local raw-key export/import 또는 seamless old-token migration
- JWKS, JWE, OIDC, sessions, roles, auth middleware, background rotation timers, Kotlin/Redisson wire compatibility
- public raw HMAC/RSA key material accessors

모든 Go API, test, benchmark, example, README, review task에는 `$bluetape-go-patterns`를 적용한다. benchmark 결과가 docs, verifier evidence, PR evidence, README에 들어가면 `$bluetape4k-diagram`과 `$vega`를 적용한다.

## 현재 근거

- `jwt/provider.go`: signing, parsing, parser hardening, provider config, `createKeyChain`, local `Provider` methods를 소유한다.
- `jwt/repository.go`: private in-memory repository behavior만 가진다.
- `jwt/keychain.go`: key material은 private이며 metadata와 package-private signing/verification helpers만 노출한다.
- `ratelimit/redis`: `redis.Cmdable`, option normalization, namespace key construction, Lua `Eval`, `%w` wrapping pattern을 보여준다.
- `testcontainers/redis`: Redis-backed integration tests용 `Start(ctx, t)`를 제공한다.
- `testing/concurrency`: `GoroutineStressTester`와 `AsyncJobTester`를 제공한다.
- `go.mod`: `github.com/redis/go-redis/v9`와 `testcontainers-go/modules/redis`가 이미 있다.
- Step 2-R 결과는 `P0=0 P1=0`이며, P2 carry-forward는 Redis command-count, benchmark budget, configured `KeyTTL` safety rule, README runbook evidence다.

## 파일 구조

| Path | 책임 |
| --- | --- |
| `jwt/distributed_provider.go` | public `DistributedProvider`, constructors, context-aware compose/parse/rotation/reset methods, internal provider composition |
| `jwt/distributed_repository.go` | exported `DistributedKeyChainRepository` interface와 backend-neutral repository contract helpers |
| `jwt/distributed_provider_test.go` | fake repository 기반 provider tests |
| `jwt/redis_options.go` | Redis `Options`, validation, namespace normalization, capacity, payload size, `KeyTTL`, key naming |
| `jwt/redis_dto.go` | package `jwt` 내부 JSON DTO encode/decode와 size/algorithm validation |
| `jwt/redis_repository.go` | `Current`, `Find`, `Rotate`, `ForcedRotate`, `DeleteAll` Redis repository implementation |
| `jwt/redis_scripts.go` | atomic current-hit, CAS store, forced rotate, trim, delete용 Lua scripts |
| `jwt/redis_repository_test.go` | Redis Testcontainers behavior, malformed state, namespace isolation, command counts, TTL, cancellation |
| `jwt/redis_benchmark_test.go` | find, rotate current-hit, expired rotate, forced rotate, compose, parse benchmark harness |
| `jwt/redis/doc.go` | Redis backend trust boundary와 caller-owned client lifecycle package docs |
| `jwt/redis/redis.go` | facade `Options`, `Repository`, `New` aliases |
| `jwt/redis/example_test.go` | compile-checked distributed provider Redis examples |
| `jwt/README.md`, `jwt/README.ko.md` | public usage, migration, operational runbook, Redis trust boundary, key format, MongoDB #198 deferral |
| `docs/research/outputs/issue-173/` | benchmark 결과를 기록할 경우 raw benchmark outputs |
| `docs/images/readme-charts/` | benchmark chart가 문서/PR 근거에 들어갈 경우 generated SVG/PNG |
| `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-*.md` | verifier, Step 6-R code review, Step 7-R PR review, benchmark evidence artifacts |

## Step 3-R 계획 리뷰

Step 4 implementation 전에 7-Tier gate를 여섯 독립 lane과 main integration으로 실행한다.

1. Tier 1 Performance: script round trips, `BITCOUNT` scope, benchmark gates, allocation risk.
2. Tier 2 Stability: cancellation, stale handles, Redis deletion/eviction, Testcontainers serial execution.
3. Tier 3 Security: namespace/key leakage, static Lua, ACL/TLS docs, error redaction.
4. Tier 4 Operator/Ops: Redis Cluster hash tags, runbook, migration, persistence/eviction.
5. Tier 5 Developer/API: Go package shape, errors, context handling, repo fit.
6. Tier 6 User/Caller: examples, misuse resistance, `Put(false)`, `Clear`, Kotlin migration.

Exit condition은 latest integrated table의 `P0=0 P1=0`이다. 근거는 `docs/review/2026-06-13-issue-182-step-3r-plan-review.md` 같은 review artifact에 저장한다.

## 작업 계획

### Task 0: 입력 재확인과 사전 구현 근거 고정

**complexity: medium**

**Files:**
- Create: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-preimplementation-risk.md`
- Read: `jwt/provider.go`, `jwt/repository.go`, `jwt/keychain.go`, `ratelimit/redis/limiter.go`, `ratelimit/redis/options.go`, `testcontainers/redis/redis.go`, `testing/concurrency/*.go`

- [x] **Step 1: branch와 clean baseline 확인**

```bash
pwd
git status --short --branch
git merge-base --is-ancestor origin/develop HEAD
```

예상 결과: working directory는 issue worktree이고, branch는 `issue-173-distributed-jwt-keychain-repositories`이며, exit code는 branch가 `origin/develop` history를 포함함을 보여준다.

- [x] **Step 2: dependency/API evidence 기록**

risk note에는 `Current Source Evidence`, `Locked Decisions`, `Step 2-R P2 Carry Forward`를 포함한다. 특히 `DistributedProvider`는 anonymous embedding이 아니라 `provider *Provider`를 사용하고, Redis core는 package `jwt` 내부에 두며, repository IO는 caller cancellation/deadline을 보존하고, key values는 error strings/logs/README examples/PR body에 나타나지 않아야 한다.

- [x] **Step 3: evidence commands 검증**

```bash
rg -n "type Provider|func \\(p \\*Provider\\) Compose|func \\(p \\*Provider\\) Parse|func \\(p \\*Provider\\) createKeyChain" jwt/provider.go
rg -n "type keyChainRepository|func \\(r \\*keyChainRepository\\) rotate|func \\(r \\*keyChainRepository\\) find" jwt/repository.go
rg -n "type Limiter|redis\\.Cmdable|Eval\\(" ratelimit/redis
rg -n "GoroutineStressTester|AsyncJobTester|type Task" testing/concurrency
git diff --check
```

예상 결과: 모든 command가 matching evidence를 출력하고 `git diff --check`가 통과한다.

### Task 1: Distributed Provider Contract Tests

**complexity: high**

**Files:**
- Create: `jwt/distributed_provider_test.go`
- Use: `$bluetape-go-patterns`

- [x] fake repository tests를 먼저 작성한다.
- [x] bootstrap, cross-instance compose/parse, rotation, forced rotation, reset, context propagation, repository failure, migration boundary를 검증한다.
- [x] production code가 없기 때문에 첫 실행은 실패해야 한다.

```bash
go test -count=1 ./jwt -run 'DistributedProvider|Distributed'
```

### Task 2: Distributed Provider API

**complexity: high**

**Files:**
- Create: `jwt/distributed_provider.go`
- Create: `jwt/distributed_repository.go`
- Modify: `jwt/provider.go`
- Modify: `jwt/repository.go`
- Modify: `jwt/keychain.go`

- [x] `DistributedKeyChainRepository` interface와 repository-neutral helper를 추가한다.
- [x] `DistributedProvider` constructor가 non-nil `context.Context`와 repository를 요구하게 한다.
- [x] compose/parse/rotate/reset methods가 repository를 통해 keychain을 조회하거나 갱신하게 한다.
- [x] key material access는 package-private boundary로만 유지한다.

### Task 3: Redis Options, Key Layout, DTO

**complexity: high**

**Files:**
- Create: `jwt/redis_options.go`
- Create: `jwt/redis_dto.go`
- Create: `jwt/redis_options_test.go`

- [x] namespace normalization, capacity, payload size, `KeyTTL`, `RetentionLeeway`, key naming을 테스트로 고정한다.
- [x] JSON DTO encode/decode가 HMAC/RSA key material을 package `jwt` 내부에서만 재구성하게 한다.
- [x] invalid algorithm, corrupt payload, oversize payload, key redaction을 검증한다.

### Task 4: Redis Repository Scripts and Behavior

**complexity: high**

**Files:**
- Create: `jwt/redis_scripts.go`
- Create: `jwt/redis_repository.go`
- Create: `jwt/redis_repository_test.go`

- [x] current-hit, CAS store, forced rotate, trim, delete를 Lua-backed atomic operation으로 구현한다.
- [x] `Current`, `Find`, `Rotate`, `ForcedRotate`, `DeleteAll` behavior를 Testcontainers로 검증한다.
- [x] malformed state, namespace isolation, command counts, TTL, cancellation을 포함한다.

### Task 5: Public Redis Facade and Examples

**complexity: medium**

**Files:**
- Create: `jwt/redis/doc.go`
- Create: `jwt/redis/redis.go`
- Create: `jwt/redis/example_test.go`

- [x] package `jwt/redis`는 caller-facing facade로 유지한다.
- [x] Redis client lifecycle은 caller-owned임을 docs에 명시한다.
- [x] examples는 compile-checked로 유지하고 raw key material을 보여주지 않는다.

### Task 6: Benchmarks, README, Review Artifacts

**complexity: high**

**Files:**
- Create: `jwt/redis_benchmark_test.go`
- Modify: `jwt/README.md`, `jwt/README.ko.md`
- Create/modify: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-*.md`

- [x] find, rotate current-hit, expired rotate, forced rotate, compose, parse benchmark harness를 추가한다.
- [x] benchmark 결과가 문서나 PR evidence에 들어가면 raw output path와 chart asset을 함께 기록한다.
- [x] README runbook에는 Redis trust boundary, key format, inspection/recovery caveats, MongoDB #198 deferral을 포함한다.
- [x] Step 6-R code review와 Step 7-R PR review는 `P0=0 P1=0`까지 닫는다.

## 최종 검증 게이트

완료 전 다음을 실행하거나, infrastructure failure는 명시적으로 기록한다.

- `go test -count=1 ./jwt/...`
- `go test -race -count=1 ./jwt/...`
- `go test -run '^$' -bench . ./jwt/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `make fmt-check`
- `make tidy-check`
- `git diff --check`
- Step 6-R / Step 7-R review artifacts with `P0=0 P1=0`

## 중단 조건

- Redis repository가 key material을 public raw-key API로 노출해야만 구현 가능한 경우 중단한다.
- cancellation/deadline이 repository IO에서 `errors.Is`로 보존되지 않으면 수정 전에는 PR을 열지 않는다.
- key values가 error strings, logs, README examples, PR body에 나타나면 release blocker로 처리한다.
