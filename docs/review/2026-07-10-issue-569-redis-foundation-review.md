# Issue 569 Redis Foundation Step 6-R Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-10
브랜치: `feat/issue-569-redis-foundation`
Baseline: `7c8af9a Plan Redis foundation implementation`
범위: new public `redis` package, package README pair, root README indexes, and
`CHANGELOG.md`.

## 검토한 증거

- Implementation:
  - `redis/token.go`: 256-bit lowercase-hex owner token, redacted formatting,
    explicit sensitive `RedisValue`.
  - `redis/key.go`: structural/logical key separation, hash tag preservation,
    deterministic redacted key IDs.
  - `redis/errors.go`: sentinel errors and sanitized `OpError`.
  - `redis/lease.go`, `redis/script.go`, `redis/ttl.go`: immutable redacted
    lease, compare-delete/extend Lua scripts, TTL guard.
- Tests:
  - Unit tests cover token canonicality/redaction, key validation and byte
    preservation, TTL errors, OpError wrapping/sanitization, lease validation,
    fake `redis.Scripter` no-dispatch validation, and script result parsing.
  - Integration tests use `testcontainers/redis` for compare-delete,
    compare-extend, stale-owner drift, pre-canceled contexts, script fallback,
    and interleaved-owner stress cases.
- Docs:
  - `redis/README.md` and `redis/README.ko.md` document scope, non-goals,
    key/token boundaries, cancellation runbook, rollback, and verification.
  - Root `README.md` and `README.ko.md` include the package index entry.

## 검증

| 명령 | 결과 |
|---|---|
| `go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'` | PASS after TDD green |
| `go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'` | PASS after TDD green |
| `go test -count=1 ./redis -run 'Lease|CompareAnd'` | PASS after TDD green |
| `go test -p 1 -count=1 ./redis -run 'CompareAnd(Delete|Extend)'` | PASS |
| `go test -p 1 -count=1 ./redis` | PASS |
| `go test -p 1 -race -count=1 ./redis` | PASS |
| `go test -count=1 ./redis -run Example` | PASS |
| `git diff --check` | PASS |
| `rg -n 'github.com/bluetape4k/bluetape-go/redis' lock/redis leader/redis ratelimit/redis probabilistic/redis cache/rediscoord cache/redisnear jwt \|\| true` | PASS, no migrated imports |
| `gh issue view 569 --json assignees,milestone,labels,state,title,url` | PASS, assignee `debop`, milestone `0.19.0`, state `OPEN` |
| `make ci` | PASS |

## 7-Tier 발견 사항

| Tier | Perspective | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---:|---|
| 1 | Performance/runtime | 0 | 0 | 0 | 0 | Package-level Lua scripts avoid per-call script construction; no benchmark was run because #569 is foundation-only. |
| 2 | Stability/reliability | 0 | 0 | 0 | 0 | Nil/pre-canceled context and nil/typed-nil client checks happen before dispatch; Testcontainers tests cover real Redis behavior. |
| 3 | Security/secrecy | 0 | 0 | 0 | 0 | Owner tokens, leases, and key diagnostics are redacted by default; `OpError.Error()` omits raw keys, tokens, and provider text. |
| 4 | Operator/Ops | 0 | 0 | 0 | 0 | README pair documents ownership drift, post-dispatch indeterminate state, redacted-id correlation limits, bounded SCAN cleanup, rollback, and no-migration boundary. |
| 5 | Developer/API | 0 | 0 | 0 | 0 | Public surface stays narrow: keys, tokens, leases, TTLs, scripts, and typed/redacted errors only. |
| 6 | User/Caller | 0 | 0 | 0 | 0 | README pair and examples show import/package name, timeout ownership, `(false, nil)` drift, and diagnostics. |
| 7 | Integration/evidence | 0 | 0 | 0 | 0 | Targeted tests, race, serial Testcontainers, no-migration scan, issue metadata, and full `make ci` passed. |

P0=0 P1=0

## 남은 위험 및 후속 조치

- #570 must own migration from package-local Redis helpers to this foundation
  package, with old/new key parity tests before any package behavior changes.
- Provider benchmark and comparison work remains outside #569. If #570 or #560
  runs benchmarks, the result artifact must include command, conditions, raw
  output, table, chart, and analysis.
- Post-dispatch Redis cancellation cannot prove commit state from the client
  return value alone; callers must treat it as an operator runbook condition.
