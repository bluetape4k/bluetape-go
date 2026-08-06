# Issue #173 Pre-Implementation Risk Note

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #173
Milestone: 0.6.1
날짜: 2026-06-12

Plan: `docs/superpowers/plans/2026-06-12-issue-173-distributed-jwt-keychain-repositories-plan.md`

## Branch and Retrieval Evidence

| Check | Evidence | Status |
| --- | --- | --- |
| Worktree branch | `issue-173-distributed-jwt-keychain-repositories...origin/develop [ahead 2]` | PASS |
| Base ancestry | `git merge-base --is-ancestor origin/develop HEAD` exited `0` | PASS |
| GNO issue evidence | `gno_search` found `bluetape-go/issues/000173.md` for issue #173 | PASS |
| code-review-graph | `get_minimal_context` returned an empty graph for this worktree, so direct source search is the source of truth | PASS |

## Current Source Evidence

- `jwt/provider.go`: `Provider` owns option normalization, key creation, signing, parsing, and local rotation.
- `jwt/repository.go`: in-memory repository is private and context-free.
- `jwt/keychain.go`: key material stays private and package-local helpers are the only safe reconstruction boundary.
- `ratelimit/redis`: Redis code uses caller-owned `redis.Cmdable`, Lua `Eval`, namespace keys, and `%w` wrapping.
- `testing/concurrency`: `GoroutineStressTester` covers shared-state stress; `AsyncJobTester` covers cancellation/deadline paths.

## Evidence Commands

```bash
pwd
git status --short --branch
git merge-base --is-ancestor origin/develop HEAD
rg -n "type Provider|func \(p \*Provider\) Compose|func \(p \*Provider\) Parse|func \(p \*Provider\) createKeyChain" jwt/provider.go
rg -n "type keyChainRepository|func \(r \*keyChainRepository\) rotate|func \(r \*keyChainRepository\) find" jwt/repository.go
rg -n "type Limiter|redis\.Cmdable|Eval\(" ratelimit/redis
rg -n "GoroutineStressTester|AsyncJobTester|type Task" testing/concurrency
```

All commands returned expected evidence.

## Locked Decisions

- `DistributedProvider` uses `provider *Provider`, not anonymous embedding.
- Redis core lives in package `jwt`, while package `jwt/redis` is a facade; this avoids public raw-key reconstruction helpers.
- Constructors require non-nil `context.Context`.
- Constructors require non-nil `DistributedKeyChainRepository` before bootstrap.
- Repository IO preserves caller cancellation and deadlines for `errors.Is`.
- Redis is signing authority; key values never appear in error strings, logs, README examples, or PR body.
- Redis `KeyTTL` default is `0`; a configured positive TTL must be greater than or equal to retained key validity plus repository-level `RetentionLeeway`.
- Benchmark results require a real chart asset if they are shown outside raw test output.

## Step 2-R P2 Carry Forward

- Pin Redis command-count expectations for hot paths before implementation.
- Pin benchmark budget expectations before implementation.
- Add README runbook tasks with safe Redis inspection and recovery checks.

## Risk Controls Before Code

| Risk | Control |
| --- | --- |
| Public raw-key API leaks signing material | Keep Redis DTO encode/decode in package `jwt`; expose `jwt/redis` facade aliases only. |
| Context cancellation is lost after expensive key generation | Add tests for pre-create and post-create cancellation before Redis store. |
| Redis Testcontainers flake under parallel package execution | Use serial `go test -p 1` for Redis-backed tests and benchmarks. |
| Parse hot path regresses to scan/list behavior | Add command-capture tests that reject scan/list/all-key commands. |
| Benchmark evidence becomes table-only | Generate chart assets with `$bluetape4k-diagram` and `$vega` when publishing benchmark results. |
