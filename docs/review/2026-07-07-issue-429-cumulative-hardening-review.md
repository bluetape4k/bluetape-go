# Issue #429 Cumulative Hardening Review

## Scope

- Issue: #429 `refactor: Apply cumulative lesson hardening to code through 0.12.0`
- Parent: #423
- Worktree: `.worktrees/issue-429-cumulative-hardening`
- Branch: `issue-429-cumulative-hardening`
- Primary lesson sources: `docs/lessons/**`, `docs/review/**`, and
  `bluetape-go-patterns` lessons promoted through 0.12.0.

## Evidence Scan

| Area | Evidence | Result |
|---|---|---|
| Retrospective P1 fixes | `docs/review/2026-07-07-0.13.0-retrospective-review.md` records the two confirmed P1s; PR #446 fixed them and added `docs/lessons/2026-07-07-retrospective-p1-fixes.md`. | feeds #425, closed |
| Testcontainers cleanup | `docs/lessons/2026-06-14-issue-201-test-gates.md` requires bounded cleanup independent from caller cancellation. `testcontainers/toxiproxy/toxiproxy_test.go` and README pair still used the start context or `testcontainers.TerminateContainer` for upstream Redis/network cleanup. | fixed |
| README errcheck shape | `docs/lessons/2026-06-04-redis-near-cache.md` records that examples must satisfy errcheck-shaped cleanup. `cache/redisnear`, `cache/rediscoord`, and `jwt` README pairs had bare `defer Close()` snippets. | fixed |
| Caller-owned Redis keys | `docs/lessons/2026-07-07-retrospective-p1-fixes.md` records the Redis key preservation rule. The confirmed `ratelimit/redis` defect was fixed by #425. Current #429 scan found no additional exact `TrimSpace` storage-key collapse in touched areas. | no-op with evidence |
| Cancellation and shared-state proof | #444 added stress coverage; #446 fixed same-key cache cancellation leakage. Current changes touch cleanup/docs only. | no-op with evidence |
| Testcontainers serial scheduling | `Makefile` still uses serial package execution for Docker-backed full-suite commands, and package README pairs continue to document `-p 1`. | no-op with evidence |

## Package Slice Checklist

| Slice | Status | Notes |
|---|---|---|
| `core`, `collections`, `codec`, `serialization`, `compression`, `id`, `money`, utility packages | no-op with evidence | No new lesson-backed P0/P1 found in the #429 scan; prior stress work covered `id` entropy and utility-shaped concurrency where applicable. |
| `concurrency`, `testing`, Testcontainers helpers | fixed | Toxiproxy integration cleanup now uses bounded `context.WithoutCancel` contexts; Testcontainers helper docs remain serial. |
| `resilience`, `cache`, `redisnear`, `rediscoord`, `lock/redis`, `ratelimit`, `leader`, JWT/key rotation | fixed | README cleanup shape fixed for cache/JWT snippets; #425 owns the prior cache/ratelimit P1 code fixes. |
| `workflow`, `batch`, SQL/database, AWS/DynamoDB, text/language/tokenizer, audit/outbox, graph/image/encryption, rules | no-op with evidence | Current review found no new P0/P1 in these slices beyond prior closed reviews and #444/#446 hardening. |
| Package README/README.ko, examples, diagrams, benchmarks, release docs | fixed | README and README.ko pairs updated together for touched packages. No diagram or benchmark changes in this PR. |

## 7-Tier Review

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | Cleanup context allocation is test-only and not production hot path. P0=0 P1=0. |
| Stability | PASS | Docker network and upstream Redis cleanup no longer borrow a possibly canceled start context. P0=0 P1=0. |
| Security | PASS | No credentials, token, auth, or persistence behavior changed. P0=0 P1=0. |
| Operator/Ops | PASS | README examples now match bounded Testcontainers cleanup guidance and serial package guidance remains intact. P0=0 P1=0. |
| Developer/API | PASS | Public APIs are unchanged; snippets now teach errcheck-shaped cleanup. P0=0 P1=0. |
| User/Caller | PASS | README and README.ko pairs stay synchronized for changed package guidance. P0=0 P1=0. |
| Integration | PASS | Targeted package tests and race tests passed before full CI. P0=0 P1=0. |

## Validation

- `rg -n "TerminateContainer\\(|nw\\.Remove\\(ctx\\)|defer (near|client)\\.Close\\(\\)|container\\.Terminate\\(context\\.Background\\(\\)" --glob '*.go' --glob 'README*.md'`: no matches.
- `go test -p 1 -count=1 ./testcontainers/toxiproxy ./cache/redisnear ./cache/rediscoord ./jwt`: PASS.
- `go test -race -p 1 -count=1 ./testcontainers/toxiproxy ./cache/redisnear ./cache/rediscoord ./jwt`: PASS.
- `git diff --check`: PASS.
- `golangci-lint cache clean && make ci`: PASS. The cache clean was required
  because the first `make ci` lint pass reported stale diagnostics from the
  removed `issue-425-retrospective-p1-fixes` worktree.

P0=0 P1=0.
