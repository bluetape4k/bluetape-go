# Issue #579 Redis Lock Substrate Migration Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 맥락

Issue #569 introduced `redis` as the shared, Go-native Redis safety substrate.
`lock/redis` still owns a parallel random-token generator, Lua unlock script,
TTL validation, and provider-error formatting. Issue #579 is the first narrow
adoption slice under #570. It must remove only duplicated internals while
preserving the existing single-instance owner-token lock contract.

## Current Contract Evidence

- `lock/redis/mutex.go` acquires with `SetNX(ctx, key, token, ttl)` and releases
  only with a compare-and-delete Lua script.
- `lock/redis/options.go` preserves `Options.Key` verbatim and accepts any
  non-blank caller-supplied `Options.Token`.
- `lock/redis/mutex_test.go` proves contention, owner drift after expiry,
  cancellation, and concurrent same-key exclusion.
- `redis/script.go`, `redis/token.go`, `redis/lease.go`, and `redis/errors.go`
  provide the same owner-safe script boundary with redacted diagnostics.

## Decision

Migrate only `lock/redis`. Keep `Mutex`, `Lease`, `Options`, `ErrNotAcquired`,
and their public method signatures unchanged. Internally, use the shared
substrate for generated tokens, script dispatch, lease ownership, and
sanitized operational errors. It retains local TTL validation because the
shared millisecond minimum would change the existing option contract.

`Options.Token` remains a caller-visible `string` and may remain any non-blank
value. Its existing `strings.TrimSpace` normalization is retained: leading and
trailing whitespace are removed, while a whitespace-only token is rejected.
The shared `redis.OwnerToken` requires canonical 64-character lowercase hex,
so it cannot directly replace caller-supplied legacy tokens without a breaking
change. The lock package will use shared generated owner tokens only when no
token is supplied, and will retain its local compatibility path for a provided
token. The shared compare-and-delete helper is used only after a compatible
shared lease can be created; otherwise the package executes an equivalent
private compatibility script with the same error-redaction rule.

## Alternatives Considered

| Approach | Decision | Reason |
|---|---|---|
| Migrate all Redis-backed packages in one PR | Reject | Key layouts, owner tokens, and public error contracts differ; the review and regression surface would be too broad. |
| Require every `Options.Token` to be canonical `redis.OwnerToken` | Reject | This silently breaks existing callers that provide arbitrary non-blank owner tokens. |
| Migrate `lock/redis` only and preserve compatibility at the option boundary | Accept | Removes duplicated generation and standardizes safe-script behavior while preserving public API and stored values. |

## Invariants And Acceptance Criteria

1. `Options.Key` is stored and sent to Redis byte-for-byte, including leading
   or trailing spaces.
2. A non-blank caller-supplied `Options.Token` retains the existing
   `strings.TrimSpace` normalization; no new re-encoding or canonicalization
   is introduced.
3. When `Options.Token` is empty, generated tokens are canonical shared
   `redis.OwnerToken` values and do not appear in formatted errors or logs.
4. `TryLock` and `Unlock` preserve a canceled or expired context and make no
   Redis dispatch after the context is already done. A nil context continues to
   normalize to `context.Background()` as the existing public behavior does.
5. A stale lease may not delete a key owned by a later owner; owner mismatch
   remains `(false, nil)`.
6. Redis/provider failures remain discoverable through `errors.Is`/`errors.As`
   but error strings omit raw Redis keys and owner-token values.
7. The package remains a single-instance `SET NX` lock, not Redlock, fencing,
   semaphore, renewal, or retry-loop functionality.

## 테스트 계획

- Add package coverage for generated-token canonicality and redacted operation
  errors using the existing Redis Testcontainers fixture where lock acquisition
  is required.
- Keep Testcontainers coverage serial for acquire, unlock, contention, expiry,
  owner drift, canceled context, and caller-owned key/token compatibility.
- Run `go test -p 1 -count=1 ./lock/redis` and
  `go test -p 1 -race -count=1 ./lock/redis`.
- Run `go test -p 1 -count=1 ./redis` to protect the dependency contract, then
  `make ci` before PR publication.

## 위험 And Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| Canonical shared tokens reject legacy custom lock tokens. | P1 | Keep the `string` option contract and test custom-token byte preservation. |
| Sanitized errors hide typed provider causes. | P1 | Wrap the original cause through `redis.OpError` and assert `errors.Is`. |
| Unlock behavior changes after TTL expiry. | P1 | Preserve the owner-mismatch `(false, nil)` test with two Redis owners. |
| A canceled context dispatches a script late. | P1 | Check `ctx.Err()` before every acquire/unlock dispatch and assert no key leak. |

## Non-Goals

- No public API redesign or key layout migration.
- No migration of leader, cache, rate limiter, probabilistic, or JWT packages.
- No benchmark run: this is a behavior-preserving internal migration. The
  provider benchmark matrix remains issue #560; therefore no result chart is
  applicable to this issue.
