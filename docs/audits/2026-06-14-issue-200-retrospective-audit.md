# Issue #200 Retrospective Audit

Issue: #200
Parent epic: #199
Milestone: `0.6.2`
Audit date: 2026-06-14
Branch: `issue-200-retrospective-audit`

## Scope And Baseline

This audit re-verifies completed and still-open implementation evidence from
milestones `0.1.0` through `0.6.1`. The branch is audit-only: no implementation
fixes are included here.

Raw evidence is stored under:

```text
docs/audits/outputs/issue-200/
```

Baseline inventory:

| Evidence | Result |
|---|---|
| `docs/audits/outputs/issue-200/milestones.json` | Captured all GitHub milestones with `state=all`; includes closed `0.1.0` through open `0.6.1`. |
| `docs/audits/outputs/issue-200/issues-0.1.0-0.6.1.json` | Captured all listed GitHub issues; 78 issues belong to milestones `0.1.0` through `0.6.1`. |
| `docs/audits/outputs/issue-200/named-issues.jsonl` | Captured 84 resolvable named issues from #200's detailed task list; #91 did not resolve as an issue or PR. |
| `docs/audits/outputs/issue-200/package-list.txt` | Captured 34 Go packages from `go list ./...`. |

Repository shape:

| Item | Count |
|---|---:|
| Go packages | 34 |
| Test files | 132 |
| README files | 64 |
| Benchmark test files | 11 |

## Audit Flow

![Issue #200 retrospective audit flow](../images/readme-diagrams/issue-200-retrospective-audit-flow.png)

The audit follows the approved flow: inventory, evidence slice, six independent
review lenses, severity ledger, P0/P1 follow-up gate, and closure gate.

## Milestone And Issue Inventory

Milestones relevant to the audit:

| Milestone | State | Open | Closed |
|---|---|---:|---:|
| `0.1.0` | closed | 0 | 31 |
| `0.1.1` | closed | 0 | 8 |
| `0.2.0` | closed | 0 | 12 |
| `0.3.0` | closed | 0 | 28 |
| `0.4.0` | closed | 0 | 11 |
| `0.5.0` | closed | 0 | 9 |
| `0.6.0` | closed | 0 | 18 |
| `0.6.1` | open | 0 | 22 |
| `0.6.2` | open | 7 | 0 |

Named issue evidence from #200:

| Issue set | Package area |
|---|---|
| #1, #8, #9, #10, #11, #12, #13, #14, #15, #16, #17, #69, #76, #89, #90, #92, #93, #94, #98 | Foundation packages, testing helpers, codecs, serializers, compression, Testcontainers, and leader basics. |
| #2, #18, #19, #20, #21, #85, #96, #97 | Resilience, HTTP retry, timeout, circuit breaker, bulkhead, observability, and leader group election. |
| #3, #22, #23, #24, #25, #86, #107, #110, #113, #114, #115, #116, #117, #123, #125 | Cache, Redis coordination, Redis near cache, Redis locks, rate limiting, package READMEs, coverage, and hard audit follow-ups. |
| #4, #26, #27, #28, #132, #133, #134, #135, #136, #137 | State, workflow, workreport, diagrams, READMEs, stress, cancellation, and examples. |
| #5, #29, #30, #31, #158 | Batch reader/processor/writer contracts, checkpoints, skip/retry safety, and migration workload examples. |
| #6, #32, #33, #34, #35, #36, #164, #165, #166, #167, #168, #169, #170, #171, #172, #173, #174, #175, #178, #179, #180, #181, #182, #187, #192, #195 | ID, JWT, measure, money, probabilistic filters, Redis JWT keychains, cache adapters, exchange rates, locale currency, codec compatibility, and benchmark hardening. |

Unresolvable named issue:

| Number | Result |
|---:|---|
| #91 | `gh issue view 91` returned `Could not resolve to an issue or pull request`. No package mapping was possible. |

## Issue To Package Map

| Packages | Issues and PR-style entries |
|---|---|
| `core` | #1, #8, #89, #90, #93, #95 |
| `collections` | #1, #9, #89, #90, #93, #95 |
| `concurrency`, `testing`, `testing/concurrency` | #10, #69, #89, #90, #93, #95, #114 |
| `codec` | #11, #187 |
| `serialization` | #12, #94 |
| `compression` | #13, #76, #195 |
| `leader`, `leader/redis` | #14, #15, #17, #85, #86, #134 |
| `testcontainers/kafka`, `testcontainers/mysql`, `testcontainers/nats`, `testcontainers/postgres`, `testcontainers/redis` | #16, #98 |
| `resilience` | #2, #18, #19, #20, #21, #96, #97 |
| `cache`, `cache/rediscoord`, `cache/redisnear` | #3, #22, #23, #107, #110, #116, #117 |
| `lock/redis` | #24 |
| `ratelimit`, `ratelimit/redis` | #25 |
| `state` | #4, #26, #132, #133, #135, #136, #137 |
| `workflow`, `workreport` | #4, #27, #28, #132, #133, #135, #136, #137 |
| `batch` | #5, #29, #30, #31, #158 |
| `id` | #6, #32, #164, #165, #166, #167, #168, #169, #170, #171, #172, #192 |
| `jwt`, `jwt/redis` | #6, #33, #173, #174, #175 |
| `measure` | #6, #34 |
| `money` | #6, #35, #178, #179, #180, #181 |
| `probabilistic`, `probabilistic/redis` | #6, #36, #182 |

## Package Findings

| Package | Source/tests/docs evidence | Perf | Stability | Security | Ops | API | User | Finding |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `core` | `core/*.go`, `core/*_test.go`, `core/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Validation helpers have explicit error returns and examples; no finding. |
| `collections` | `collections/*.go`, `collections/*_test.go`, `collections/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Nil function inputs return errors; examples cover public helpers; no finding. |
| `serialization` | `serialization/*.go`, `serialization/*_test.go`, `serialization/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Envelope sentinels use `errors.Is`; trailing JSON payloads rejected; no finding. |
| `codec` | `codec/*.go`, `codec/*_test.go`, `codec/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Alphabet panics are package invariant construction only; compatibility tests cover codec paths; no finding. |
| `compression` | `compression/*.go`, benchmarks, examples, `compression/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Streaming close errors are wrapped; benchmark matrix exists; no finding. |
| `concurrency` | `concurrency/*.go`, tests, examples, `concurrency/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Goroutine helpers and cancellation tests present; no finding. |
| `testing` | `testing/*.go`, tests, `testing/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Eventually helper behavior is covered; no finding. |
| `testing/concurrency` | stress tester source, tests, examples, `testing/concurrency/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Bounded `GoroutineStressTester` and panic/error aggregation covered; no finding. |
| `workreport` | `workreport/*.go`, concurrency tests, examples, `workreport/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Status/failure-policy model is tested under stress; no finding. |
| `cache` | memory cache source, tests, benchmarks, `cache/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | In-process cache covers cancellation, TTL, stampede loader, and benchmarks; no finding. |
| `cache/rediscoord` | Redis coordinator source, tests, benchmark, `cache/rediscoord/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Lock wait/cancel and Testcontainers paths covered; no finding. |
| `cache/redisnear` | Redis near cache source, failure-injection tests, benchmark, `cache/redisnear/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Subscriber close/error paths and invalidation failure injection covered; no finding. |
| `leader` | strategy/elector source, tests, `leader/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Process-local election contracts covered; no finding. |
| `leader/redis` | Redis leader/group/strategic source, tests, examples, `leader/redis/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Lease renew/release, expired slots, and strategic candidates covered; no finding. |
| `lock/redis` | Redis mutex source, tests, examples, `lock/redis/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Owner-token unlock script and cancellation cases covered; no finding. |
| `ratelimit` | local limiter source, HTTP middleware, tests, benchmark, `ratelimit/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Local limiter covers idle pruning, HTTP behavior, stress, and benchmark; no finding. |
| `ratelimit/redis` | Redis limiter source, tests, `ratelimit/redis/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Redis key expiry and multi-client tests cover distributed behavior; no finding. |
| `probabilistic/redis` | Redis Bloom source, config/concurrency tests, benchmark, examples | 0 | 0 | 0 | 0 | 0 | 1 | P2: public Redis Bloom subpackage lacks `README.md`/`README.ko.md`; defer to `0.6.2` docs hardening. |
| `jwt/redis` | Redis alias package source, tests, examples | 0 | 0 | 0 | 0 | 0 | 1 | P3: subpackage relies on root `jwt/README*.md` Redis sections rather than package-local README; acceptable but note for docs parity. |
| `resilience` | retry/timeout/circuit/bulkhead/http source, tests, examples, `resilience/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Sentinels, `errors.Is`/`errors.As`, context cancellation, HTTP body replay checks, and examples covered; no finding. |
| `workflow` | runner source, tests, concurrency tests, examples, `workflow/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Sequential/parallel/conditional cancellation and failure policy covered; no finding. |
| `state` | FSM source, tests, concurrency tests, examples, `state/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Guard, invalid transition, final state, and concurrent transition cases covered; no finding. |
| `batch` | batch source, checkpoint tests, policy tests | 0 | 0 | 0 | 0 | 0 | 1 | P2: public batch package lacks package README pair while scope includes restart/checkpoint semantics; defer to `0.6.2` docs hardening. |
| `id` | generator source, tests, concurrency tests, benchmark, examples, `id/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | UUID/ULID/KSUID/Snowflake parse, clock, entropy, stress, and benchmark evidence present; no finding. |
| `jwt` | provider/cache/distributed/Redis repository source, tests, stress, benchmarks, `jwt/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Algorithm restrictions, cache profile secrecy, key rotation, distributed repository, and context APIs covered; no finding. |
| `measure` | measure/parse/registry source, tests, concurrency tests, examples, `measure/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Parse errors preserve sentinel/cause; concurrency tests cover registry/value behavior; no finding. |
| `money` | money/currency/provider source, tests, concurrency tests, benchmark, examples, `money/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Exchange-rate providers, locale currency, FastMoney benchmark decision, and concurrency coverage present; no finding. |
| `probabilistic` | local Bloom source, tests, concurrency tests, benchmark, examples, `probabilistic/README*.md` | 0 | 0 | 0 | 0 | 0 | 0 | Config validation, hasher invariants, concurrency, and benchmark coverage present; no finding. |
| `testcontainers/kafka` | helper source, tests, `testcontainers/kafka/README*.md` | 0 | 0 | 0 | 1 | 0 | 0 | P2: cleanup uses unbounded `context.Background()` for `Terminate`; defer bounded cleanup context hardening to `0.6.2`. |
| `testcontainers/mysql` | helper source, tests, `testcontainers/mysql/README*.md` | 0 | 0 | 0 | 1 | 0 | 0 | Same P2 as Testcontainers cleanup family. |
| `testcontainers/nats` | helper source, tests, `testcontainers/nats/README*.md` | 0 | 0 | 0 | 1 | 0 | 0 | Same P2 as Testcontainers cleanup family. |
| `testcontainers/postgres` | helper source, tests, `testcontainers/postgres/README*.md` | 0 | 0 | 0 | 1 | 0 | 0 | Same P2 as Testcontainers cleanup family. |
| `testcontainers/redis` | helper source, tests, `testcontainers/redis/README*.md` | 0 | 0 | 0 | 1 | 0 | 0 | Same P2 as Testcontainers cleanup family. |

Package verdict totals:

```text
P0=0 P1=0 P2=3 P3=1
```

## P0/P1 Follow-Up Issues

No P0/P1 follow-up issues required.

## Deferred Parity Gaps

| Severity | Area | Rationale | Target milestone |
|---|---|---|---|
| P2 | `probabilistic/redis` README parity | Public Redis Bloom filter package has examples and root package docs but no local README pair. This is a docs/user gap, not a runtime defect. | `0.6.2` |
| P2 | `batch` README parity | Public batch package contains checkpoint/retry/skip semantics but no package README pair. This is a docs/user gap, not a runtime defect. | `0.6.2` |
| P2 | Testcontainers cleanup timeout | Helpers terminate containers from `t.Cleanup` with `context.Background()`. Current tests pass, but bounded cleanup would reduce local/CI hang risk when Docker is unhealthy. | `0.6.2` |
| P3 | `jwt/redis` local README | The alias package is documented through root `jwt/README*.md`; a local README could improve discoverability but is not required for correctness. | `0.6.3` |

## Validation Evidence

| Command | Output file | Result |
|---|---|---|
| `go test -count=1 ./...` | `docs/audits/outputs/issue-200/go-test-all.txt` | PASS |
| `go test -race -count=1 ./...` | `docs/audits/outputs/issue-200/go-test-race-all.txt` | PASS |
| `go test -count=1 ./testing/concurrency ./concurrency` plus targeted Redis/JWT race gate | `docs/audits/outputs/issue-200/go-test-race-targeted.txt` | PASS |
| `make ci` | `docs/audits/outputs/issue-200/make-ci.txt` | PASS after clearing stale `golangci-lint` cache entries from removed `issue-180-fastmoney-evaluation` worktree. |

Operational note:

- The first `make ci` attempt failed because `golangci-lint` cache still
  referenced files under a removed worktree:
  `.worktrees/issue-180-fastmoney-evaluation`.
- `golangci-lint cache clean` cleared the stale absolute paths.
- The rerun reported `0 issues` and completed `tidy-check`, `fmt-check`, `vet`,
  `lint`, `test`, and `race`.

## 7-Tier Integration Verdict

| Lane | P0 | P1 | P2 | P3 | Verdict |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Benchmark files exist for cache, compression, ID, JWT, money, probabilistic, Redis probabilistic, and rate limit surfaces; validation gates passed. |
| Stability | 0 | 0 | 0 | 0 | Full test and race gates passed; targeted goroutine/stress and Redis/JWT race gates passed. |
| Security | 0 | 0 | 0 | 0 | JWT algorithm restrictions, cache profile secrecy, Redis namespace validation, parser sentinels, and key handling tests showed no P0/P1 defects. |
| Operator/Ops | 0 | 0 | 1 | 0 | Testcontainers cleanup lacks bounded terminate contexts; current tests pass, but bounded cleanup should be hardened. |
| Developer/API | 0 | 0 | 0 | 0 | Public APIs use Go-native errors and context-aware methods where runtime operations require them. |
| User/Caller | 0 | 0 | 2 | 1 | `batch` and `probabilistic/redis` need README parity; `jwt/redis` local README is optional discoverability work. |

Final gate:

```text
P0=0 P1=0
```

## DoD Status

| Requirement | Status | Evidence |
|---|---|---|
| Committed audit artifact records package-by-package P0/P1/P2/P3 severity | PASS | `## Package Findings` and package verdict totals above. |
| Final audit gate includes exact P0/P1 counts | PASS | `P0=0 P1=0`. |
| P0/P1 findings filed as follow-up issues before close | PASS | No P0/P1 follow-up issues required. |
| Deferred parity gaps include rationale and target milestone | PASS | `## Deferred Parity Gaps`. |
| Race/stress validation included | PASS | Full race gate and targeted goroutine/Redis/JWT gate passed. |
| CI evidence included | PASS | `make ci` passed after stale linter cache cleanup. |
